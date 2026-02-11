# Scale Set Mode

## 概要

Scale setモードは、GitHub Actions Runner Scale Set APIを使用したlong-polling駆動の自動スケーリング機能です。従来のWebhookモード（GitHub → myshoes（push））から、myshoes → GitHub（pull）への切り替えが可能になります。

### Webhookモードとの違い

| 項目 | Webhookモード（従来） | Scale setモード（新規） |
|------|----------------------|------------------------|
| **通信方向** | GitHub → myshoes (push) | myshoes → GitHub (pull) |
| **トリガー** | GitHub webhook | Long-polling API |
| **ランナー登録** | Registration token + config.sh | JIT (Just-In-Time) config |
| **起動速度** | 通常 | 高速（JIT configにより） |
| **Job queue** | 使用 | 不使用 |
| **スケーリング** | starter/runner loopで管理 | Scale set managerで管理 |

## GitHub App 権限

Scale setモードで必要なGitHub App権限:

| 権限 | Repository scope | Organization scope | 理由 |
|------|-----------------|-------------------|------|
| `actions` | Read & Write | Read & Write | Runnerの登録・削除（既存と同じ） |
| `administration` | Read | - | Repository設定の読み取り（既存と同じ） |
| `organization_self_hosted_runners` | - | Read & Write | **新規追加**: Organization-level scale set管理用 |

**重要な変更点**:
- Repository-level targetの場合: 既存の権限で動作（変更不要）
- **Organization-level targetの場合**: `organization_self_hosted_runners` 権限が**新たに必要**になる

既にWebhookモードでOrganization-level targetを使用している場合、この権限を追加する必要があります。

参考: [Authenticating ARC to the GitHub API - GitHub Docs](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/authenticate-to-the-api)

## 設定方法

### 環境変数

```bash
# Scale setモードの有効化
SCALESET_ENABLED=true              # Scale setモードを有効化（default: false）
SCALESET_RUNNER_GROUP=default      # Runner group名（default: "default"）
SCALESET_MAX_RUNNERS=10            # Scale set あたりの最大ランナー数（default: 10）
SCALESET_NAME_PREFIX=myshoes       # Scale set名のプレフィックス（default: "myshoes"）

# 既存の環境変数（引き続き必要）
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY_BASE64=...
GITHUB_URL=https://github.com  # GHESの場合は変更
RUNNER_VERSION=v2.311.0
RUNNER_USER=runner
RUNNER_BASE_DIRECTORY=/tmp
PLUGIN=./shoes-lxd
# ... その他の既存設定
```

### Scale setモード有効時の動作

1. **Web server**: 起動する（REST API + メトリクス提供）
2. **starter.Loop**: 起動しない（scale set scalerが代替）
3. **runner.Loop**: 起動しない（HandleJobCompletedが代替）
4. **scaleset.Manager.Loop**: 起動する（新規）

## Webエンドポイントの変更

| エンドポイント | Webhookモード | Scale setモード | 理由 |
|---------------|--------------|----------------|------|
| `/github/events` (POST) | 必須 | **不要** | GitHubからのwebhookを受信しない。GitHub App設定でWebhook URLも不要 |
| `/target` (CRUD) | 必須 | **必須** | Scale set managerがdatastoreからtargetを読み取る |
| `/healthz` | 必須 | **必須** | ヘルスチェック |
| `/metrics` | 必須 | **必須** | Prometheusメトリクス（scale set固有メトリクスも追加） |
| `/config/*` | 任意 | **任意** | ランタイム設定変更 |

**重要**: Scale setモード有効時、`/github/events` エンドポイントは存在するが使用されません。GitHub App設定でWebhook URLを設定する必要はありません。

## 動作フロー比較

### Webhookモード（既存）

```
GitHub Actions → webhook → myshoes → job queue
                                        ↓
                                    starter loop
                                        ↓
                                  shoes plugin → runner
                                        ↓
                                  runner manager (定期削除)
```

### Scale setモード（新規）

```
myshoes scale set manager → long-poll GitHub Scale Set API
                                ↓ (JobAssigned event)
                          generate JIT config
                                ↓
                          shoes plugin → runner
                                ↓ (JobCompleted event)
                          HandleJobCompleted → 即座に削除
```

## JIT Runnerの特徴

