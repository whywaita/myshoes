package main

import (
	"fmt"
	"log"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/datastore/mysql"
	"github.com/whywaita/myshoes/pkg/shoes"
	"github.com/whywaita/myshoes/pkg/web"
)

func main() {
	myshoes, err := New()
	if err != nil {
		log.Fatal(err)
	}

	if err := myshoes.Run(); err != nil {
		log.Fatalln(err)
	}
}

type myShoes struct {
	ds    datastore.Datastore
	shoes shoes.ShoesClient
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
	if err := web.Serve(m.ds); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
