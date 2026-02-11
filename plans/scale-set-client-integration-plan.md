# Scale Set Client Integration Plan

## Context

GitHub Actions Runner Scale Set Client (`github.com/actions/scaleset`) が2026年2月にpublic previewとして公開された。これは Actions Runner Controller (ARC) から抽出されたスタンドアロンGoモジュールで、Kubernetesなしにカスタムオートスケーリングソリューションを構築できる。

myshoesは現在webhook駆動（GitHub → webhook → job queue → shoes plugin → runner）で動作しているが、scale setモードでは**long-polling駆動**（myshoes → scale set API → job受信 → shoes plugin → runner）に切り替わる。JIT (Just-In-Time) runner configにより、registration token + config.shが不要になり、ランナー起動が高速化される。

**目的**: `SCALESET_ENABLED=true` 環境変数で全targetをscale setモードに切り替え可能にする。

## Architecture Overview

```
[現在] GitHub webhook → myshoes web → job queue → starter → shoes plugin → runner
[新規] myshoes scaleset manager → long-poll scale set API → JIT config生成 → shoes plugin → runner
```

Scale setモード有効時:
- `web.Serve` → **起動する**（REST API + メトリクス提供。ただし `/github/events` は不要）
- `starter.Loop` → **起動しない**（scale set scalerが代替。job queue不使用）
- `runner.Loop` → **起動しない**（HandleJobCompletedが代替。定期ポーリング不要）
- `scaleset.Manager.Loop` → **起動する**（新規追加）

## Implementation Steps

### Step 1: 依存追加

**File**: `go.mod`
- `go get github.com/actions/scaleset@latest`

### Step 2: Config拡張

**File**: `pkg/config/config.go`
- `Conf` structに追加:
  ```go
  ScaleSetEnabled    bool
  ScaleSetRunnerGroup string
  ScaleSetMaxRunners  int
  ScaleSetNamePrefix  string
  ```
- 環境変数定数追加:
  - `SCALESET_ENABLED` (bool, default: false)
  - `SCALESET_RUNNER_GROUP` (string, default: "default")
  - `SCALESET_MAX_RUNNERS` (int, default: 10)
  - `SCALESET_NAME_PREFIX` (string, default: "myshoes")

**File**: `pkg/config/init.go`
- `LoadWithDefault()` に新環境変数の読み込みを追加

### Step 3: scaleset パッケージ新規作成

#### 3a. Manager (`pkg/scaleset/manager.go` NEW)

Scale setライフサイクルを管理するオーケストレーター。

```go
type Manager struct {
    ds      datastore.Datastore
    cfg     ManagerConfig
    scalers map[uuid.UUID]*targetScaler // target UUID -> scaler
    mu      sync.RWMutex
}

type ManagerConfig struct {
    AppID           int64
    PrivateKeyPEM   []byte
    GitHubURL       string
    RunnerGroupName string
    MaxRunners      int
    ScaleSetPrefix  string
    RunnerVersion   string
    RunnerUser      string
    RunnerBaseDir   string
}
```

- `New(ds, cfg) *Manager`
- `Loop(ctx) error` - 30秒ごとにtargetリストを取得し、各targetに対してscale setとlistenerを起動/停止

**Loop処理フロー**:
1. `datastore.ListTargets(ctx, ds)` でアクティブなtargetを取得
2. 各targetについて:
   - installation IDを `gh.IsInstalledGitHubApp(ctx, scope)` で解決
   - scaleset clientを `scaleset.NewClientWithGitHubApp()` で作成
     - `GitHubConfigURL`: `{GitHubURL}/{scope}` (既存の `config.GitHubURL` を再利用)
     - `GitHubAppAuth`: `{ClientID: strconv.FormatInt(AppID, 10), InstallationID: installationID, PrivateKey: string(PEMByte)}`
   - `CreateRunnerScaleSet` or `GetRunnerScaleSet` でscale setを確保
   - `MessageSessionClient` でメッセージセッションを確立
   - `listener.New()` + `listener.Run(ctx, scaler)` でlistenerを起動
3. 削除されたtargetのlistenerをcontext cancelで停止

#### 3b. Scaler (`pkg/scaleset/scaler.go` NEW)

`listener.Scaler` interfaceを実装。scale set listenerとshoes pluginの橋渡し。

