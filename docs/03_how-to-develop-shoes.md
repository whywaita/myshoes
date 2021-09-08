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