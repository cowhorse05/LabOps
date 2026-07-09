package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Allow multiple concurrent reads for file-based databases (read-heavy
	// workloads). For :memory: databases, keep MaxOpenConns=1 because each
	// connection creates its own in-memory database.
	maxOpen := 1
	if path != ":memory:" {
		maxOpen = 4
	}
	db.SetMaxOpenConns(maxOpen)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Init(ctx context.Context) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password TEXT NOT NULL,
			roles TEXT NOT NULL,
			must_change_password INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_name TEXT NOT NULL,
			profile TEXT NOT NULL,
			version TEXT NOT NULL,
			hostname TEXT NOT NULL,
			os TEXT NOT NULL,
			ip TEXT NOT NULL,
			cpu_cores INTEGER NOT NULL,
			memory_mb INTEGER NOT NULL,
			disk_total_gb INTEGER NOT NULL,
			cpu_usage REAL NOT NULL DEFAULT 0,
			memory_usage REAL NOT NULL DEFAULT 0,
			disk_usage REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agent_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			connected_at TEXT NOT NULL,
			disconnected_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL,
			group_name TEXT NOT NULL,
			command TEXT NOT NULL,
			status TEXT NOT NULL,
			requested_by TEXT NOT NULL,
			created_at TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS task_results (
			task_id TEXT PRIMARY KEY,
			stdout TEXT NOT NULL,
			stderr TEXT NOT NULL,
			exit_code INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			device_id TEXT,
			task_id TEXT,
			status TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range schema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_tasks_device_status ON tasks(device_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status_started ON tasks(status, started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_device ON audit_logs(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_name)`,
	}
	for _, stmt := range indexes {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	// Migrate: add must_change_password column for existing databases
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0`)
	hashed, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO users (id, username, display_name, password, roles, must_change_password, created_at)
		VALUES ('user_admin', 'admin', 'LabOps Admin', ?, 'admin,operator', 1, ?)`, string(hashed), nowString())
	return err
}

func (s *Store) FindUser(ctx context.Context, username, password string) (User, bool, error) {
	var roles string
	var storedHash string
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, password, roles FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.DisplayName, &storedHash, &roles)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
		return User{}, false, nil
	}
	user.Roles = splitRoles(roles)
	return user, true, nil
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (User, bool, error) {
	var roles string
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, roles FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.DisplayName, &roles)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Roles = splitRoles(roles)
	return user, true, nil
}

func (s *Store) AdminUser() User {
	return User{ID: "user_admin", Username: "admin", DisplayName: "LabOps Admin", Roles: []string{"admin", "operator"}}
}

func (s *Store) UpdatePassword(ctx context.Context, username, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"UPDATE users SET password = ?, must_change_password = 0 WHERE username = ?",
		string(hashed), username)
	return err
}

func (s *Store) MustChangePassword(ctx context.Context, username string) bool {
	var must int
	err := s.db.QueryRowContext(ctx, "SELECT must_change_password FROM users WHERE username = ?", username).Scan(&must)
	return err == nil && must == 1
}

func (s *Store) UpsertDevice(ctx context.Context, d Device) error {
	now := nowString()
	if d.CreatedAt == "" {
		d.CreatedAt = now
	}
	if d.UpdatedAt == "" {
		d.UpdatedAt = now
	}
	if d.LastSeen == "" {
		d.LastSeen = now
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO devices (
		id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		group_name = excluded.group_name,
		profile = excluded.profile,
		version = excluded.version,
		hostname = excluded.hostname,
		os = excluded.os,
		ip = excluded.ip,
		cpu_cores = excluded.cpu_cores,
		memory_mb = excluded.memory_mb,
		disk_total_gb = excluded.disk_total_gb,
		cpu_usage = excluded.cpu_usage,
		memory_usage = excluded.memory_usage,
		disk_usage = excluded.disk_usage,
		status = excluded.status,
		last_seen = excluded.last_seen,
		updated_at = excluded.updated_at`,
		d.ID, d.Name, d.GroupName, d.Profile, d.Version, d.Hostname, d.OS, d.IP, d.CPUCores, d.MemoryMB, d.DiskTotalGB,
		d.CPUUsage, d.MemoryUsage, d.DiskUsage, d.Status, d.LastSeen, d.CreatedAt, d.UpdatedAt)
	return err
}

func (s *Store) UpdateHeartbeat(ctx context.Context, deviceID string, hb HeartbeatPayload) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET cpu_usage = ?, memory_usage = ?, disk_usage = ?, status = ?, last_seen = ?, updated_at = ? WHERE id = ?`,
		hb.CPUUsage, hb.MemoryUsage, hb.DiskUsage, StatusOnline, nowString(), nowString(), deviceID)
	return err
}

func (s *Store) MarkDeviceOffline(ctx context.Context, deviceID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET status = ?, updated_at = ? WHERE id = ?`, StatusOffline, nowString(), deviceID)
	return err
}