- **Registration token不要**: JIT configに認証情報が含まれる
- **config.sh不要**: `./run.sh --jitconfig` で直接起動
- **RunnerService.jsパッチ不要**: JITランナーは本質的にephemeral
- **高速起動**: トークン生成とconfig.shステップがスキップされる

### 従来のsetup scriptとの比較

| 項目 | Webhookモード | Scale setモード |
|------|--------------|----------------|
| Registration token取得 | 必要 | 不要（JIT configに含まれる） |
| `config.sh --unattended` | 実行 | 不要 |
| `RunnerService.js` パッチ | 適用 | 不要 |
| `--ephemeral`/`--once` フラグ | 必要 | 不要（JITは本質的にephemeral） |
| 起動コマンド | `./run.sh --once` | `./run.sh --jitconfig <encoded>` |

## 既存Shoes Providerとの互換性

- **Proto変更なし**: `AddInstance` の `setupScript` 引数にJIT config含むスクリプトを渡します
- **透過的対応**: Providerは通常のsetup scriptとして扱うだけで動作します
- **移行不要**: 既存のshoes-lxd, shoes-aws, shoes-openstackがそのまま動作します

## Scale Setの命名規則

- **Format**: `{SCALESET_NAME_PREFIX}-{sanitized-scope}`
- **例**:
  - org `myorg` → scale set名 `myshoes-myorg`
  - repo `myorg/myrepo` → scale set名 `myshoes-myorg-myrepo`
- **sanitization**: `/` やその他の無効文字は `-` に置換されます

## Prometheusメトリクス

Scale setモード固有のメトリクス:

| メトリクス名 | Type | Labels | 説明 |
|------------|------|--------|------|
| `myshoes_scaleset_listener_running` | gauge | target_scope | 実行中のscale set listener数 |
| `myshoes_scaleset_desired_runners` | gauge | target_scope | 要求されたランナー数 |
| `myshoes_scaleset_active_runners` | gauge | target_scope | アクティブなランナー数 |
| `myshoes_scaleset_jobs_completed_total` | counter | target_scope | 完了したジョブの総数 |
| `myshoes_scaleset_provision_errors_total` | counter | target_scope | プロビジョニングエラーの総数 |

## 制限事項・注意点

1. **モード排他性**: Scale setモードとWebhookモードは排他的です（グローバルスイッチ）
2. **Installation必須**: Scale set作成にはGitHub App installationが必要です
3. **1 target = 1 scale set**: 各targetに対して1つのscale setが作成されます
4. **Runner group**: Scale set作成時に指定（デフォルト: "default"）
5. **GHES対応**: `GITHUB_URL` 環境変数でGHES URLを設定することで対応可能
6. **Permission**: Organization-level targetでは追加の権限（`organization_self_hosted_runners`）が必要

## 移行ガイド

### WebhookモードからScale setモードへの移行

1. **GitHub App権限の追加**（Organization-level targetを使用している場合）
   - GitHub App設定で `organization_self_hosted_runners: Read & Write` を追加

2. **環境変数の設定**
   ```bash
   SCALESET_ENABLED=true
   ```

3. **Webhook URLの削除**（任意）
   - GitHub App設定からWebhook URLを削除しても問題ありません（Scale setモードでは使用されません）

4. **myshoes再起動**
   - 環境変数を反映してmyshoesを再起動

5. **動作確認**
   - `/metrics` エンドポイントで `myshoes_scaleset_*` メトリクスが出力されることを確認
   - ログで `Starting in scale set mode` が出力されることを確認
   - Workflow jobをトリガーしてランナーがプロビジョニングされることを確認

### トラブルシューティング

**Scale setが作成されない**
- Runner group名が正しいか確認（デフォルト: "default"）
- GitHub App installationが正しく行われているか確認
- ログで `failed to get runner group` エラーがないか確認

**Runnerがプロビジョニングされない**
- `myshoes_scaleset_provision_errors_total` メトリクスを確認
- ログで `failed to provision runner` エラーを確認
- Shoes pluginが正しく動作しているか確認

**Organization-level targetでエラーが出る**
- GitHub App権限に `organization_self_hosted_runners` が追加されているか確認
- Installation IDが正しく取得できているか確認

## 参考リンク

- [GitHub Actions Runner Scale Set (scaleset) - GitHub](https://github.com/actions/scaleset)
- [Authenticating ARC to the GitHub API - GitHub Docs](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/authenticate-to-the-api)
- [Actions Runner Controller (ARC) - GitHub](https://github.com/actions/actions-runner-controller)
