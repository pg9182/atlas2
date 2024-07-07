package sessiondb

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

func init() {
	migrate(up002, down002)
}

func up002(ctx context.Context, tx *sqlx.Tx) error {
	if _, err := tx.ExecContext(ctx, strings.ReplaceAll(`
		CREATE TABLE pdata_lock (
			player_uid INTEGER PRIMARY KEY,
			pdata_lock_token TEXT, -- opaque lock token for clients
			pdata_lock_desc TEXT, -- human-readable description of what locked the pdata
			pdata_lock_created INTEGER -- unix timestamp
		) STRICT;
	`, `
		`, "\n")); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, strings.ReplaceAll(`
		CREATE INDEX pdata_lock_token_idx ON pdata_lock (pdata_lock_token); -- used for lookup
	`, `
		`, "\n")); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

func down002(ctx context.Context, tx *sqlx.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		DROP INDEX pdata_lock_token_idx;
	`); err != nil {
		return fmt.Errorf("drop indexes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DROP TABLE pdata_lock;
	`); err != nil {
		return fmt.Errorf("drop tables: %w", err)
	}
	return nil
}
