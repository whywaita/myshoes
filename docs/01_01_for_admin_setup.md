# Setup myshoes daemon

## Goal

- Start myshoes daemon

## Prepare

- The network connectivity to myshoes server.
  - The webhook endpoint from github.com **OR** your GitHub Enterprise Server (`POST /github/events`).
  - REST API from your workspace (`GET, POST, DELETE /target`).
- You decide platform for runner and shoes-provider
  - The official shoes-provider topic is [myshoes-provider](https://github.com/search?q=topic%3Amyshoes-provider).
  - You can implement and use your private shoes-provider. Please check [how-to-develop-shoes.md](./03_how-to-develop-shoes.md).

## Word definition

- `your_shoes_host`: The endpoint of serving myshoes.
  - e.g.) `https://myshoes.example.com`

## Setup

Please prepare a few things first.

### Machine image for runner

- Virtual Machine Image on your cloud provider.
  - installed a some commands.
    - required: curl (1)
    - optional: jq (1), docker (1)
      - optional, but **STRONG RECOMMEND INSTALLING BEFORE** (please read known issue)
  - put latest runner tar.gz to `/usr/local/etc` [optional]
    - optional, but **STRONG RECOMMEND INSTALLING BEFORE** (please read known issue)

For example is [here](https://github.com/whywaita/myshoes-providers/tree/master/shoes-lxd/images). (packer file)

### Create GitHub Apps

#### Configure values

- GitHub App Name: any text
- Homepage URL: any text
  
##### Webhook
- Webhook URL: `${your_shoes_host}/github/events`
- Webhook secret: any text

##### Repository permissions

- Actions: Read-only
- Administration: Read & write
- Checks: Read-only

##### Organization permissions

- Self-hosted runners: Read & write
  
##### Subscribe to events

- Check `Workflow job`

### Download private key

- download from GitHub or upload private key from your machine.

### Running

```bash
$ make build
$ ./myshoes
```

A config variables can set from environment values.

- `PORT`
  - default: 8080
  - Listen port for myshoes.
- GitHub Apps information
  - required
  - `GITHUB_APP_ID`
  - `GITHUB_APP_SECRET` (if you set `Webhook secret` for your GitHub App)
  - `GITHUB_PRIVATE_KEY_BASE64`
    - base64 encoded private key from GitHub Apps
    - `$ cat privatekey.pem | base64 -w 0`
- `MYSQL_URL`
  - required
  - DataSource Name, ex) `username:password@tcp(localhost:3306)/myshoes`
- `PLUGIN`
  - required
  - set path of myshoes-provider binary.
  - example) `./shoes-mock` `https://example.com/shoes-mock` `https://github.com/whywaita/myshoes-providers/releases/download/v0.1.0/shoes-lxd-linux-amd64`
- `PLUGIN_OUTPUT`
  - default: `.`
  - set path of directory that contains myshoes-provider binary.
- `GITHUB_URL`
  - default: `https://github.com`
  - The URL of GitHub Enterprise Server.
  - Please contain schema.
- `RUNNER_VERSION`
  - default: `latest`
    - Use the latest version in starting job
  - The version of `actions/runner`
  - example) `v2.302.1`, `latest`
- `RUNNER_USER`
  - default: `runner`
  - set linux username that executes runner. you need to set exist user.
    - DO NOT set root. It can't run GitHub Actions runner in root permission.
    - Example: `ubuntu`

For tuning values

- `DEBUG`
  - default: false
  - show debugging log
- `STRICT`
  - default: true
  - set strict mode
- `MODE_WEBHOOK_TYPE`
  - default: `check_run` (use receive `workflow_job` event)
  - Set type of webhook from GitHub
  - option: `workflow_job`
- `MAX_CONNECTIONS_TO_BACKEND`
  - default: 50
  - The number of max connections to shoes-provider
- `MAX_CONCURRENCY_DELETING`
  - default: 1
  - The number of max concurrency of deleting

and more some env values from [shoes provider](https://github.com/search?q=topic%3Amyshoes-provider).