```go
type targetScaler struct {
    ds            datastore.Datastore
    target        datastore.Target
    client        *scaleset.Client
    scaleSetID    int
    cfg           ManagerConfig
    activeRunners sync.Map // runner name -> runnerInfo
}
```

**`HandleDesiredRunnerCount(ctx, count) (int, error)`**:
1. 現在のアクティブランナー数を取得
2. `count > current` の場合、差分のランナーをプロビジョニング:
   - `client.GenerateJitRunnerConfig(ctx, setting, scaleSetID)` でJIT config生成
   - `GetJITSetupScript(encodedJITConfig, ...)` でスクリプト生成
   - `shoes.GetClient()` → `client.AddInstance(ctx, name, script, resourceType, labels)` でインスタンス作成
   - `ds.CreateRunner(ctx, runner)` でdatastoreに記録
3. スケールダウンは不要（ephemeralランナーはジョブ完了後に自動終了）

**`HandleJobStarted(ctx, *scaleset.JobStarted) error`**:
- ログ出力 + メトリクス更新のみ

**`HandleJobCompleted(ctx, *scaleset.JobCompleted) error`**:
1. `activeRunners` からランナー情報を取得（RunnerName で検索）
2. `shoes.GetClient()` → `client.DeleteInstance(ctx, cloudID, labels)` でインスタンス削除
3. `ds.DeleteRunner(ctx, id, time.Now(), RunnerStatusCompleted)` でdatastoreを更新
4. `activeRunners` から削除

#### 3c. JIT Setup Script (`pkg/scaleset/scripts.go` NEW)

JIT config用の簡略化されたsetup script。既存の `pkg/starter/scripts.go` のパターンを再利用するが、大幅にシンプル化。

```go
func GetJITSetupScript(encodedJITConfig, runnerVersion, runnerUser, runnerBaseDir string) (string, error)
```

**JIT scriptの特徴**（既存scriptとの差分）:
- registration token不要（JIT configに含まれる）
- `config.sh --unattended` 不要（JIT configで自動設定）
- `RunnerService.js` パッチ不要
- `--ephemeral`/`--once` フラグ不要（JITランナーは本質的にephemeral）
- ランナー起動: `./run.sh --jitconfig <encoded>` のみ

スクリプトテンプレート（既存の `templateCreateLatestRunnerOnce` から圧縮テンプレートの仕組みを再利用）:
1. ランナーバイナリダウンロード（既存ロジック再利用）
2. `./run.sh --jitconfig ${JIT_CONFIG}` で起動

#### 3d. Metrics (`pkg/scaleset/metrics.go` NEW)

Prometheusメトリクス:
- `myshoes_scaleset_listener_running` (gauge, per target)
- `myshoes_scaleset_desired_runners` (gauge, per target)
- `myshoes_scaleset_active_runners` (gauge, per target)
- `myshoes_scaleset_jobs_completed_total` (counter, per target)
- `myshoes_scaleset_provision_errors_total` (counter, per target)

### Step 4: Server統合

**File**: `cmd/server/cmd.go`

- `myShoes` structに `ss *scaleset.Manager` を追加（nilの場合は無効）
- `newShoes()` で `config.Config.ScaleSetEnabled` を確認し、有効なら `scaleset.New()` を呼ぶ
- `Run()` の条件分岐:
  ```go
  // web.Serve は常に起動（REST API + metrics）
  eg.Go(func() error { return web.Serve(ctx, m.ds) })

  if m.ss != nil {
      // Scale setモード: starter/runner loopは起動しない
      eg.Go(func() error { return m.ss.Loop(ctx) })
  } else {
      // Webhookモード: 従来のstarter/runner loopを起動
      eg.Go(func() error { return m.start.Loop(ctx) })
      eg.Go(func() error { return m.run.Loop(ctx) })
  }
  ```
- `newShoes()` で scale setモード時も `notifyEnqueueCh`, `Starter`, `runner.Manager` は初期化するが `Run()` では使わない（将来の混在モード対応のため構造を維持）

### Step 5: テスト

#### 5a. `pkg/scaleset/scaler_test.go` (NEW)
- `HandleDesiredRunnerCount` のスケールアップテスト
- `HandleJobCompleted` のクリーンアップテスト
- エラーハンドリングテスト

