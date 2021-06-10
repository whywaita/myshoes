package mysql

import (
	"context"
	"fmt"

	"github.com/whywaita/myshoes/pkg/datastore"
)

const (
	LockKey = "myshoes"
)

// GetLock get lock
func (m *MySQL) GetLock(ctx context.Context) error {
	var res int

	query := fmt.Sprintf(`SELECT GET_LOCK('%s', 10)`, LockKey)
	if err := m.Conn.GetContext(ctx, &res, query); err != nil {
		return fmt.Errorf("failed to GET_LOCK: %w", err)
	}

	return nil
}

// IsLocked return status of lock
func (m *MySQL) IsLocked(ctx context.Context) (string, error) {
	var res int

	query := fmt.Sprintf(`SELECT IS_FREE_LOCK('%s')`, LockKey)
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
