package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

// Driver identifies the database backend.
type Driver string

const (
	DriverSQLite Driver = "sqlite"
	DriverMySQL  Driver = "mysql"
	DriverJSON   Driver = "jsonfile"
)

// DataStore is the persistence contract used by the application.
// Both *Store (SQL-backed) and *JSONStore (file-backed) implement it.
type DataStore interface {
	// Users
	CountUsers(ctx context.Context) (int, error)
	SetupStatus(ctx context.Context) (SetupStatus, error)
	FindUser(ctx context.Context, username, password string) (User, bool, error)
	FindUserByUsername(ctx context.Context, username string) (User, bool, error)
	AdminUser() User
	CreateUser(ctx context.Context, username, displayName, password, role string) (User, error)
	UpdatePassword(ctx context.Context, username, newPassword string) error
	ChangePassword(ctx context.Context, username, newPassword, userID string) (string, string, error)
	MustChangePassword(ctx context.Context, username string) bool
	UpdateUserAccess(ctx context.Context, id, role, status string) error
	ListUsers(ctx context.Context) ([]User, error)
	TryCreateInitialAdmin(ctx context.Context, username, displayName, password string) (User, error)
	BootstrapFirstAdmin(ctx context.Context, input BootstrapAdminInput) (User, error)

	// Sessions
	CreateWebSession(ctx context.Context, userID, remoteAddr, userAgent string) (string, string, error)
	AuthenticateWebSession(ctx context.Context, rawToken string) (User, string, string, bool, error)
	DeleteWebSession(ctx context.Context, sessionID string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	PruneExpiredWebSessions(ctx context.Context) error

	// Agent Sessions
	CreateSession(ctx context.Context, deviceID, remoteAddr string) (int64, error)
	CloseSession(ctx context.Context, id int64) error

	// Devices
	UpsertDevice(ctx context.Context, d Device) error
	CreateDevice(ctx context.Context, d Device) error
	DeleteDevice(ctx context.Context, id string) error
	UpdateHeartbeat(ctx context.Context, deviceID string, hb HeartbeatPayload) error
	MarkDeviceOffline(ctx context.Context, deviceID string) error
	ExpireDevices(ctx context.Context, cutoff string) error
	ListDevices(ctx context.Context) ([]Device, error)
	ListDevicesByGroup(ctx context.Context, groupName string) ([]Device, error)
	GetDevice(ctx context.Context, id string) (Device, bool, error)
	Stats(ctx context.Context) (DeviceStats, error)
	Groups(ctx context.Context) ([]DeviceGroup, error)

	// Tasks
	CreateTask(ctx context.Context, deviceID, groupName, command, requestedBy string) (Task, error)
	CreateTaskSpec(ctx context.Context, task Task) (Task, error)
	GetTask(ctx context.Context, id string) (Task, bool, error)
	ListTasks(ctx context.Context) ([]Task, error)
	ListTasksByDevice(ctx context.Context, deviceID string) ([]Task, error)
	MarkTaskRunning(ctx context.Context, taskID string) error
	CompleteTask(ctx context.Context, result TaskResultPayload) error
	FailTask(ctx context.Context, taskID, stderr string) error
	TimeoutTasks(ctx context.Context, cutoff string) error
	PendingTasksForDevice(ctx context.Context, deviceID string) ([]Task, error)

	// Audit
	CreateAudit(ctx context.Context, audit AuditLog) error
	ListAudit(ctx context.Context, filter AuditFilter) ([]AuditLog, error)

	// LLM Config
	GetLLMConfig(ctx context.Context) (LLMConfig, error)
	SaveLLMConfig(ctx context.Context, cfg LLMConfig) error

	// Enrollment
	CreateEnrollmentCode(ctx context.Context, createdBy string, ttl time.Duration, maxUses int) (EnrollmentCode, error)
	ListEnrollmentCodes(ctx context.Context) ([]EnrollmentCode, error)
	RevokeEnrollmentCode(ctx context.Context, id string) error
	EnrollDevice(ctx context.Context, rawCode string, reg RegisterPayload) (Device, string, error)
	ValidateDeviceCredential(ctx context.Context, deviceID, secret string) (bool, error)
	RevokeDeviceCredential(ctx context.Context, deviceID string) error

	// Command Templates
	ListCommandTemplates(ctx context.Context) ([]CommandTemplate, error)
	GetCommandTemplate(ctx context.Context, id string) (CommandTemplate, bool, error)
	SaveCommandTemplate(ctx context.Context, item CommandTemplate) (CommandTemplate, error)

	// Lifecycle
	Close() error
	ConfigureEncryptionKey(raw string) error
	ProtectStoredLLMSecret(ctx context.Context) error
	Init(ctx context.Context) error
	InitSecure(ctx context.Context, bootstrapPassword string) error
}

// Compile-time interface satisfaction checks.
var _ DataStore = (*Store)(nil)
var _ DataStore = (*JSONStore)(nil)

type Store struct {
	db            *sql.DB
	dialect       Dialect
	encryptionKey []byte
	bootstrapMu   sync.Mutex
}

func OpenStore(driver Driver, dsn string) (*Store, error) {
	dialect, err := NewDialect(driver)
	if err != nil {
		return nil, err
	}
	if err := dialect.PreConnect(dsn); err != nil {
		return nil, fmt.Errorf("pre-connect: %w", err)
	}
	db, err := sql.Open(dialect.DriverName(), dsn)
	if err != nil {
		return nil, err
	}
	dialect.ConfigurePool(db, dsn)
	if err := dialect.Validate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("validate: %w", err)
	}
	return &Store{db: db, dialect: dialect}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Init(ctx context.Context) error {
	if err := s.initSchema(ctx); err != nil {
		return err
	}
	// Init is retained for package tests and local examples. Production startup
	// uses InitSecure and never creates a known default password.
	return s.bootstrapAdmin(ctx, "admin")
}

func (s *Store) InitSecure(ctx context.Context, bootstrapPassword string) error {
	if err := s.initSchema(ctx); err != nil {
		return err
	}
	status, err := s.SetupStatus(ctx)
	if err != nil {
		return err
	}
	if !status.ActiveAdminExists {
		if len(bootstrapPassword) >= 12 {
			return s.bootstrapAdmin(ctx, bootstrapPassword)
		}
		// No bootstrap password and no active administrator — setup API will
		// handle first-admin creation or recovery.
		return nil
	}
	if _, usesDefault, err := s.FindUser(ctx, "admin", "admin"); err != nil {
		return err
	} else if usesDefault {
		if len(bootstrapPassword) < 12 {
			return fmt.Errorf("existing admin still uses the legacy default password; set LABOPS_BOOTSTRAP_ADMIN_PASSWORD to replace it")
		}
		return s.setPassword(ctx, "admin", bootstrapPassword, true)
	}
	return nil
}

func (s *Store) initSchema(ctx context.Context) error {
	for _, stmt := range buildDDL(s.dialect) {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	if err := s.applyVersionedMigrations(ctx); err != nil {
		return err
	}
	for _, stmt := range buildIndexes(s.dialect) {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			if !s.dialect.IsDuplicateIndexError(err) {
				return err
			}
		}
	}
	for _, stmt := range buildSeedSQL(s.dialect) {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("seed schema: %w", err)
		}
	}
	return s.seedCommandTemplates(ctx)
}