#### 5b. `pkg/scaleset/scripts_test.go` (NEW)
- JIT setup scriptの生成テスト
- テンプレート出力の検証

#### 5c. `pkg/scaleset/manager_test.go` (NEW)
- Manager初期化テスト
- target追加/削除時のlistener起動/停止テスト

#### 5d. `pkg/config/config_test.go` (MODIFY if exists)
- 新環境変数の読み込みテスト

## Key Files to Modify/Create

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | MODIFY | `github.com/actions/scaleset` 依存追加 |
| `pkg/config/config.go` | MODIFY | ScaleSet関連フィールド + 環境変数定数追加 |
| `pkg/config/init.go` | MODIFY | 環境変数読み込み追加 |
| `cmd/server/cmd.go` | MODIFY | scaleset.Manager統合 |
| `pkg/scaleset/manager.go` | NEW | Scale set lifecycle manager |
| `pkg/scaleset/scaler.go` | NEW | listener.Scaler実装 |
| `pkg/scaleset/scripts.go` | NEW | JIT setup script生成 |
| `pkg/scaleset/metrics.go` | NEW | Prometheusメトリクス |
| `pkg/scaleset/manager_test.go` | NEW | Managerテスト |
| `pkg/scaleset/scaler_test.go` | NEW | Scalerテスト |
| `pkg/scaleset/scripts_test.go` | NEW | Scriptテスト |
| `docs/scaleset-mode.md` | NEW | Scale setモード運用ドキュメント |

## Reusable Existing Code

| 既存コード | 場所 | 再利用方法 |
|-----------|------|-----------|
| `shoes.GetClient()` / `shoes.Client` | `pkg/shoes/shoes.go` | インスタンス作成/削除にそのまま使用 |
| `gh.IsInstalledGitHubApp()` | `pkg/gh/jwt.go` | installation ID解決に使用 |
| `gh.DetectScope()` / `gh.DivideScope()` | `pkg/gh/scope.go` | scope解析に使用 |
| `runner.ToName()` | `pkg/runner/util.go` | ランナー命名規則に使用 |
| `datastore.ListTargets()` | `pkg/datastore/interface.go` | アクティブtarget取得に使用 |
| `datastore.Datastore.CreateRunner()` / `DeleteRunner()` | `pkg/datastore/interface.go` | ランナー記録に使用 |
| setup scriptのダウンロード関数群 | `pkg/starter/scripts.go` | テンプレートパターンを参考に簡略化版を作成 |
| `config.Config.GitHubURL` | `pkg/config/config.go` | GHES対応に再利用 |
| `logger.Logf()` | `pkg/logger/logger.go` | ログ出力に使用 |

## Technical Notes

- **Go version**: go.mod は `go 1.25` で scaleset client の要件（Go 1.25+）を満たしている
- **Auth**: scaleset clientの `GitHubAppAuth.ClientID` は AppID（int64→string変換）を受け付ける
- **GHES**: `GitHubConfigURL` に `config.GitHubURL + "/" + scope` を設定すればGHES対応可能
- **proto変更なし**: JIT configをsetup scriptにラップすることで、既存のshoesプラグインインターフェースを変更せずに対応
- **ラベル**: Scale set作成時に `Labels` を設定。targetのscopeベースでラベルを生成
- **並行性**: 各targetのlistenerは独立したgoroutineで動作。`context.WithCancel` で個別に停止可能

### Step 6: ドキュメント作成

**File**: `docs/scaleset-mode.md` (NEW)

Scale setモードに関する運用ドキュメントを作成。詳細な内容を含む（後述）。

## GitHub App 権限の変更

Scale setモードで必要なGitHub App権限:

| 権限 | Repository scope | Organization scope | 理由 |
|------|-----------------|-------------------|------|
| `actions` | Read & Write | Read & Write | Runnerの登録・削除 (既存と同じ) |
| `administration` | Read | - | Repository設定の読み取り (既存と同じ) |
| `organization_self_hosted_runners` | - | Read & Write | **新規追加**: Organization-level scale set管理用 |

**重要な変更点**:
- Repository-level targetの場合: 既存の権限で動作（変更不要）
- **Organization-level targetの場合**: `organization_self_hosted_runners` 権限が**新たに必要**になる

既にWebhookモードでOrganization-level targetを使用している場合、この権限を追加する必要がある。

参考: [Authenticating ARC to the GitHub API - GitHub Docs](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/authenticate-to-the-api)

