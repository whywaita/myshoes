# myshoes

Auto scaling self-hosted runner :runner: for GitHub Actions

## Setup (only once)

### Required

- network connectivity to myshoes server.
  - github.com **OR** your GitHub Enterprise Server to myshoes (`/github/events`).

### Prepare

please prepare a something in first.

- Virtual Machine Image on your cloud provider.
  - installed a some commands.
    - curl(1)
    - jq(1)
      - optional. will be to install boot runner if not installed. 
- Create GitHub Apps
  - set values
    - GitHub App Name: any text
    - Homepage URL: any text
    - Webhook URL: `${your_shoes_host}/github/events`
    - Webhook secret: any text
    - Repository permissions:
      - Actions: Read-only
      - Administration: Read & write
      - Checks: Read-only
    - Subscribe to events
      - check `Check run`
  - download from GitHub or upload private key from your machine.
- Install GitHub Apps to target repository to organization.
  
### Running

```bash
$ ./myshoes
```

- `PORT`
  - Listen port for web application.
- GitHub Apps information
  - `GITHUB_APP_ID`
  - `GITHUB_APP_SECRET` (if you set)
  - `GITHUB_PRIVATE_KEY_BASE64`
    - base64 encoded private key from GitHub Apps
- `MYSQL_URL`
  - DataSource Name, ex) `username:password@tcp(localhost:3306)/myshoes`
  - set if you use MySQL as a `datastore`.
- `PLUGIN`
  - set path of myshoes-provider.
  - example) `./shoes-mock` `https://example.com/shoes-mock`

and more a some values from [shoes provider](https://github.com/whywaita/myshoes-providers).

## Repository setup

### Register target

you need to register a target that repository or organization.

- scope: set target scope for auto-scaling runner.
  - repository example: `octocat/hello-worlds`
  - organization example: `octocat`
- ghe_domain: set domain of your GitHub Enterprise Server.
  - example: `https://github.example.com`
  - please contain schema.
- runner_user: set linux username that execute runner. you need to set exist user.
  - DO NOT set `root`. It can't run GitHub Actions runner in root permission.
  - example: `ubuntu`
- resource_type: set instance size for a runner.
  - please check a documents of shoes-providers.
- github_personal_token: set token of GitHub Personal.
  - please check a documents of [GPT](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token).

create target example:

```bash
$ curl -XPOST -d '{"scope": "octocat/hello-world", "ghe_domain": "https://github.example.com", "github_personal_token": "xxx", "resource_type": "micro", "runner_user": "ubuntu"}' ${your_shoes_host}/target
```

### Create an offline runner (only one)

GitHub Actions need offline runner if queueing job.
please create an offline runner in target repository.

https://docs.github.com/en/free-pro-team@latest/actions/hosting-your-own-runners/adding-self-hosted-runners

please delete a runner after registered.

### Let's go using your shoes!

:runner::runner::runner:

## Known issue

- sometimes occurred race condition if running multi jobs
  - GitHub Action's runner has a problem like a race condition.
  - related PullRequest: https://github.com/actions/runner/pull/660