func (s *Store) bootstrapAdmin(ctx context.Context, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	now := nowString()
	_, err = s.db.ExecContext(ctx,
		s.dialect.InsertOrIgnorePrefix()+" users (id, username, display_name, password, roles, must_change_password, status, created_at, updated_at) VALUES ('user_admin', 'admin', 'LabOps Admin', ?, 'admin', 1, 'active', ?, ?)",
		string(hashed), now, now)
	return err
}

// ErrAlreadyInitialized is returned by TryCreateInitialAdmin when the system
// already has one or more user accounts.
var ErrAlreadyInitialized = errors.New("system is already initialized")
var ErrUsernameExists = errors.New("username already exists")

// CountUsers returns the number of user records in the database.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (s *Store) SetupStatus(ctx context.Context) (SetupStatus, error) {
	var status SetupStatus
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&status.TotalUsers); err != nil {
		return status, fmt.Errorf("count users: %w", err)
	}
	var adminCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE roles = ?", RoleAdmin).Scan(&adminCount); err != nil {
		return status, fmt.Errorf("count admins: %w", err)
	}
	var activeAdminCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE roles = ? AND status = 'active'", RoleAdmin).Scan(&activeAdminCount); err != nil {
		return status, fmt.Errorf("count active admins: %w", err)
	}
	status.AdminExists = adminCount > 0
	status.ActiveAdminExists = activeAdminCount > 0
	status.Initialized = status.ActiveAdminExists
	status.RegistrationAllowed = !status.ActiveAdminExists
	status.RecoveryRequired = status.TotalUsers > 0 && !status.ActiveAdminExists
	return status, nil
}

