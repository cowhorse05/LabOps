package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type mySQLDialect struct{}

func (d *mySQLDialect) DriverName() string { return "mysql" }

func (d *mySQLDialect) PreConnect(dsn string) error {
	return ensureMySQLDatabase(dsn)
}

func (d *mySQLDialect) ConfigurePool(db *sql.DB, _ string) {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
}

func (d *mySQLDialect) Validate(db *sql.DB) error {
	return db.Ping()
}

func (d *mySQLDialect) TypeMap() TypeMap {
	return TypeMap{
		CT_String:      "VARCHAR(256)",
		CT_String32:    "VARCHAR(32)",
		CT_String64:    "VARCHAR(64)",
		CT_String128:   "VARCHAR(128)",
		CT_String512:   "VARCHAR(512)",
		CT_String1024:  "VARCHAR(1024)",
		CT_Text:        "TEXT",
		CT_MediumText:  "MEDIUMTEXT",
		CT_Int:         "INTEGER",
		CT_TinyInt:     "TINYINT",
		CT_BigInt:      "BIGINT",
		CT_BigIntAuto:  "BIGINT PRIMARY KEY AUTO_INCREMENT",
		CT_Double:      "DOUBLE",
	}
}

func (d *mySQLDialect) TableSuffix() string {
	return " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
}

func (d *mySQLDialect) IndexIfNotExists() bool { return false }

func (d *mySQLDialect) IsDuplicateIndexError(err error) bool {
	if err == nil {
		return true
	}
	if me, ok := err.(*mysql.MySQLError); ok && me.Number == 1061 {
		return true // Duplicate key name
	}
	return false
}

func (d *mySQLDialect) InsertOrIgnorePrefix() string { return "INSERT IGNORE INTO" }

func (d *mySQLDialect) UpsertSuffix(_ string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf("%s = VALUES(%s)", c, c)
	}
	return "ON DUPLICATE KEY UPDATE " + strings.Join(parts, ", ")
}

func (d *mySQLDialect) ReplaceInto(table string) string {
	return "REPLACE INTO " + table
}

// ensureMySQLDatabase creates the target database if it does not already exist.
func ensureMySQLDatabase(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse mysql dsn: %w", err)
	}
	dbName := cfg.DBName
	if dbName == "" {
		return fmt.Errorf("mysql DSN must specify a database name")
	}
	cfg.DBName = ""
	tmp, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer tmp.Close()
	if _, err := tmp.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName)); err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}
	return nil
}