func (s *Store) ExpireDevices(ctx context.Context, cutoff string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET status = ?, updated_at = ? WHERE status = ? AND last_seen < ?`,
		StatusOffline, nowString(), StatusOnline, cutoff)
	return err
}

func (s *Store) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at
		FROM devices ORDER BY group_name, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) ListDevicesByGroup(ctx context.Context, groupName string) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at
		FROM devices WHERE group_name = ? ORDER BY name`, groupName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, id string) (Device, bool, error) {
	var d Device
	err := s.db.QueryRowContext(ctx, `SELECT id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at
		FROM devices WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, false, nil
	}
	if err != nil {
		return Device{}, false, err
	}
	return d, true, nil
}

func (s *Store) Stats(ctx context.Context) (DeviceStats, error) {
	var stats DeviceStats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status != 'online' THEN 1 ELSE 0 END), 0)
		FROM devices`).Scan(&stats.Total, &stats.Online, &stats.Offline)
	return stats, err
}

func (s *Store) Groups(ctx context.Context) ([]DeviceGroup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_name, COUNT(*),
		COALESCE(SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END), 0)
		FROM devices GROUP BY group_name ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []DeviceGroup
	for rows.Next() {
		var g DeviceGroup
		if err := rows.Scan(&g.Name, &g.Total, &g.Online); err != nil {
			return nil, err
		}
		g.Description = fmt.Sprintf("%d online / %d total", g.Online, g.Total)
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) CreateSession(ctx context.Context, deviceID, remoteAddr string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO agent_sessions (device_id, remote_addr, connected_at) VALUES (?, ?, ?)`, deviceID, remoteAddr, nowString())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CloseSession(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agent_sessions SET disconnected_at = ? WHERE id = ?`, nowString(), id)
	return err
}

func (s *Store) CreateTask(ctx context.Context, deviceID, groupName, command, requestedBy string) (Task, error) {
	task := Task{
		ID:          newID("task"),
		DeviceID:    deviceID,
		GroupName:   groupName,
		Command:     command,
		Status:      StatusPending,
		RequestedBy: requestedBy,
		CreatedAt:   nowString(),
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks (id, device_id, group_name, command, status, requested_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, task.ID, task.DeviceID, task.GroupName, task.Command, task.Status, task.RequestedBy, task.CreatedAt)
	return task, err
}

func (s *Store) MarkTaskRunning(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`,
		StatusRunning, nowString(), taskID)
	return err
}

func (s *Store) FailTask(ctx context.Context, taskID, stderr string) error {
	now := nowString()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE tasks SET status = ?, finished_at = ? WHERE id = ?`, StatusFailed, now, taskID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO task_results (task_id, stdout, stderr, exit_code, duration_ms, created_at)
		VALUES (?, '', ?, -1, 0, ?)`, taskID, stderr, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteTask(ctx context.Context, result TaskResultPayload) error {
	status := result.Status
	if status == "" {
		if result.ExitCode == 0 {
			status = StatusSuccess
		} else {
			status = StatusFailed
		}
	}
	now := nowString()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE tasks SET status = ?, finished_at = ? WHERE id = ? AND status NOT IN (?, ?, ?)`,
		status, now, result.TaskID, StatusSuccess, StatusFailed, StatusTimeout)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return tx.Commit() // already in a terminal state
	}
	_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO task_results (task_id, stdout, stderr, exit_code, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, result.TaskID, result.Stdout, result.Stderr, result.ExitCode, result.DurationMS, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) TimeoutTasks(ctx context.Context, cutoff string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, finished_at = ? WHERE status = ? AND started_at < ?`,
		StatusTimeout, nowString(), StatusRunning, cutoff)
	return err
}

func (s *Store) PendingTasksForDevice(ctx context.Context, deviceID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, device_id, group_name, command, status, requested_by, created_at, started_at, finished_at
		FROM tasks WHERE device_id = ? AND status = ? ORDER BY created_at`, deviceID, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) ListTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.status, t.requested_by,
		t.created_at, t.started_at, t.finished_at, r.stdout, r.stderr, r.exit_code, r.duration_ms, r.created_at
		FROM tasks t
		LEFT JOIN devices d ON d.id = t.device_id
		LEFT JOIN task_results r ON r.task_id = t.id
		ORDER BY t.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTaskWithResult(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) ListTasksByDevice(ctx context.Context, deviceID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.status, t.requested_by,
		t.created_at, t.started_at, t.finished_at, r.stdout, r.stderr, r.exit_code, r.duration_ms, r.created_at
		FROM tasks t
		LEFT JOIN devices d ON d.id = t.device_id
		LEFT JOIN task_results r ON r.task_id = t.id
		WHERE t.device_id = ?
		ORDER BY t.created_at DESC LIMIT 50`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTaskWithResult(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) GetTask(ctx context.Context, id string) (Task, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.status, t.requested_by,
		t.created_at, t.started_at, t.finished_at, r.stdout, r.stderr, r.exit_code, r.duration_ms, r.created_at
		FROM tasks t
		LEFT JOIN devices d ON d.id = t.device_id
		LEFT JOIN task_results r ON r.task_id = t.id
		WHERE t.id = ?`, id)
	task, err := scanTaskWithResult(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, false, err
	}
	return task, true, nil
}

