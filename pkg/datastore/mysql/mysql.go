package mysql

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
)

// MySQL is implement datastore in MySQL
type MySQL struct {
	Conn *sqlx.DB
}

// New create mysql connection
func New(dsn string) (*MySQL, error) {
	conn, err := sqlx.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connection: %w", err)
	}

	return &MySQL{
		Conn: conn,
	}, nil
}