// TryCreateInitialAdmin creates the first admin user atomically. If the users
// table already contains rows, it returns ErrAlreadyInitialized. The method
// uses a database transaction to prevent concurrent first-admin creation.
func (s *Store) TryCreateInitialAdmin(ctx context.Context, username, displayName, password string) (User, error) {
	return s.BootstrapFirstAdmin(ctx, BootstrapAdminInput{Username: username, DisplayName: displayName, Password: password, ConfirmPassword: password})
}

func (s *Store) BootstrapFirstAdmin(ctx context.Context, input BootstrapAdminInput) (User, error) {
	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var activeAdmins int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE roles = ? AND status = 'active'", RoleAdmin).Scan(&activeAdmins); err != nil {
		return User{}, fmt.Errorf("count active admins: %w", err)
	}
	if activeAdmins > 0 {
		return User{}, ErrAlreadyInitialized
	}
	var sameUsername int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = ?", input.Username).Scan(&sameUsername); err != nil {
		return User{}, fmt.Errorf("count username: %w", err)
	}
	if sameUsername > 0 {
		return User{}, ErrUsernameExists
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return User{}, err
	}
	now := nowString()
	id := newID("user")
	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, username, display_name, password, roles, must_change_password, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'admin', 0, 'active', ?, ?)`,
		id, input.Username, input.DisplayName, string(hashed), now, now)
	if err != nil {
		return User{}, fmt.Errorf("insert initial admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit: %w", err)
	}

	user := User{ID: id, Username: input.Username, DisplayName: input.DisplayName, Status: "active"}
	applyUserAuthorization(&user, RoleAdmin)
	return user, nil
}

func (s *Store) FindUser(ctx context.Context, username, password string) (User, bool, error) {
	var roles string
	var storedHash string
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, password, roles, status FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.DisplayName, &storedHash, &roles, &user.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
		return User{}, false, nil
	}
	if user.Status != "active" {
		return User{}, false, nil
	}
	applyUserAuthorization(&user, roles)
	return user, true, nil
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (User, bool, error) {
	var roles string
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, roles, status FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.DisplayName, &roles, &user.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	applyUserAuthorization(&user, roles)
	return user, true, nil
}

func (s *Store) AdminUser() User {
	user := User{ID: "user_admin", Username: "admin", DisplayName: "LabOps Admin", Status: "active"}
	applyUserAuthorization(&user, RoleAdmin)
	return user
}

func (s *Store) UpdatePassword(ctx context.Context, username, newPassword string) error {
	return s.setPassword(ctx, username, newPassword, false)
}

// ChangePassword updates the user's password and replaces all sessions atomically.
// It runs in a single transaction: update password → delete old sessions → create new session.
// Returns the new session token and CSRF token for immediate use by the caller.
func (s *Store) ChangePassword(ctx context.Context, username, newPassword, userID string) (string, string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return "", "", err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	// Step 1: Update password, clear must_change_password flag.
	now := nowString()
	_, err = tx.ExecContext(ctx,
		"UPDATE users SET password = ?, must_change_password = 0, updated_at = ? WHERE username = ?",
		string(hashed), now, username)
	if err != nil {
		return "", "", fmt.Errorf("change password: update: %w", err)
	}

	// Step 2: Delete all existing sessions for this user.
	_, err = tx.ExecContext(ctx, "DELETE FROM web_sessions WHERE user_id = ?", userID)
	if err != nil {
		return "", "", fmt.Errorf("change password: delete sessions: %w", err)
	}

	// Step 3: Create a new session within the transaction.
	token, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	sessionID := newID("session")
	t := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO web_sessions
		(id, user_id, token_hash, csrf_hash, remote_addr, user_agent, created_at, last_seen_at, idle_expires_at, absolute_expires_at)
		VALUES (?, ?, ?, ?, '', '', ?, ?, ?, ?)`,
		sessionID, userID, tokenHash(token), tokenHash(csrf),
		t.Format(time.RFC3339), t.Format(time.RFC3339),
		t.Add(8*time.Hour).Format(time.RFC3339), t.Add(24*time.Hour).Format(time.RFC3339))
	if err != nil {
		return "", "", fmt.Errorf("change password: create session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("change password: commit: %w", err)
	}
	return token, csrf, nil
}

