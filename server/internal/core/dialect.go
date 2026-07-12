package core

import (
	"database/sql"
	"fmt"
	"strings"
)

// Column type constants — logical types that each dialect maps to its physical type.
const (
	CT_String     = "varchar_256"
	CT_String32   = "varchar_32"
	CT_String64   = "varchar_64"
	CT_String128  = "varchar_128"
	CT_String512  = "varchar_512"
	CT_String1024 = "varchar_1024"
	CT_Text       = "text_col"
	CT_MediumText = "mediumtext"
	CT_Int        = "int_col"
	CT_TinyInt    = "tinyint"
	CT_BigInt     = "bigint"      // BIGINT (MySQL) / INTEGER (SQLite) — non-auto-increment
	CT_BigIntAuto = "bigint_auto" // includes PRIMARY KEY AUTO_INCREMENT / AUTOINCREMENT
	CT_Double     = "double_col"
)

// TypeMap maps logical column type constants to physical SQL type names.
type TypeMap map[string]string

// Dialect abstracts database-specific SQL syntax and connection behaviour.
type Dialect interface {
	// DriverName returns the database/sql driver name (e.g. "sqlite", "mysql").
	DriverName() string

	// PreConnect runs any setup needed before sql.Open (e.g. mkdir, CREATE DATABASE).
	PreConnect(dsn string) error

	// ConfigurePool sets connection pool parameters on the opened database.
	ConfigurePool(db *sql.DB, dsn string)

	// Validate checks that the database is reachable (ping, PRAGMA, etc.).
	Validate(db *sql.DB) error

	// TypeMap returns the logical-to-physical column type mapping for this dialect.
	TypeMap() TypeMap

	// TableSuffix returns the ENGINE/CHARSET clause for CREATE TABLE (MySQL),
	// or an empty string (SQLite).
	TableSuffix() string

	// IndexIfNotExists returns true if CREATE INDEX should use IF NOT EXISTS (SQLite).
	IndexIfNotExists() bool

	// IsDuplicateIndexError returns true if err represents a duplicate-key/index error
	// that should be silently ignored during index creation.
	IsDuplicateIndexError(err error) bool

	// --- Query-building fragments ---

	// InsertOrIgnorePrefix returns the prefix for an idempotent insert.
	// SQLite: "INSERT OR IGNORE INTO" / MySQL: "INSERT IGNORE INTO"
	InsertOrIgnorePrefix() string

	// UpsertSuffix returns the suffix for an upsert (INSERT ... ON CONFLICT / ON DUPLICATE KEY UPDATE).
	// conflictCol is the primary key column name. cols lists the columns to update on conflict.
	UpsertSuffix(conflictCol string, cols []string) string

	// ReplaceInto returns the "REPLACE INTO" or "INSERT OR REPLACE INTO" prefix for the given table.
	ReplaceInto(table string) string
}

