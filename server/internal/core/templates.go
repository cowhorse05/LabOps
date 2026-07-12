package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func scanTemplate(scanner interface{ Scan(...any) error }) (CommandTemplate, error) {
	var item CommandTemplate
	var argsJSON, paramsJSON string
	var privileged, enabled int
	err := scanner.Scan(&item.ID, &item.Name, &item.Description, &item.OS, &item.Executable, &argsJSON, &paramsJSON,
		&privileged, &enabled, &item.TimeoutSeconds, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(argsJSON), &item.Args); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(paramsJSON), &item.Parameters); err != nil {
		return item, err
	}
	item.RequiresPrivilege = privileged == 1
	item.Enabled = enabled == 1
	return item, nil
}

const templateSelect = `SELECT id, name, description, os, executable, args_json, parameters_json,
	requires_privilege, enabled, timeout_seconds, created_at, updated_at FROM command_templates`

func (s *Store) ListCommandTemplates(ctx context.Context) ([]CommandTemplate, error) {
	rows, err := s.db.QueryContext(ctx, templateSelect+" ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CommandTemplate{}
	for rows.Next() {
		item, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetCommandTemplate(ctx context.Context, id string) (CommandTemplate, bool, error) {
	item, err := scanTemplate(s.db.QueryRowContext(ctx, templateSelect+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return CommandTemplate{}, false, nil
	}
	return item, err == nil, err
}

func validateTemplate(item CommandTemplate) error {
	item.Executable = strings.TrimSpace(item.Executable)
	if item.Name == "" || item.Executable == "" || !strings.HasPrefix(item.Executable, "/") {
		return fmt.Errorf("name and absolute executable path are required")
	}
	if item.TimeoutSeconds < 1 || item.TimeoutSeconds > 300 {
		return fmt.Errorf("timeoutSeconds must be between 1 and 300")
	}
	for _, parameter := range item.Parameters {
		if parameter.Name == "" {
			return fmt.Errorf("parameter name is required")
		}
		if parameter.Pattern != "" {
			if _, err := regexp.Compile(parameter.Pattern); err != nil {
				return fmt.Errorf("invalid pattern for %s", parameter.Name)
			}
		}
	}
	return nil
}

func (s *Store) SaveCommandTemplate(ctx context.Context, item CommandTemplate) (CommandTemplate, error) {
	if err := validateTemplate(item); err != nil {
		return CommandTemplate{}, err
	}
	if item.ID == "" {
		item.ID = newID("tpl")
	}
	argsJSON, _ := json.Marshal(item.Args)
	paramsJSON, _ := json.Marshal(item.Parameters)
	now := nowString()
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	cols := []string{"name", "description", "os", "executable", "args_json", "parameters_json", "requires_privilege", "enabled", "timeout_seconds", "created_at", "updated_at"}
	query := fmt.Sprintf("INSERT INTO command_templates (id, %s) VALUES (%s) %s", strings.Join(cols, ", "), placeholders(len(cols)+1), s.dialect.UpsertSuffix("id", cols))
	privileged, enabled := 0, 0
	if item.RequiresPrivilege {
		privileged = 1
	}
	if item.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, query, item.ID, item.Name, item.Description, item.OS, item.Executable, string(argsJSON), string(paramsJSON), privileged, enabled, item.TimeoutSeconds, item.CreatedAt, item.UpdatedAt)
	return item, err
}

func RenderTemplate(item CommandTemplate, values map[string]any) ([]string, error) {
	if !item.Enabled {
		return nil, fmt.Errorf("template is disabled")
	}
	if item.RequiresPrivilege {
		return nil, fmt.Errorf("privileged templates are not supported by low-privilege agents")
	}
	resolved := map[string]string{}
	for _, parameter := range item.Parameters {
		raw, exists := values[parameter.Name]
		if !exists {
			return nil, fmt.Errorf("parameter %s is required", parameter.Name)
		}
		value := fmt.Sprint(raw)
		if len(parameter.Enum) > 0 {
			allowed := false
			for _, candidate := range parameter.Enum {
				if value == candidate {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("parameter %s is not allowed", parameter.Name)
			}
		}
		if parameter.Pattern != "" {
			matched, _ := regexp.MatchString("^(?:"+parameter.Pattern+")$", value)
			if !matched {
				return nil, fmt.Errorf("parameter %s has invalid format", parameter.Name)
			}
		}
		if parameter.Type == "integer" {
			number, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("parameter %s must be an integer", parameter.Name)
			}
			if parameter.Min != nil && number < *parameter.Min {
				return nil, fmt.Errorf("parameter %s is below minimum", parameter.Name)
			}
			if parameter.Max != nil && number > *parameter.Max {
				return nil, fmt.Errorf("parameter %s is above maximum", parameter.Name)
			}
		}
		resolved[parameter.Name] = value
	}
	args := make([]string, len(item.Args))
	for i, arg := range item.Args {
		args[i] = arg
		for name, value := range resolved {
			args[i] = strings.ReplaceAll(args[i], "{{"+name+"}}", value)
		}
		if strings.Contains(args[i], "{{") {
			return nil, fmt.Errorf("unresolved template argument")
		}
	}
	return args, nil
}

func (a *App) handleListCommandTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListCommandTemplates(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "TEMPLATE_LIST_FAILED", "unable to list templates")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleCreateCommandTemplate(w http.ResponseWriter, r *http.Request) {
	var item CommandTemplate
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = ""
	created, err := a.store.SaveCommandTemplate(r.Context(), item)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *App) handleUpdateCommandTemplate(w http.ResponseWriter, r *http.Request) {
	var item CommandTemplate
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = r.PathValue("id")
	updated, err := a.store.SaveCommandTemplate(r.Context(), item)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