func (s *Store) setPassword(ctx context.Context, username, newPassword string, mustChange bool) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	must := 0
	if mustChange {
		must = 1
	}
	_, err = s.db.ExecContext(ctx,
		"UPDATE users SET password = ?, must_change_password = ?, updated_at = ? WHERE username = ?",
		string(hashed), must, nowString(), username)
	return err
}

func (s *Store) MustChangePassword(ctx context.Context, username string) bool {
	var must int
	err := s.db.QueryRowContext(ctx, "SELECT must_change_password FROM users WHERE username = ?", username).Scan(&must)
	return err == nil && must == 1
}

// GetLLMConfig returns the stored LLM configuration.
func (s *Store) GetLLMConfig(ctx context.Context) (LLMConfig, error) {
	var cfg LLMConfig
	var enabled int
	var autoExec int
	err := s.db.QueryRowContext(ctx,
		`SELECT provider_url, api_key, model, provider_type, enabled, auto_execute_read_only, updated_at FROM llm_config WHERE id = 1`).
		Scan(&cfg.ProviderURL, &cfg.APIKey, &cfg.Model, &cfg.ProviderType, &enabled, &autoExec, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LLMConfig{}, nil
		}
		return LLMConfig{}, err
	}
	cfg.Enabled = enabled == 1
	cfg.AutoExecuteReadOnly = autoExec == 1
	if cfg.APIKey != "" {
		plain, err := s.decryptSecret(cfg.APIKey)
		if err != nil {
			return LLMConfig{}, fmt.Errorf("decrypt LLM API key: %w", err)
		}
		cfg.APIKey = plain
	}
	return cfg, nil
}

// SaveLLMConfig persists the LLM configuration (upsert into the single row).
func (s *Store) SaveLLMConfig(ctx context.Context, cfg LLMConfig) error {
	cfg.UpdatedAt = nowString()
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	autoExec := 0
	if cfg.AutoExecuteReadOnly {
		autoExec = 1
	}
	cfgCols := []string{"provider_url", "api_key", "model", "provider_type", "enabled", "auto_execute_read_only", "updated_at"}
	allCols := append([]string{"id"}, cfgCols...)
	query := fmt.Sprintf("INSERT INTO llm_config (%s) VALUES (%s) %s",
		strings.Join(allCols, ", "),
		placeholders(len(allCols)),
		s.dialect.UpsertSuffix("id", cfgCols))
	storedKey, err := s.encryptSecret(cfg.APIKey)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, query,
		1, cfg.ProviderURL, storedKey, cfg.Model, cfg.ProviderType, enabled, autoExec, cfg.UpdatedAt)
	return err
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

	devCols := []string{"name", "group_name", "profile", "version", "hostname", "os", "ip",
		"cpu_cores", "memory_mb", "disk_total_gb", "cpu_usage", "memory_usage", "disk_usage",
		"status", "last_seen", "created_at", "updated_at"}
	allCols := append([]string{"id"}, devCols...)
	query := fmt.Sprintf("INSERT INTO devices (%s) VALUES (%s) %s",
		strings.Join(allCols, ", "),
		placeholders(len(allCols)),
		s.dialect.UpsertSuffix("id", devCols))
	_, err := s.db.ExecContext(ctx, query,
		d.ID, d.Name, d.GroupName, d.Profile, d.Version, d.Hostname, d.OS, d.IP,
		d.CPUCores, d.MemoryMB, d.DiskTotalGB,
		d.CPUUsage, d.MemoryUsage, d.DiskUsage,
		d.Status, d.LastSeen, d.CreatedAt, d.UpdatedAt)
	return err
}