// NewDialect returns the Dialect implementation for the given driver.
// For DriverJSON, returns nil since JSON storage does not use SQL.
func NewDialect(driver Driver) (Dialect, error) {
	switch driver {
	case DriverSQLite:
		return &sqliteDialect{}, nil
	case DriverMySQL:
		return &mySQLDialect{}, nil
	case DriverJSON:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// --- Schema definition ---

type colDef struct {
	name     string
	colType  string // one of the CT_* constants
	nullable bool
	defVal   string // empty = no default
	unique   bool   // inline UNIQUE constraint
}

type tableDef struct {
	name    string
	columns []colDef
	suffix  bool // append TableSuffix()?
}

type indexDef struct {
	name  string
	table string
	cols  []string
}

type migrationDef struct {
	table   string
	col     string
	colType string
	defVal  string
}

type seedDef struct {
	table  string
	cols   []string
	values []string
}

// schema is the single source of truth for all table/index/migration/seed definitions.
var schema = struct {
	tables     []tableDef
	indexes    []indexDef
	migrations []migrationDef
	seeds      []seedDef
}{
	tables: []tableDef{
		{
			name: "users",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "username", colType: CT_String128, unique: true},
				{name: "display_name", colType: CT_String},
				{name: "password", colType: CT_String},
				{name: "roles", colType: CT_String512},
				{name: "must_change_password", colType: CT_TinyInt, defVal: "0"},
				{name: "status", colType: CT_String32, defVal: "'active'"},
				{name: "created_at", colType: CT_String32},
				{name: "updated_at", colType: CT_String32, defVal: "''"},
			},
			suffix: true,
		},
		{
			name: "devices",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "name", colType: CT_String},
				{name: "group_name", colType: CT_String128},
				{name: "profile", colType: CT_String64},
				{name: "version", colType: CT_String32},
				{name: "hostname", colType: CT_String},
				{name: "os", colType: CT_String64},
				{name: "ip", colType: CT_String64},
				{name: "cpu_cores", colType: CT_Int},
				{name: "memory_mb", colType: CT_Int},
				{name: "disk_total_gb", colType: CT_Int},
				{name: "cpu_usage", colType: CT_Double, defVal: "0"},
				{name: "memory_usage", colType: CT_Double, defVal: "0"},
				{name: "disk_usage", colType: CT_Double, defVal: "0"},
				{name: "status", colType: CT_String32},
				{name: "last_seen", colType: CT_String32},
				{name: "created_at", colType: CT_String32},
				{name: "updated_at", colType: CT_String32},
				{name: "credential_status", colType: CT_String32, defVal: "'pending_reenrollment'"},
				{name: "revoked_at", colType: CT_String32, defVal: "''"},
			},
			suffix: true,
		},
		{
			name: "agent_sessions",
			columns: []colDef{
				{name: "id", colType: CT_BigIntAuto},
				{name: "device_id", colType: CT_String64},
				{name: "remote_addr", colType: CT_String128},
				{name: "connected_at", colType: CT_String32},
				{name: "disconnected_at", colType: CT_String32, nullable: true},
			},
			suffix: true,
		},
		{
			name: "tasks",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "device_id", colType: CT_String64},
				{name: "group_name", colType: CT_String128},
				{name: "command", colType: CT_Text},
				{name: "kind", colType: CT_String32, defVal: "'ad_hoc'"},
				{name: "template_id", colType: CT_String64, defVal: "''"},
				{name: "executable", colType: CT_String512, defVal: "''"},
				{name: "args_json", colType: CT_String1024, defVal: "''"},
				{name: "timeout_seconds", colType: CT_Int, defVal: "30"},
				{name: "status", colType: CT_String32},
				{name: "requested_by", colType: CT_String128},
				{name: "created_at", colType: CT_String32},
				{name: "started_at", colType: CT_String32, nullable: true},
				{name: "finished_at", colType: CT_String32, nullable: true},
			},
			suffix: true,
		},
		{
			name: "task_results",
			columns: []colDef{
				{name: "task_id", colType: CT_String64},
				{name: "stdout", colType: CT_MediumText},
				{name: "stderr", colType: CT_MediumText},
				{name: "exit_code", colType: CT_Int},
				{name: "duration_ms", colType: CT_BigInt},
				{name: "created_at", colType: CT_String32},
			},
			suffix: true,
		},
		{
			name: "audit_logs",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "actor", colType: CT_String128},
				{name: "actor_id", colType: CT_String64, defVal: "''"},
				{name: "actor_role", colType: CT_String32, defVal: "''"},
				{name: "remote_addr", colType: CT_String128, defVal: "''"},
				{name: "request_id", colType: CT_String64, defVal: "''"},
				{name: "action", colType: CT_String64},
				{name: "device_id", colType: CT_String64, nullable: true},
				{name: "task_id", colType: CT_String64, nullable: true},
				{name: "status", colType: CT_String32},
				{name: "message", colType: CT_Text},
				{name: "created_at", colType: CT_String32},
			},
			suffix: true,
		},
		{
			name: "llm_config",
			columns: []colDef{
				{name: "id", colType: CT_Int, defVal: "1"},
				{name: "provider_url", colType: CT_String1024, defVal: "''"},
				{name: "api_key", colType: CT_String512, defVal: "''"},
				{name: "model", colType: CT_String128, defVal: "''"},
				{name: "provider_type", colType: CT_String32, defVal: "'openai'"},
				{name: "auto_execute_read_only", colType: CT_TinyInt, defVal: "0"},
				{name: "enabled", colType: CT_TinyInt, defVal: "0"},
				{name: "updated_at", colType: CT_String32, defVal: "''"},
			},
			suffix: true,
		},
		{
			name: "schema_migrations",
			columns: []colDef{
				{name: "version", colType: CT_Int},
				{name: "name", colType: CT_String},
				{name: "applied_at", colType: CT_String32},
			},
			suffix: true,
		},
		{
			name: "web_sessions",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "user_id", colType: CT_String64},
				{name: "token_hash", colType: CT_String64, unique: true},
				{name: "csrf_hash", colType: CT_String64},
				{name: "remote_addr", colType: CT_String128, defVal: "''"},
				{name: "user_agent", colType: CT_String512, defVal: "''"},
				{name: "created_at", colType: CT_String32},
				{name: "last_seen_at", colType: CT_String32},
				{name: "idle_expires_at", colType: CT_String32},
				{name: "absolute_expires_at", colType: CT_String32},
			},
			suffix: true,
		},
		{
			name: "enrollment_codes",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "code_hash", colType: CT_String64, unique: true},
				{name: "expires_at", colType: CT_String32},
				{name: "max_uses", colType: CT_Int, defVal: "1"},
				{name: "used_count", colType: CT_Int, defVal: "0"},
				{name: "created_by", colType: CT_String64},
				{name: "created_at", colType: CT_String32},
				{name: "revoked_at", colType: CT_String32, defVal: "''"},
			},
			suffix: true,
		},
		{
			name: "device_credentials",
			columns: []colDef{
				{name: "device_id", colType: CT_String64},
				{name: "secret_hash", colType: CT_String64},
				{name: "status", colType: CT_String32, defVal: "'active'"},
				{name: "created_at", colType: CT_String32},
				{name: "last_used_at", colType: CT_String32, defVal: "''"},
				{name: "revoked_at", colType: CT_String32, defVal: "''"},
			},
			suffix: true,
		},
		{
			name: "command_templates",
			columns: []colDef{
				{name: "id", colType: CT_String64},
				{name: "name", colType: CT_String},
				{name: "description", colType: CT_String512, defVal: "''"},
				{name: "os", colType: CT_String32, defVal: "'linux'"},
				{name: "executable", colType: CT_String512},
				{name: "args_json", colType: CT_String1024, defVal: "'[]'"},
				{name: "parameters_json", colType: CT_String1024, defVal: "'[]'"},
				{name: "requires_privilege", colType: CT_TinyInt, defVal: "0"},
				{name: "enabled", colType: CT_TinyInt, defVal: "1"},
				{name: "timeout_seconds", colType: CT_Int, defVal: "30"},
				{name: "created_at", colType: CT_String32},
				{name: "updated_at", colType: CT_String32},
			},
			suffix: true,
		},
	},
	indexes: []indexDef{
		{name: "idx_tasks_device_status", table: "tasks", cols: []string{"device_id", "status"}},
		{name: "idx_tasks_status_started", table: "tasks", cols: []string{"status", "started_at"}},
		{name: "idx_audit_logs_device", table: "audit_logs", cols: []string{"device_id"}},
		{name: "idx_devices_group", table: "devices", cols: []string{"group_name"}},
		{name: "idx_web_sessions_token", table: "web_sessions", cols: []string{"token_hash"}},
		{name: "idx_web_sessions_user", table: "web_sessions", cols: []string{"user_id"}},
		{name: "idx_enrollment_expiry", table: "enrollment_codes", cols: []string{"expires_at"}},
	},
	migrations: []migrationDef{
		{table: "users", col: "must_change_password", colType: CT_TinyInt, defVal: "0"},
		{table: "llm_config", col: "provider_type", colType: CT_String32, defVal: "'openai'"},
		{table: "llm_config", col: "auto_execute_read_only", colType: CT_TinyInt, defVal: "0"},
	},
	seeds: []seedDef{
		{
			table:  "llm_config",
			cols:   []string{"id", "provider_url", "api_key", "model", "provider_type", "auto_execute_read_only", "enabled", "updated_at"},
			values: []string{"1", "''", "''", "''", "'openai'", "0", "0", "''"},
		},
	},
}

