package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	PermissionRead           = "system:read"
	PermissionUserManage     = "users:manage"
	PermissionEnrollManage   = "enrollment:manage"
	PermissionDeviceRevoke   = "devices:revoke"
	PermissionTemplateManage = "templates:manage"
	PermissionTemplateRun    = "templates:execute"
	PermissionAdHocRun       = "commands:adhoc"
	PermissionLLMManage      = "llm:manage"
)

var permissionsByRole = map[string][]string{
	RoleAdmin:    {PermissionRead, PermissionUserManage, PermissionEnrollManage, PermissionDeviceRevoke, PermissionTemplateManage, PermissionTemplateRun, PermissionAdHocRun, PermissionLLMManage},
	RoleOperator: {PermissionRead, PermissionTemplateRun},
	RoleViewer:   {PermissionRead},
}

func applyUserAuthorization(user *User, roles string) {
	user.Roles = splitRoles(roles)
	user.Role = RoleViewer
	for _, role := range user.Roles {
		if role == RoleAdmin {
			user.Role = RoleAdmin
			break
		}
		if role == RoleOperator {
			user.Role = RoleOperator
		}
	}
	user.Roles = []string{user.Role}
	user.Permissions = append([]string(nil), permissionsByRole[user.Role]...)
	if user.Status == "" {
		user.Status = "active"
	}
}

func HasPermission(user User, permission string) bool {
	for _, candidate := range user.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func validRole(role string) bool {
	return role == RoleAdmin || role == RoleOperator || role == RoleViewer
}

func randomToken(byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Store) CreateWebSession(ctx context.Context, userID, remoteAddr, userAgent string) (string, string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	id := newID("session")
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO web_sessions
		(id, user_id, token_hash, csrf_hash, remote_addr, user_agent, created_at, last_seen_at, idle_expires_at, absolute_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, tokenHash(token), tokenHash(csrf), remoteAddr, userAgent,
		now.Format(time.RFC3339), now.Format(time.RFC3339), now.Add(8*time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339))
	if err != nil {
		return "", "", fmt.Errorf("create web session: %w", err)
	}
	return token, csrf, nil
}

func (s *Store) AuthenticateWebSession(ctx context.Context, rawToken string) (User, string, string, bool, error) {
	var user User
	var roles, sessionID, csrfHash, idleExpiry, absoluteExpiry string
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.roles, u.status,
		s.id, s.csrf_hash, s.idle_expires_at, s.absolute_expires_at
		FROM web_sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, tokenHash(rawToken)).
		Scan(&user.ID, &user.Username, &user.DisplayName, &roles, &user.Status,
			&sessionID, &csrfHash, &idleExpiry, &absoluteExpiry)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", "", false, nil
	}
	if err != nil {
		return User{}, "", "", false, err
	}
	now := time.Now().UTC()
	idle, idleErr := time.Parse(time.RFC3339, idleExpiry)
	absolute, absoluteErr := time.Parse(time.RFC3339, absoluteExpiry)
	if user.Status != "active" || idleErr != nil || absoluteErr != nil || now.After(idle) || now.After(absolute) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM web_sessions WHERE id = ?", sessionID)
		return User{}, "", "", false, nil
	}
	applyUserAuthorization(&user, roles)
	_, err = s.db.ExecContext(ctx, "UPDATE web_sessions SET last_seen_at = ?, idle_expires_at = ? WHERE id = ?",
		now.Format(time.RFC3339), now.Add(8*time.Hour).Format(time.RFC3339), sessionID)
	if err != nil {
		return User{}, "", "", false, err
	}
	return user, sessionID, csrfHash, true, nil
}

func (s *Store) DeleteWebSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM web_sessions WHERE id = ?", sessionID)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM web_sessions WHERE user_id = ?", userID)
	return err
}

func (s *Store) PruneExpiredWebSessions(ctx context.Context) error {
	now := nowString()
	_, err := s.db.ExecContext(ctx, "DELETE FROM web_sessions WHERE idle_expires_at < ? OR absolute_expires_at < ?", now, now)
	return err
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, username, display_name, roles, status FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		var roles string
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &roles, &user.Status); err != nil {
			return nil, err
		}
		applyUserAuthorization(&user, roles)
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, username, displayName, password, role string) (User, error) {
	if !validRole(role) {
		return User{}, fmt.Errorf("invalid role")
	}
	if len(password) < 12 {
		return User{}, fmt.Errorf("password must be at least 12 characters")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return User{}, err
	}
	id := newID("user")
	now := nowString()
	_, err = s.db.ExecContext(ctx, `INSERT INTO users
		(id, username, display_name, password, roles, must_change_password, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, 'active', ?, ?)`, id, username, displayName, string(hashed), role, now, now)
	if err != nil {
		return User{}, err
	}
	user := User{ID: id, Username: username, DisplayName: displayName, Status: "active"}
	applyUserAuthorization(&user, role)
	return user, nil
}

func (s *Store) UpdateUserAccess(ctx context.Context, id, role, status string) error {
	if !validRole(role) || (status != "active" && status != "disabled") {
		return fmt.Errorf("invalid role or status")
	}
	result, err := s.db.ExecContext(ctx, "UPDATE users SET roles = ?, status = ?, updated_at = ? WHERE id = ?", role, status, nowString(), id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	if status != "active" {
		return s.DeleteUserSessions(ctx, id)
	}
	return nil
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
