// +heroku goVersion go1.15
// +heroku install ./cmd/server/...

module github.com/whywaita/myshoes

go 1.15

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/go-sql-driver/mysql v1.4.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-github/v32 v32.1.0
	github.com/hashicorp/go-plugin v1.4.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/satori/go.uuid v1.2.0
	goji.io v2.0.2+incompatible
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
)
