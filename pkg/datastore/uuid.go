package datastore

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"uuid"
)

// UUID wraps the standard library uuid.UUID and adds database/sql support
// (driver.Valuer and sql.Scanner). UUID columns are stored as VARCHAR(36), so
// Valuc returns the string form and Scan parses the string back.
type UUID struct {
	uuid.UUID
}

var (
	_ driver.Valuer = UUID{}
	_ sql.Scanner   = (*UUID)(nil)
)

// Value implements driver.Valuer.
func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
}

// Scan implements sql.Scanner.
func (u *UUID) Scan(src interface{}) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("cannot scan %T into datastore.UUID", src)
	}

	parsed, err := uuid.Parse(s)
	if err != nil {
		return fmt.Errorf("failed to parse uuid %q: %w", s, err)
	}
	u.UUID = parsed
	return nil
}
