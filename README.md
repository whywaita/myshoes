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
- create and install GitHub Apps
  - set values
    - GitHub App Name: free
    - Homepage URL: free
    - Webhook URL: ${your_shoes_host}/github/events
    - Webhook secret: free
    - Repository permissions:
      - Actions: Read-only
      - Administration: Read & write
      - Checks: Read-only
    - Subscribe to events
      - check `Check run`
  - download or upload private key
  
## Configure Environment

- `GITHUB_APP_ID`
- `GITHUB_APP_SECRET`
- `GITHUB_PRIVATE_KEY_BASE64`
  - base64 encoded private key from GitHub Apps

and some values from shoes provider. 


## Target

you need to register a target that repository or organization.

- scope: set target scope for auto-scaling runner.
  - repository example: `octocat/hello-worlds`
  - organization example: `octocat`
- ghe_domain: set domain of your GitHub Enterprise Server.
  - example: `https://github.example.com`
- runner_user: set linux username that execute runner. you need to set exist user.
  - DO NOT set `root`. It can't run GitHub Actions runner in root permission.
  - example: `ubuntu`
- resource_type: set instance size for a runner.
  - please check a documents of shoes-provider.
- github_personal_token: set token of GitHub Personal.
  - please check a documents of [GPT](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token).

```bash
$ curl -XPOST -d '{"scope": "octocat/hello-world", "ghe_domain": "https://github.example.com", "resource_type": "micro", "runner_user": "ubuntu"}' ${your_shoes_host}/target
```