func (s *Store) CreateAudit(ctx context.Context, audit AuditLog) error {
	if audit.ID == "" {
		audit.ID = newID("audit")
	}
	if audit.CreatedAt == "" {
		audit.CreatedAt = nowString()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs (id, actor, action, device_id, task_id, status, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, audit.ID, audit.Actor, audit.Action, audit.DeviceID, audit.TaskID, audit.Status, audit.Message, audit.CreatedAt)
	return err
}

func (s *Store) ListAudit(ctx context.Context) ([]AuditLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id, a.actor, a.action, COALESCE(a.device_id, ''), COALESCE(d.name, ''), COALESCE(a.task_id, ''),
		a.status, a.message, a.created_at
		FROM audit_logs a
		LEFT JOIN devices d ON d.id = a.device_id
		ORDER BY a.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var audits []AuditLog
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.DeviceID, &a.Device, &a.TaskID, &a.Status, &a.Message, &a.CreatedAt); err != nil {
			return nil, err
		}
		audits = append(audits, a)
	}
	return audits, rows.Err()
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTaskWithResult(row taskScanner) (Task, error) {
	var task Task
	var startedAt, finishedAt sql.NullString
	var stdout, stderr, resultAt sql.NullString
	var exitCode sql.NullInt64
	var duration sql.NullInt64
	err := row.Scan(&task.ID, &task.DeviceID, &task.DeviceName, &task.GroupName, &task.Command, &task.Status, &task.RequestedBy,
		&task.CreatedAt, &startedAt, &finishedAt, &stdout, &stderr, &exitCode, &duration, &resultAt)
	if err != nil {
		return Task{}, err
	}
	task.StartedAt = nullableString(startedAt)
	task.FinishedAt = nullableString(finishedAt)
	if resultAt.Valid {
		task.Result = &TaskResult{
			TaskID:     task.ID,
			Stdout:     nullableString(stdout),
			Stderr:     nullableString(stderr),
			ExitCode:   int(exitCode.Int64),
			DurationMS: duration.Int64,
			CreatedAt:  resultAt.String,
		}
	}
	return task, nil
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var task Task
		var startedAt, finishedAt sql.NullString
		if err := rows.Scan(&task.ID, &task.DeviceID, &task.GroupName, &task.Command, &task.Status, &task.RequestedBy, &task.CreatedAt, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		task.StartedAt = nullableString(startedAt)
		task.FinishedAt = nullableString(finishedAt)
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func nullableString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func splitRoles(roles string) []string {
	parts := strings.Split(roles, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