// --- Schema builder functions ---

// buildDDL generates CREATE TABLE IF NOT EXISTS statements from the shared schema.
func buildDDL(d Dialect) []string {
	tm := d.TypeMap()
	suffix := d.TableSuffix()
	var stmts []string

	for _, t := range schema.tables {
		var cols []string
		isFirst := true
		for _, c := range t.columns {
			physType := tm[c.colType]
			parts := []string{c.name, physType}

			// The first column is the primary key. CT_BigIntAuto already includes PK syntax.
			if isFirst && c.colType != CT_BigIntAuto {
				parts = append(parts, "PRIMARY KEY")
			}
			isFirst = false

			if !c.nullable {
				parts = append(parts, "NOT NULL")
			}
			if c.unique {
				parts = append(parts, "UNIQUE")
			}
			if c.defVal != "" {
				parts = append(parts, "DEFAULT "+c.defVal)
			}
			cols = append(cols, strings.Join(parts, " "))
		}
		stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n\t%s\n)", t.name, strings.Join(cols, ",\n\t"))
		if t.suffix {
			stmt += suffix
		}
		stmts = append(stmts, stmt)
	}
	return stmts
}

// buildIndexes generates CREATE INDEX statements from the shared schema.
func buildIndexes(d Dialect) []string {
	ifNotExists := ""
	if d.IndexIfNotExists() {
		ifNotExists = " IF NOT EXISTS"
	}
	var stmts []string
	for _, idx := range schema.indexes {
		stmt := fmt.Sprintf("CREATE INDEX%s %s ON %s(%s)",
			ifNotExists, idx.name, idx.table, strings.Join(idx.cols, ", "))
		stmts = append(stmts, stmt)
	}
	return stmts
}

// buildMigrations generates ALTER TABLE statements from the shared schema.
func buildMigrations(d Dialect) []string {
	tm := d.TypeMap()
	var stmts []string
	for _, m := range schema.migrations {
		physType := tm[m.colType]
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NOT NULL DEFAULT %s",
			m.table, m.col, physType, m.defVal)
		stmts = append(stmts, stmt)
	}
	return stmts
}

// buildSeedSQL generates INSERT statements from the shared schema.
func buildSeedSQL(d Dialect) []string {
	var stmts []string
	for _, s := range schema.seeds {
		stmt := fmt.Sprintf("%s %s (%s) VALUES (%s)",
			d.InsertOrIgnorePrefix(), s.table, strings.Join(s.cols, ", "), strings.Join(s.values, ", "))
		stmts = append(stmts, stmt)
	}
	return stmts
}

// placeholders returns n comma-separated "?" parameter markers.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
