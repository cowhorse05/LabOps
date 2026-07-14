package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type sqliteDialect struct{}

func (d *sqliteDialect) DriverName() string { return "sqlite" }

func (d *sqliteDialect) PreConnect(dsn string) error {
	if dsn != ":memory:" {
		return os.MkdirAll(filepath.Dir(dsn), 0o755)
	}
	return nil
}

func (d *sqliteDialect) ConfigurePool(db *sql.DB, dsn string) {
	// WAL mode requires a single writer; concurrent readers are safe.
	// Reference: Gitea, Grafana, Caddy all use MaxOpenConns=1 with SQLite WAL.
	db.SetMaxOpenConns(1)
}

func (d *sqliteDialect) Validate(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return err
		}
	}
	return nil
}

func (d *sqliteDialect) TypeMap() TypeMap {
	return TypeMap{
		CT_String:      "TEXT",
		CT_String32:    "TEXT",
		CT_String64:    "TEXT",
		CT_String128:   "TEXT",
		CT_String512:   "TEXT",
		CT_String1024:  "TEXT",
		CT_Text:        "TEXT",
		CT_MediumText:  "TEXT",
		CT_Int:         "INTEGER",
		CT_TinyInt:     "INTEGER",
		CT_BigInt:      "INTEGER",
		CT_BigIntAuto:  "INTEGER PRIMARY KEY AUTOINCREMENT",
		CT_Double:      "REAL",
	}
}

func (d *sqliteDialect) TableSuffix() string { return "" }

func (d *sqliteDialect) IndexIfNotExists() bool { return true }

func (d *sqliteDialect) IsDuplicateIndexError(error) bool { return false }

func (d *sqliteDialect) InsertOrIgnorePrefix() string { return "INSERT OR IGNORE INTO" }

func (d *sqliteDialect) UpsertSuffix(conflictCol string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf("%s = excluded.%s", c, c)
	}
	return fmt.Sprintf("ON CONFLICT(%s) DO UPDATE SET %s", conflictCol, strings.Join(parts, ", "))
}

func (d *sqliteDialect) ReplaceInto(table string) string {
	return "INSERT OR REPLACE INTO " + table
}
