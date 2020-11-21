# myshoes

Auto scaling self-hosted runner :runner: for GitHub Actions

# Setup

## Required

- network connectivity
  - github.com to myshoes OR GitHub Enterprise Server to myshoes (`/github/events`)
  - Runner machine to myshoes (`/setup`)

## Prepare

- Virtual Machine Image on your cloud privider
  - TODO: write setup path later.
  - install a some commands.
    - curl(1)
    - jq(1)

## Configure Environment

- `GITHUB_APP_ID`
- `GITHUB_APP_SECRET`
- `GITHUB_PRIVATE_KEY_BASE64`
  - base64 encoded private key from GitHub Apps