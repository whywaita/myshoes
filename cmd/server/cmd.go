package main

import (
	"context"
	"fmt"
	"log"

	"github.com/whywaita/myshoes/pkg/web"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore/mysql"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/whywaita/myshoes/pkg/shoes"
)

func main() {
	shoes, err := New()
	if err != nil {
		log.Fatal(err)
	}

	if err := shoes.Run(); err != nil {
		log.Fatalln(err)
	}
}

type myShoes struct {
	ds datastore.Datastore
}

func New() (*myShoes, error) {
	m, err := mysql.New(config.Config.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to mysql.New: %w", err)
	}

	return &myShoes{
		ds: m,
	}, nil
}

func (m *myShoes) Run() error {
	pluginClient, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}

	if err := pluginClient.AddInstance(context.Background()); err != nil {
		return fmt.Errorf("failed to AddInstance: %w", err)
	}

	defer teardown()

	if err := web.Serve(); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
