FROM golang:1.23 AS builder

WORKDIR /go/src/github.com/whywaita/myshoes

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.1
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
RUN apt-get update -y \
    && apt-get install -y protobuf-compiler

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY . .
RUN make build-linux

FROM alpine

RUN apk update \
  && apk update
RUN apk add --no-cache ca-certificates \
  && update-ca-certificates 2>/dev/null || true

COPY --from=builder /go/src/github.com/whywaita/myshoes/myshoes-linux-amd64 /app

CMD ["/app"]
