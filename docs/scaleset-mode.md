# Scale Set Mode

## Overview

Scale set mode provides long-polling-driven auto-scaling using the GitHub Actions Runner Scale Set API. It switches communication from the traditional webhook mode (GitHub → myshoes, push) to myshoes → GitHub (pull).

### Differences from Webhook Mode

| Item | Webhook Mode (existing) | Scale Set Mode (new) |
|------|------------------------|----------------------|
| **Communication** | GitHub → myshoes (push) | myshoes → GitHub (pull) |
| **Trigger** | GitHub webhook | Long-polling API |
| **Runner registration** | Registration token + config.sh | JIT (Just-In-Time) config |
| **Startup speed** | Normal | Fast (via JIT config) |
| **Job queue** | Used | Not used |
| **Scaling** | Managed by starter/runner loop | Managed by scale set manager |

## GitHub App Permissions

Required GitHub App permissions for scale set mode:

| Permission | Repository scope | Organization scope | Reason |
|-----------|-----------------|-------------------|--------|
| `actions` | Read & Write | Read & Write | Runner registration/deletion (same as existing) |
| `administration` | Read | - | Read repository settings (same as existing) |
| `organization_self_hosted_runners` | - | Read & Write | **Newly required**: For organization-level scale set management |

**Important changes**:
- Repository-level targets: Work with existing permissions (no changes needed)
- **Organization-level targets**: The `organization_self_hosted_runners` permission is **newly required**

If you are already using organization-level targets in webhook mode, you need to add this permission.

Reference: [Authenticating ARC to the GitHub API - GitHub Docs](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/authenticate-to-the-api)

## Configuration

### Environment Variables

```bash
# Enable scale set mode
SCALESET_ENABLED=true              # Enable scale set mode (default: false)
SCALESET_RUNNER_GROUP=default      # Runner group name (default: "default")
SCALESET_MAX_RUNNERS=10            # Max runners per scale set (default: 10)
SCALESET_NAME_PREFIX=myshoes       # Scale set name prefix (default: "myshoes")

# Existing environment variables (still required)
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY_BASE64=...
GITHUB_URL=https://github.com  # Change for GHES
RUNNER_VERSION=v2.311.0
RUNNER_USER=runner
RUNNER_BASE_DIRECTORY=/tmp
PLUGIN=./shoes-lxd
# ... other existing settings
```

### Behavior When Scale Set Mode Is Enabled

1. **Web server**: Starts (serves REST API + metrics)
2. **starter.Loop**: Does not start (replaced by scale set scaler)
3. **runner.Loop**: Does not start (replaced by HandleJobCompleted)
4. **scaleset.Manager.Loop**: Starts (new)

## Web Endpoint Changes

| Endpoint | Webhook Mode | Scale Set Mode | Reason |
|----------|-------------|----------------|--------|
| `/github/events` (POST) | Required | **Not required** | Does not receive webhooks from GitHub. Webhook URL in GitHub App settings is also unnecessary |
| `/target` (CRUD) | Required | **Required** | Scale set manager reads targets from the datastore |
| `/healthz` | Required | **Required** | Health check |
| `/metrics` | Required | **Required** | Prometheus metrics (scale set specific metrics are also added) |
| `/config/*` | Optional | **Optional** | Runtime configuration changes |

**Important**: When scale set mode is enabled, the `/github/events` endpoint exists but is not used. You do not need to configure a Webhook URL in your GitHub App settings.

## Flow Comparison

### Webhook Mode (existing)

```
GitHub Actions → webhook → myshoes → job queue
                                        ↓
                                    starter loop
                                        ↓
                                  shoes plugin → runner
                                        ↓
                                  runner manager (periodic cleanup)
```

### Scale Set Mode (new)

```
myshoes scale set manager → long-poll GitHub Scale Set API
                                ↓ (JobAssigned event)
                          generate JIT config
                                ↓
                          shoes plugin → runner
                                ↓ (JobCompleted event)
                          HandleJobCompleted → immediate cleanup
```

## JIT Runner Characteristics

