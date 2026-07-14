package core

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Store) CreateEnrollmentCode(ctx context.Context, createdBy string, ttl time.Duration, maxUses int) (EnrollmentCode, error) {
	if ttl <= 0 || ttl > time.Hour || maxUses < 1 || maxUses > 20 {
		return EnrollmentCode{}, fmt.Errorf("ttl must be at most 1 hour and maxUses between 1 and 20")
	}
	raw, err := randomToken(24)
	if err != nil {
		return EnrollmentCode{}, err
	}
	id := newID("enroll")
	now := time.Now().UTC()
	code := EnrollmentCode{ID: id, Code: raw, ExpiresAt: now.Add(ttl).Format(time.RFC3339), MaxUses: maxUses, CreatedBy: createdBy, CreatedAt: now.Format(time.RFC3339)}
	_, err = s.db.ExecContext(ctx, `INSERT INTO enrollment_codes
		(id, code_hash, expires_at, max_uses, used_count, created_by, created_at, revoked_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, '')`, code.ID, tokenHash(raw), code.ExpiresAt, maxUses, createdBy, code.CreatedAt)
	return code, err
}

func (s *Store) ListEnrollmentCodes(ctx context.Context) ([]EnrollmentCode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, expires_at, max_uses, used_count, created_by, created_at, revoked_at
		FROM enrollment_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EnrollmentCode{}
	for rows.Next() {
		var item EnrollmentCode
		if err := rows.Scan(&item.ID, &item.ExpiresAt, &item.MaxUses, &item.UsedCount, &item.CreatedBy, &item.CreatedAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RevokeEnrollmentCode(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE enrollment_codes SET revoked_at = ? WHERE id = ? AND revoked_at = ''", nowString(), id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) EnrollDevice(ctx context.Context, rawCode string, reg RegisterPayload) (Device, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, "", err
	}
	defer tx.Rollback()
	var codeID, expiresAt, revokedAt string
	var maxUses, usedCount int
	err = tx.QueryRowContext(ctx, `SELECT id, expires_at, max_uses, used_count, revoked_at
		FROM enrollment_codes WHERE code_hash = ?`, tokenHash(strings.TrimSpace(rawCode))).
		Scan(&codeID, &expiresAt, &maxUses, &usedCount, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, "", fmt.Errorf("invalid enrollment code")
	}
	if err != nil {
		return Device{}, "", err
	}
	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || revokedAt != "" || time.Now().UTC().After(expiry) || usedCount >= maxUses {
		return Device{}, "", fmt.Errorf("enrollment code expired, revoked, or already used")
	}
	result, err := tx.ExecContext(ctx, `UPDATE enrollment_codes SET used_count = used_count + 1
		WHERE id = ? AND used_count = ? AND revoked_at = ''`, codeID, usedCount)
	if err != nil {
		return Device{}, "", err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return Device{}, "", fmt.Errorf("enrollment code was used concurrently")
	}
	secret, err := randomToken(32)
	if err != nil {
		return Device{}, "", err
	}
	reg.AgentID = newID("device")
	device := deviceFromRegister(reg)
	device.CredentialStatus = "active"
	device.RevokedAt = ""
	devCols := []string{"name", "group_name", "profile", "version", "hostname", "os", "ip", "cpu_cores", "memory_mb", "disk_total_gb", "cpu_usage", "memory_usage", "disk_usage", "status", "last_seen", "created_at", "updated_at", "credential_status", "revoked_at"}
	allCols := append([]string{"id"}, devCols...)
	query := fmt.Sprintf("INSERT INTO devices (%s) VALUES (%s) %s", strings.Join(allCols, ", "), placeholders(len(allCols)), s.dialect.UpsertSuffix("id", devCols))
	_, err = tx.ExecContext(ctx, query, device.ID, device.Name, device.GroupName, device.Profile, device.Version, device.Hostname, device.OS, device.IP,
		device.CPUCores, device.MemoryMB, device.DiskTotalGB, device.CPUUsage, device.MemoryUsage, device.DiskUsage,
		StatusOffline, device.LastSeen, device.CreatedAt, device.UpdatedAt, "active", "")
	if err != nil {
		return Device{}, "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO device_credentials
		(device_id, secret_hash, status, created_at, last_used_at, revoked_at) VALUES (?, ?, 'active', ?, '', '')`,
		device.ID, tokenHash(secret), nowString())
	if err != nil {
		return Device{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return Device{}, "", err
	}
	return device, secret, nil
}

func (s *Store) ValidateDeviceCredential(ctx context.Context, deviceID, secret string) (bool, error) {
	var storedHash, status string
	err := s.db.QueryRowContext(ctx, "SELECT secret_hash, status FROM device_credentials WHERE device_id = ?", deviceID).Scan(&storedHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	valid := status == "active" && subtle.ConstantTimeCompare([]byte(tokenHash(secret)), []byte(storedHash)) == 1
	if valid {
		_, _ = s.db.ExecContext(ctx, "UPDATE device_credentials SET last_used_at = ? WHERE device_id = ?", nowString(), deviceID)
	}
	return valid, nil
}

func (s *Store) RevokeDeviceCredential(ctx context.Context, deviceID string) error {
	now := nowString()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE device_credentials SET status = 'revoked', revoked_at = ? WHERE device_id = ? AND status = 'active'", now, deviceID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, "UPDATE devices SET credential_status = 'revoked', revoked_at = ?, status = 'offline', updated_at = ? WHERE id = ?", now, now, deviceID); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) handleListEnrollmentCodes(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListEnrollmentCodes(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ENROLLMENT_LIST_FAILED", "unable to list enrollment codes")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleCreateEnrollmentCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpiresInSeconds int `json:"expiresInSeconds"`
		MaxUses          int `json:"maxUses"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.ExpiresInSeconds == 0 {
		req.ExpiresInSeconds = 600
	}
	if req.MaxUses == 0 {
		req.MaxUses = 1
	}
	item, err := a.store.CreateEnrollmentCode(r.Context(), currentUser(r.Context()).ID, time.Duration(req.ExpiresInSeconds)*time.Second, req.MaxUses)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "ENROLLMENT_CREATE_FAILED", err.Error())
		return
	}
	a.audit(r, currentUser(r.Context()), "enrollment.create", "success", fmt.Sprintf("created enrollment code %s", item.ID))
	writeJSON(w, http.StatusCreated, item)
}

func (a *App) handleRevokeEnrollmentCode(w http.ResponseWriter, r *http.Request) {
	if err := a.store.RevokeEnrollmentCode(r.Context(), r.PathValue("id")); err != nil {
		writeAPIError(w, http.StatusNotFound, "ENROLLMENT_NOT_FOUND", "enrollment code not found")
		return
	}
	a.audit(r, currentUser(r.Context()), "enrollment.revoke", "success", "revoked enrollment code "+r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string          `json:"code"`
		Device RegisterPayload `json:"device"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Device.Name) == "" {
		writeAPIError(w, http.StatusBadRequest, "ENROLLMENT_INVALID", "code and device name are required")
		return
	}
	device, secret, err := a.store.EnrollDevice(r.Context(), req.Code, req.Device)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "ENROLLMENT_REJECTED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"deviceId": device.ID, "deviceSecret": secret, "device": device})
}

func (a *App) handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.RevokeDeviceCredential(r.Context(), id); err != nil {
		writeAPIError(w, http.StatusNotFound, "DEVICE_CREDENTIAL_NOT_FOUND", "active device credential not found")
		return
	}
	a.mu.RLock()
	client := a.clients[id]
	a.mu.RUnlock()
	if client != nil {
		client.Close()
	}
	user := currentUser(r.Context())
	_ = a.store.CreateAudit(r.Context(), AuditLog{Actor: user.Username, ActorID: user.ID, ActorRole: user.Role, RemoteAddr: clientIP(r), RequestID: requestID(r.Context()), Action: "device.revoke", DeviceID: id, Status: StatusSuccess, Message: "device credential revoked"})
	w.WriteHeader(http.StatusNoContent)
}
