FROM golang:1.15 AS builder

WORKDIR /go/src/github.com/whywaita/myshoes

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go get -u google.golang.org/grpc \
    && go get -u github.com/rakyll/statik \
    && go get -u github.com/golang/protobuf/protoc-gen-go
RUN apt-get update -y \
    && apt-get install -y protobuf-compiler

COPY . .
RUN make build-linux

FROM alpine

RUN apk update \
  && apk update
RUN apk add --no-cache ca-certificates \
  && update-ca-certificates 2>/dev/null || true

COPY --from=builder /go/src/github.com/whywaita/myshoes/myshoes-linux-amd64 /app

CMD ["/app"]