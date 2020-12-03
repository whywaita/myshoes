# myshoes

Auto scaling self-hosted runner :runner: for GitHub Actions

## Setup

### Required

- network connectivity
  - github.com **OR** your GitHub Enterprise Server to myshoes (`/github/events`)

### Prepare

- Virtual Machine Image on your cloud privider
  - Ubuntu
  - install a some commands.
    - curl(1)
    - jq(1)

## Configure Environment

- `GITHUB_APP_ID`
- `GITHUB_APP_SECRET`
- `GITHUB_PRIVATE_KEY_BASE64`
  - base64 encoded private key from GitHub Apps

and some values from shoes provider. 