// CreateDevice inserts a new device. It fails if a device with the same id
// already exists (pure INSERT, not upsert).
func (s *Store) CreateDevice(ctx context.Context, d Device) error {
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

	_, err := s.db.ExecContext(ctx, `INSERT INTO devices (id, name, group_name, profile, version, hostname, os, ip,
		cpu_cores, memory_mb, disk_total_gb, cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.GroupName, d.Profile, d.Version, d.Hostname, d.OS, d.IP,
		d.CPUCores, d.MemoryMB, d.DiskTotalGB,
		d.CPUUsage, d.MemoryUsage, d.DiskUsage,
		d.Status, d.LastSeen, d.CreatedAt, d.UpdatedAt)
	return err
}

// DeleteDevice removes a device from the database. The agent can re-create
// the device on reconnect via UpsertDevice (soft delete semantics).
func (s *Store) DeleteDevice(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateHeartbeat(ctx context.Context, deviceID string, hb HeartbeatPayload) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET cpu_usage = ?, memory_usage = ?, disk_usage = ?, status = ?, last_seen = ?, updated_at = ? WHERE id = ? AND credential_status != 'revoked'`,
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
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at, credential_status, revoked_at
		FROM devices ORDER BY group_name, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt, &d.CredentialStatus, &d.RevokedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) ListDevicesByGroup(ctx context.Context, groupName string) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at, credential_status, revoked_at
		FROM devices WHERE group_name = ? ORDER BY name`, groupName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt, &d.CredentialStatus, &d.RevokedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, id string) (Device, bool, error) {
	var d Device
	err := s.db.QueryRowContext(ctx, `SELECT id, name, group_name, profile, version, hostname, os, ip, cpu_cores, memory_mb, disk_total_gb,
		cpu_usage, memory_usage, disk_usage, status, last_seen, created_at, updated_at, credential_status, revoked_at
		FROM devices WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.GroupName, &d.Profile, &d.Version, &d.Hostname, &d.OS, &d.IP, &d.CPUCores, &d.MemoryMB, &d.DiskTotalGB,
			&d.CPUUsage, &d.MemoryUsage, &d.DiskUsage, &d.Status, &d.LastSeen, &d.CreatedAt, &d.UpdatedAt, &d.CredentialStatus, &d.RevokedAt)
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
		ID:             newID("task"),
		DeviceID:       deviceID,
		GroupName:      groupName,
		Command:        command,
		Kind:           TaskKindAdHoc,
		TimeoutSeconds: 30,
		Status:         StatusPending,
		RequestedBy:    requestedBy,
		CreatedAt:      nowString(),
	}
	return s.CreateTaskSpec(ctx, task)
}

