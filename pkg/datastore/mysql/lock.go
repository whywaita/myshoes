package mysql

import (
	"context"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
)

// GetLock get lock
func (m *MySQL) GetLock(ctx context.Context) error {
	var res int

	cfg, err := mysql.ParseDSN(config.Config.MySQLDSN)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", err)
	}
	lockKey := cfg.DBName

	query := fmt.Sprintf(`SELECT GET_LOCK('%s', 10)`, lockKey)
	if err := m.Conn.GetContext(ctx, &res, query); err != nil {
		return fmt.Errorf("failed to GET_LOCK: %w", err)
	}

	return nil
}

// IsLocked return status of lock
func (m *MySQL) IsLocked(ctx context.Context) (string, error) {
	var res int

	cfg, err := mysql.ParseDSN(config.Config.MySQLDSN)
	if err != nil {
		return "", fmt.Errorf("failed to parse DSN: %w", err)
	}
	lockKey := cfg.DBName

	query := fmt.Sprintf(`SELECT IS_FREE_LOCK('%s')`, lockKey)
	if err := m.Conn.GetContext(ctx, &res, query); err != nil {
		return "", fmt.Errorf("failed to IS_FREE_LOCK: %w", err)
	}

	switch res {
	case 1:
		return datastore.IsNotLocked, nil
	case 0:
		return datastore.IsLocked, nil
	}

	return "", fmt.Errorf("IS_FREE_LOCK return NULL")
}
