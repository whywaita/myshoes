package testutils

import (
	"fmt"
	"log"
	"net/http/httptest"
	"testing"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/datastore/mysql"
	"github.com/whywaita/myshoes/pkg/web"

	"github.com/jmoiron/sqlx"
	"github.com/ory/dockertest/v3"
)

const (
	mysqlRootPassword = "secret"
)

var (
	testDB        *sqlx.DB
	testDatastore datastore.Datastore

	testURL string
)

// IntegrationTestRunner is all integration test
func IntegrationTestRunner(m *testing.M) int {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.Run("mysql", "8.0", []string{"MYSQL_ROOT_PASSWORD=" + mysqlRootPassword})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		var err error
		dsn := fmt.Sprintf("root:%s@(localhost:%s)/mysql", mysqlRootPassword, resource.GetPort("3306/tcp"))
		testDatastore, err = mysql.New(dsn)
		if err != nil {
			log.Fatalf("failed to create datastore instance: %s", err)
		}

		testDB, err = sqlx.Open("mysql", fmt.Sprintf("root:%s@(localhost:%s)/mysql?parseTime=true&loc=Local", mysqlRootPassword, resource.GetPort("3306/tcp")))
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	createTablesIfNotExist()
	//SetupDefaultFixtures()

	mux := web.NewMux(testDatastore)
	ts := httptest.NewServer(mux)
	testURL = ts.URL

	code := m.Run()

	ts.Close()
	truncateTables()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	return code
}
