#!/bin/sh

protoc -I . *.proto --go_out=plugins=grpc:.