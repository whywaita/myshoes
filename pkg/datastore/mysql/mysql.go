package mysql

import (
	"fmt"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/jmoiron/sqlx"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	Conn *sqlx.DB
}

func New(dsn string) (*MySQL, error) {
	conn, err := sqlx.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connection: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to conn.Ping: %w", err)
	}

	return &MySQL{
		Conn: conn,
	}, nil
}

func (m *MySQL) CreateTarget(target datastore.Target) error {
	query := `INSERT INTO targets(uuid, scope, github_personal_token) VALUES (?, ?, ?)`
	if _, err := m.Conn.Exec(query, target.UUID, target.Scope, target.GitHubPersonalToken); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) GetTarget(uuid string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, github_personal_token FROM targets WHERE uuid = "%s"`, uuid)
	if err := m.Conn.Get(&t, query); err != nil {
		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) GetTargetByScope(scope string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, github_personal_token FROM targets WHERE scope = "%s"`, scope)
	if err := m.Conn.Get(&t, query); err != nil {
		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) DeleteTarget(uuid string) error {
	query := fmt.Sprintf(`DELETE FROM targets WHERE uuid = "%s"`, uuid)
	if _, err := m.Conn.Exec(query, uuid); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}