func (s *Store) CreateTaskSpec(ctx context.Context, task Task) (Task, error) {
	if task.ID == "" {
		task.ID = newID("task")
	}
	if task.Kind == "" {
		task.Kind = TaskKindAdHoc
	}
	if task.TimeoutSeconds < 1 {
		task.TimeoutSeconds = 30
	}
	if task.TimeoutSeconds > 300 {
		task.TimeoutSeconds = 300
	}
	if task.Status == "" {
		task.Status = StatusPending
	}
	if task.CreatedAt == "" {
		task.CreatedAt = nowString()
	}
	argsJSON, err := json.Marshal(task.Args)
	if err != nil {
		return Task{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO tasks
		(id, device_id, group_name, command, kind, template_id, executable, args_json, timeout_seconds, status, requested_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.ID, task.DeviceID, task.GroupName, task.Command, task.Kind,
		task.TemplateID, task.Executable, string(argsJSON), task.TimeoutSeconds, task.Status, task.RequestedBy, task.CreatedAt)
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
	query := fmt.Sprintf("%s (task_id, stdout, stderr, exit_code, duration_ms, created_at) VALUES (?, '', ?, -1, 0, ?)",
		s.dialect.ReplaceInto("task_results"))
	_, err = tx.ExecContext(ctx, query, taskID, stderr, now)
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
	query := fmt.Sprintf("%s (task_id, stdout, stderr, exit_code, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		s.dialect.ReplaceInto("task_results"))
	_, err = tx.ExecContext(ctx, query, result.TaskID, result.Stdout, result.Stderr, result.ExitCode, result.DurationMS, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) TimeoutTasks(ctx context.Context, cutoff string) error {
	// Timeout running tasks (started but not finished)
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, finished_at = ? WHERE status = ? AND started_at < ?`,
		StatusTimeout, nowString(), StatusRunning, cutoff)
	if err != nil {
		return err
	}
	// Timeout stale pending tasks (never picked up, e.g. agent offline)
	_, err = s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, finished_at = ? WHERE status = ? AND created_at < ?`,
		StatusTimeout, nowString(), StatusPending, cutoff)
	return err
}

func (s *Store) PendingTasksForDevice(ctx context.Context, deviceID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, device_id, group_name, command, kind, template_id, executable, args_json, timeout_seconds, status, requested_by, created_at, started_at, finished_at
		FROM tasks WHERE device_id = ? AND status = ? ORDER BY created_at`, deviceID, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) ListTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.kind, t.template_id, t.executable, t.args_json, t.timeout_seconds, t.status, t.requested_by,
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
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.kind, t.template_id, t.executable, t.args_json, t.timeout_seconds, t.status, t.requested_by,
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
	row := s.db.QueryRowContext(ctx, `SELECT t.id, t.device_id, COALESCE(d.name, ''), t.group_name, t.command, t.kind, t.template_id, t.executable, t.args_json, t.timeout_seconds, t.status, t.requested_by,
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs (id, actor, actor_id, actor_role, remote_addr, request_id, action, device_id, task_id, status, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, audit.ID, audit.Actor, audit.ActorID, audit.ActorRole, audit.RemoteAddr, audit.RequestID, audit.Action, audit.DeviceID, audit.TaskID, audit.Status, audit.Message, audit.CreatedAt)
	return err
}

func (s *Store) ListAudit(ctx context.Context, filter AuditFilter) ([]AuditLog, error) {
	where := []string{"1=1"}
	args := []any{}

	if filter.Action != "" {
		where = append(where, "a.action = ?")
		args = append(args, filter.Action)
	}
	if filter.Actor != "" {
		where = append(where, "a.actor = ?")
		args = append(args, filter.Actor)
	}
	if filter.DeviceID != "" {
		where = append(where, "a.device_id = ?")
		args = append(args, filter.DeviceID)
	}
	if filter.Status != "" {
		where = append(where, "a.status = ?")
		args = append(args, filter.Status)
	}
	if filter.From != "" {
		where = append(where, "a.created_at >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		where = append(where, "a.created_at <= ?")
		args = append(args, filter.To)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	query := fmt.Sprintf(`SELECT a.id, a.actor, a.actor_id, a.actor_role, a.remote_addr, a.request_id, a.action, COALESCE(a.device_id, ''), COALESCE(d.name, ''), COALESCE(a.task_id, ''),
		a.status, a.message, a.created_at
		FROM audit_logs a
		LEFT JOIN devices d ON d.id = a.device_id
		WHERE %s
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`, strings.Join(where, " AND "))
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var audits []AuditLog
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.Actor, &a.ActorID, &a.ActorRole, &a.RemoteAddr, &a.RequestID, &a.Action, &a.DeviceID, &a.Device, &a.TaskID, &a.Status, &a.Message, &a.CreatedAt); err != nil {
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
	var argsJSON string
	var startedAt, finishedAt sql.NullString
	var stdout, stderr, resultAt sql.NullString
	var exitCode sql.NullInt64
	var duration sql.NullInt64
	err := row.Scan(&task.ID, &task.DeviceID, &task.DeviceName, &task.GroupName, &task.Command, &task.Kind, &task.TemplateID, &task.Executable, &argsJSON, &task.TimeoutSeconds, &task.Status, &task.RequestedBy,
		&task.CreatedAt, &startedAt, &finishedAt, &stdout, &stderr, &exitCode, &duration, &resultAt)
	if err != nil {
		return Task{}, err
	}
	_ = json.Unmarshal([]byte(argsJSON), &task.Args)
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
		var argsJSON string
		var startedAt, finishedAt sql.NullString
		if err := rows.Scan(&task.ID, &task.DeviceID, &task.GroupName, &task.Command, &task.Kind, &task.TemplateID, &task.Executable, &argsJSON, &task.TimeoutSeconds, &task.Status, &task.RequestedBy, &task.CreatedAt, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(argsJSON), &task.Args)
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
