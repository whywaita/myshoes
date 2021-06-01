# myshoes

Auto scaling self-hosted runner :runner: (like GitHub-hosted) for GitHub Actions

# features

- Auto-scaling and runner with your cloud-provider 

## Setup (only once)

### Required

- network connectivity to myshoes server.
  - github.com **OR** your GitHub Enterprise Server to myshoes (`/github/events`).

### Prepare

please prepare a few things first.

- Virtual Machine Image on your cloud provider.
  - installed a some commands.
    - curl (1)
    - jq (1)
      - optional, but **STRONG RECOMMEND INSTALLING BEFORE** (please read known issue)
    - docker (1)
      - optional, but **STRONG RECOMMEND INSTALLING BEFORE** (please read known issue)
  - put latest runner tar.gz to `/usr/local/etc`
    - optional, but **STRONG RECOMMEND INSTALLING BEFORE** (please read known issue)
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
$ make build
$ ./myshoes
```

- `PORT`
  - default: 8080
  - Listen port for web application.
- GitHub Apps information
  - required
  - `GITHUB_APP_ID`
  - `GITHUB_APP_SECRET` (if you set)
  - `GITHUB_PRIVATE_KEY_BASE64`
    - base64 encoded private key from GitHub Apps
- `MYSQL_URL`
  - required
  - DataSource Name, ex) `username:password@tcp(localhost:3306)/myshoes`
  - set if you use MySQL as a `datastore`.
- `PLUGIN`
  - required
  - set path of myshoes-provider.
  - example) `./shoes-mock` `https://example.com/shoes-mock`
- `DEBUG`
  - default: false
  - show debugging log

and more some env values from [shoes provider](https://github.com/whywaita/myshoes-providers).

## Repository setup

### Register target

you need to register a target that repository or organization.

- scope: set target scope for an auto-scaling runner.
  - repository example: `octocat/hello-worlds`
  - organization example: `octocat`
- ghe_domain: set domain of your GitHub Enterprise Server.
  - example: `https://github.example.com`
  - please contain schema.
- runner_user: set linux username that executes runner. you need to set exist user.
  - DO NOT set `root`. It can't run GitHub Actions runner in root permission.
  - example: `ubuntu`
- resource_type: set instance size for a runner.
  - please check a document of shoes-providers.

create target example:

```bash
$ curl -XPOST -d '{"scope": "octocat/hello-world", "ghe_domain": "https://github.example.com", "resource_type": "micro", "runner_user": "ubuntu"}' ${your_shoes_host}/target
```

### Create an offline runner (only one)

GitHub Actions need offline runner if queueing job.
please create an offline runner in the target repository.

https://docs.github.com/en/free-pro-team@latest/actions/hosting-your-own-runners/adding-self-hosted-runners

please delete a runner after registered.

### Let's go using your shoes!

Let's execute your jobs! :runner::runner::runner:

## Known issue

- sometimes occurred race condition if running multi jobs
  - GitHub Action's runner has a problem like a race condition.
  - related PullRequest: https://github.com/actions/runner/pull/660