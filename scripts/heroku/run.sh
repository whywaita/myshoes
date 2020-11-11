#!/bin/sh

protoc -I . *.proto --go_out=plugins=grpc:.

## setup, build binaries to app root directory
ls plugins | tr -d "/" | xargs -I%% bash -c "cd plugins/%% ; go build -o ../../%% . ; cd ../../"

go run cmd/server/...