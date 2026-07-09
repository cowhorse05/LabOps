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
	maxOpen := 1
	if dsn != ":memory:" {
		maxOpen = 4
	}
	db.SetMaxOpenConns(maxOpen)
}

func (d *sqliteDialect) Validate(db *sql.DB) error {
	_, err := db.Exec("PRAGMA busy_timeout = 5000")
	return err
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