- **No registration token needed**: Authentication credentials are included in the JIT config
- **No config.sh needed**: Starts directly with `./run.sh --jitconfig`
- **No RunnerService.js patch needed**: JIT runners are inherently ephemeral
- **Fast startup**: Token generation and config.sh steps are skipped

### Comparison with Traditional Setup Script

| Item | Webhook Mode | Scale Set Mode |
|------|-------------|----------------|
| Registration token retrieval | Required | Not required (included in JIT config) |
| `config.sh --unattended` | Executed | Not required |
| `RunnerService.js` patch | Applied | Not required |
| `--ephemeral`/`--once` flag | Required | Not required (JIT is inherently ephemeral) |
| Startup command | `./run.sh --once` | `./run.sh --jitconfig <encoded>` |

## Compatibility with Existing Shoes Providers

- **No proto changes**: The JIT config script is passed via the `setupScript` argument to `AddInstance`
- **Transparent support**: Providers handle it as a regular setup script
- **No migration needed**: Existing shoes-lxd, shoes-aws, and shoes-openstack work as-is

## Scale Set Naming Convention

- **Format**: `{SCALESET_NAME_PREFIX}-{sanitized-scope}`
- **Examples**:
  - org `myorg` → scale set name `myshoes-myorg`
  - repo `myorg/myrepo` → scale set name `myshoes-myorg-myrepo`
- **Sanitization**: `/` and other invalid characters are replaced with `-`

## Prometheus Metrics

Scale set mode specific metrics:

| Metric Name | Type | Labels | Description |
|------------|------|--------|-------------|
| `myshoes_scaleset_listener_running` | gauge | target_scope | Number of running scale set listeners |
| `myshoes_scaleset_desired_runners` | gauge | target_scope | Number of desired runners |
| `myshoes_scaleset_active_runners` | gauge | target_scope | Number of active runners |
| `myshoes_scaleset_jobs_completed_total` | counter | target_scope | Total number of completed jobs |
| `myshoes_scaleset_provision_errors_total` | counter | target_scope | Total number of provisioning errors |

## Limitations and Notes

1. **Mode exclusivity**: Scale set mode and webhook mode are mutually exclusive (global switch)
2. **Installation required**: GitHub App installation is required to create scale sets
3. **1 target = 1 scale set**: One scale set is created per target
4. **Runner group**: Specified at scale set creation (default: "default")
5. **GHES support**: Supported by setting the GHES URL via the `GITHUB_URL` environment variable
6. **Permissions**: Organization-level targets require additional permission (`organization_self_hosted_runners`)

## Migration Guide

### Migrating from Webhook Mode to Scale Set Mode

1. **Add GitHub App permissions** (if using organization-level targets)
   - Add `organization_self_hosted_runners: Read & Write` in your GitHub App settings

2. **Set environment variables**
   ```bash
   SCALESET_ENABLED=true
   ```

3. **Remove Webhook URL** (optional)
   - You can safely remove the Webhook URL from your GitHub App settings (it is not used in scale set mode)

4. **Restart myshoes**
   - Restart myshoes to apply the environment variable changes

5. **Verify operation**
   - Confirm that `myshoes_scaleset_*` metrics are output at the `/metrics` endpoint
   - Confirm that `Starting in scale set mode` appears in the logs
   - Trigger a workflow job and confirm that a runner is provisioned

### Troubleshooting

**Scale set is not created**
- Verify the runner group name is correct (default: "default")
- Verify that the GitHub App installation is set up correctly
- Check the logs for `failed to get runner group` errors

**Runners are not provisioned**
- Check the `myshoes_scaleset_provision_errors_total` metric
- Check the logs for `failed to provision runner` errors
- Verify that the shoes plugin is working correctly

**Errors with organization-level targets**
- Verify that `organization_self_hosted_runners` has been added to the GitHub App permissions
- Verify that the installation ID is being retrieved correctly

## References

- [GitHub Actions Runner Scale Set (scaleset) - GitHub](https://github.com/actions/scaleset)
- [Authenticating ARC to the GitHub API - GitHub Docs](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/authenticate-to-the-api)
- [Actions Runner Controller (ARC) - GitHub](https://github.com/actions/actions-runner-controller)
