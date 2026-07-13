package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	sessionCookieName = "labops_session"
	csrfCookieName    = "labops_csrf"
)

type authContextKey struct{}
type requestIDContextKey struct{}

type authContext struct {
	User      User
	SessionID string
	Legacy    bool
}

func currentAuth(ctx context.Context) authContext {
	value, _ := ctx.Value(authContextKey{}).(authContext)
	return value
}

func currentUser(ctx context.Context) User { return currentAuth(ctx).User }

func setAuthCookies(w http.ResponseWriter, sessionToken, csrfToken string, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: sessionToken, Path: "/", MaxAge: 86400, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", MaxAge: 86400, HttpOnly: false, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func clearAuthCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func isStateChanging(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func isCSRFExempt(path string) bool {
	return path == "/api/auth/login" ||
		path == "/api/agent/enroll" ||
		path == "/api/v1/system/bootstrap" ||
		path == "/api/setup/admin"
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func (a *App) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			buf := make([]byte, 12)
			_, _ = rand.Read(buf)
			id = hex.EncodeToString(buf)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id)))
	})
}