## docs/scaleset-mode.md の内容

以下の詳細なドキュメントを作成:

### 概要
- Scale setモードとは: GitHub Actions Runner Scale Set APIを使用したlong-polling駆動の自動スケーリング
- Webhookモードとの違い: GitHub → myshoes (push) から myshoes → GitHub (pull) へ

### GitHub App 権限

上記の権限表を含める。特に `organization_self_hosted_runners` 権限がOrganization-level targetで必要になることを強調。

### 設定方法
```bash
SCALESET_ENABLED=true              # Scale setモードを有効化
SCALESET_RUNNER_GROUP=default      # Runner group名
SCALESET_MAX_RUNNERS=10            # Scale set あたりの最大ランナー数
SCALESET_NAME_PREFIX=myshoes       # Scale set名のプレフィックス
# 既存の環境変数（GITHUB_APP_ID, GITHUB_PRIVATE_KEY_BASE64 等）も必要
```

### Webエンドポイントの変更

| エンドポイント | Webhookモード | Scale setモード | 理由 |
|---------------|--------------|----------------|------|
| `/github/events` (POST) | 必須 | **不要** | GitHubからのwebhookを受信しない。GitHub App設定でWebhook URLも不要 |
| `/target` (CRUD) | 必須 | **必須** | Scale set managerがdatastoreからtargetを読み取る |
| `/healthz` | 必須 | **必須** | ヘルスチェック |
| `/metrics` | 必須 | **必須** | Prometheusメトリクス（scale set固有メトリクスも追加） |
| `/config/*` | 任意 | **任意** | ランタイム設定変更 |

**重要**: Scale setモード有効時、`/github/events` エンドポイントは存在するが使用されない。GitHub App設定でWebhook URLを設定する必要はない。

### 動作フロー比較

**Webhookモード** (既存):
```
GitHub Actions → webhook → myshoes → job queue
                                        ↓
                                    starter loop
                                        ↓
                                  shoes plugin → runner
                                        ↓
                                  runner manager (定期削除)
```

**Scale setモード** (新規):
```
myshoes scale set manager → long-poll GitHub Scale Set API
                                ↓ (JobAssigned event)
                          generate JIT config
                                ↓
                          shoes plugin → runner
                                ↓ (JobCompleted event)
                          HandleJobCompleted → 即座に削除
```

### JIT Runnerの特徴
- **Registration token不要**: JIT configに認証情報が含まれる
- **config.sh不要**: `./run.sh --jitconfig` で直接起動
- **RunnerService.jsパッチ不要**: JITランナーは本質的にephemeral
- **高速起動**: トークン生成とconfig.shステップがスキップされる

### 既存Shoes Providerとの互換性
- **Proto変更なし**: `AddInstance` の `setupScript` 引数にJIT config含むスクリプトを渡す
- **透過的対応**: Providerは通常のsetup scriptとして扱うだけで動作
- **移行不要**: 既存のshoes-lxd, shoes-aws, shoes-openstackがそのまま動作

### Scale Setの命名規則
- Format: `{SCALESET_NAME_PREFIX}-{sanitized-scope}`
- 例:
  - org `myorg` → scale set名 `myshoes-myorg`
  - repo `myorg/myrepo` → scale set名 `myshoes-myorg-myrepo`

### 制限事項・注意点
- Scale setモードとWebhookモードは排他（グローバルスイッチ）
- Scale set作成にはGitHub App installationが必要
- 各targetに対して1つのscale setが作成される
- Runner groupはscale set作成時に指定（デフォルト: "default"）
- GHESサポート: `GITHUB_URL` 設定により対応

## Verification

1. **ビルド確認**: `go build ./...` が成功すること
2. **ユニットテスト**: `go test ./pkg/scaleset/...` が全てパスすること
3. **既存テスト**: `go test ./...` で既存テストが壊れていないこと
4. **lint/fmt**: `go fmt ./...` でフォーマット済みであること
5. **統合テスト（手動）**:
   - `SCALESET_ENABLED=true` + 他の必要な環境変数を設定してサーバー起動
   - targetが登録済みの状態でscale setが作成されることを確認
   - workflow jobをトリガーしてランナーがプロビジョニングされることを確認
   - ジョブ完了後にインスタンスがクリーンアップされることを確認
