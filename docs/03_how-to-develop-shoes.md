# How to develop shoes provider

## TL;DR

- implement to gRPC server
    - `shoes`, `health`, `stdio`
- define resource type in your shoes provider's flavor.

## gRPC server

shoes provider use [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).
you need to register three Service.

if you use a golang in development, you can use `pkg/pluginutils/setup.go`.

please check `plugins/shoes-mock`. There are mock shoes provider.

### shoes server

`shoes` is gRPC server. you need to implement two funtion.

- `AddInstance`
- `DeleteInstance`

please check `api/proto/myshoes.proto`.

### health

`health` is [grpc-ecosystem/grpc-health-probe](https://github.com/grpc-ecosystem/grpc-health-probe).

### stdio

`stdio` is standard I/O service.

this service communicate plugin binary's standard I/O. 

## Resource type

myshoes defined some machine type. you need to map machine spec for your resource type.

- nano
- micro
- small
- medium
- large
- xlarge
- 2xlarge
- 3xlarge
- 4xlarge

## Testing shoes provider

`shoes-tester` is a CLI tool for testing shoes provider without running myshoes server.

### Build

```bash
go build -o shoes-tester ./cmd/shoes-tester
```

### Usage

#### Add instance

Simple mode (with setup script):

```bash
./shoes-tester add \
  --plugin ./path/to/your-shoes-provider \
  --runner-name test-runner \
  --resource-type nano \
  --labels "label1,label2" \
  --setup-script "#!/bin/bash\necho 'setup'"
```

Script generation mode (generate setup script automatically):

```bash
./shoes-tester add \
  --plugin ./path/to/your-shoes-provider \
  --runner-name test-runner \
  --resource-type nano \
  --generate-script \
  --scope owner/repo \
  --github-app-id 123456 \
  --github-private-key-path /path/to/key.pem \
  --runner-version latest
```

#### Delete instance

```bash
./shoes-tester delete \
  --plugin ./path/to/your-shoes-provider \
  --cloud-id your-cloud-id \
  --labels "label1,label2"
```

#### Options

Add command:
- `--plugin`: Path to shoes-provider binary (required)
- `--runner-name`: Runner name (required)
- `--resource-type`: Resource type (default: nano)
- `--labels`: Comma-separated labels
- `--setup-script`: Setup script (simple mode)
- `--generate-script`: Generate setup script automatically (script generation mode)
- `--scope`: Repository (owner/repo) or Organization (script generation mode)
- `--github-app-id`: GitHub App ID (script generation mode)
- `--github-private-key-path`: GitHub App private key path (script generation mode)
- `--runner-version`: Runner version (default: latest, script generation mode)
- `--runner-user`: Runner user (default: runner, script generation mode)
- `--runner-base-directory`: Runner base directory (default: /tmp, script generation mode)
- `--github-url`: GitHub Enterprise Server URL (script generation mode)
- `--json`: Output in JSON format

Delete command:
- `--plugin`: Path to shoes-provider binary (required)
- `--cloud-id`: Cloud ID (required)
- `--labels`: Comma-separated labels
- `--json`: Output in JSON format