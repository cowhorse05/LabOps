package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type migrationColumn struct {
	table, name, logicalType, defaultValue string
}

var securityMigrationColumns = []migrationColumn{
	{"users", "must_change_password", CT_TinyInt, "0"},
	{"users", "status", CT_String32, "'active'"},
	{"users", "updated_at", CT_String32, "''"},
	{"devices", "credential_status", CT_String32, "'pending_reenrollment'"},
	{"devices", "revoked_at", CT_String32, "''"},
	{"tasks", "kind", CT_String32, "'ad_hoc'"},
	{"tasks", "template_id", CT_String64, "''"},
	{"tasks", "executable", CT_String512, "''"},
	{"tasks", "args_json", CT_String1024, "''"},
	{"tasks", "timeout_seconds", CT_Int, "30"},
	{"audit_logs", "actor_id", CT_String64, "''"},
	{"audit_logs", "actor_role", CT_String32, "''"},
	{"audit_logs", "remote_addr", CT_String128, "''"},
	{"audit_logs", "request_id", CT_String64, "''"},
	{"llm_config", "provider_type", CT_String32, "'openai'"},
	{"llm_config", "auto_execute_read_only", CT_TinyInt, "0"},
}

func (s *Store) applyVersionedMigrations(ctx context.Context) error {
	var applied int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&applied)
	if err != nil {
		return fmt.Errorf("read schema migration state: %w", err)
	}
	if applied > 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration 1: %w", err)
	}
	defer tx.Rollback()
	for _, column := range securityMigrationColumns {
		exists, err := columnExists(ctx, tx, s.dialect, column.table, column.name)
		if err != nil {
			return fmt.Errorf("inspect %s.%s: %w", column.table, column.name, err)
		}
		if exists {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NOT NULL DEFAULT %s",
			column.table, column.name, s.dialect.TypeMap()[column.logicalType], column.defaultValue)
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration 1 add %s.%s: %w", column.table, column.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
		1, "secure_sessions_agent_identity_templates", nowString()); err != nil {
		return fmt.Errorf("record schema migration 1: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migration 1: %w", err)
	}
	return nil
}

func columnExists(ctx context.Context, tx *sql.Tx, dialect Dialect, table, column string) (bool, error) {
	if dialect.DriverName() == "sqlite" {
		rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
		if err != nil {
			return false, err
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue any
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
				return false, err
			}
			if name == column {
				return true, nil
			}
		}
		return false, rows.Err()
	}
	var count int
	err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		table, column).Scan(&count)
	return count > 0, err
}

func (s *Store) seedCommandTemplates(ctx context.Context) error {
	templates := []CommandTemplate{
		{ID: "tpl_hostname", Name: "主机名", Description: "显示设备主机名", OS: "linux", Executable: "/bin/hostname", Args: []string{}, Enabled: true, TimeoutSeconds: 15},
		{ID: "tpl_uptime", Name: "在线时长", Description: "显示系统在线时间和负载", OS: "linux", Executable: "/usr/bin/uptime", Args: []string{}, Enabled: true, TimeoutSeconds: 15},
		{ID: "tpl_disk", Name: "磁盘占用", Description: "显示文件系统磁盘用量", OS: "linux", Executable: "/bin/df", Args: []string{"-h"}, Enabled: true, TimeoutSeconds: 30},
		{ID: "tpl_memory", Name: "内存占用", Description: "显示系统内存用量", OS: "linux", Executable: "/usr/bin/free", Args: []string{"-h"}, Enabled: true, TimeoutSeconds: 15},
		{ID: "tpl_processes", Name: "进程摘要", Description: "按 CPU 使用率显示进程摘要", OS: "linux", Executable: "/bin/ps", Args: []string{"-eo", "pid,user,pcpu,pmem,comm", "--sort=-pcpu"}, Enabled: true, TimeoutSeconds: 30},
	}
	for _, template := range templates {
		argsJSON, _ := json.Marshal(template.Args)
		paramsJSON, _ := json.Marshal(template.Parameters)
		now := nowString()
		_, err := s.db.ExecContext(ctx, s.dialect.InsertOrIgnorePrefix()+` command_templates
			(id, name, description, os, executable, args_json, parameters_json, requires_privilege, enabled, timeout_seconds, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			template.ID, template.Name, template.Description, template.OS, template.Executable,
			string(argsJSON), string(paramsJSON), 0, 1, template.TimeoutSeconds, now, now)
		if err != nil {
			return fmt.Errorf("seed command template %s: %w", template.ID, err)
		}
	}
	return nil
}
