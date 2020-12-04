package main

import (
	"fmt"
	"log"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/datastore/mysql"
	"github.com/whywaita/myshoes/pkg/starter"
	"github.com/whywaita/myshoes/pkg/starter/safety/unlimited"
	"github.com/whywaita/myshoes/pkg/web"
	"golang.org/x/sync/errgroup"
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
	start *starter.Starter
}

func New() (*myShoes, error) {
	m, err := mysql.New(config.Config.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to mysql.New: %w", err)
	}

	unlimit := unlimited.Unlimit{}

	s := starter.New(m, unlimit)

	return &myShoes{
		ds:    m,
		start: s,
	}, nil
}

func (m *myShoes) Run() error {
	eg := errgroup.Group{}

	eg.Go(func() error {
		if err := web.Serve(m.ds); err != nil {
			return fmt.Errorf("failed to serve: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		if err := m.start.Loop(); err != nil {
			return fmt.Errorf("failed to loop: %w", err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait errgroup: %w", err)
	}

	return nil
}
