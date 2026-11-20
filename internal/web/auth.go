package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "mikromon_session"

func (s *Server) ensureAuthSeed() {
	if s.store.GetSetting("auth_secret") == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err == nil {
			_ = s.store.SetSetting("auth_secret", hex.EncodeToString(b))
		}
	}
	envPass := os.Getenv("MIKROMON_ADMIN_PASS")
	if envPass != "" && s.store.GetSetting("auth_pass_hash") == "" {
		user := os.Getenv("MIKROMON_ADMIN_USER")
		if user == "" {
			user = "admin"
		}
		_ = s.saveAdminCredentials(user, envPass)
	}
}

func hashPassword(saltHex, pass string) string {
	h := sha256.Sum256([]byte(saltHex + ":" + pass))
	for i := 0; i < 10000; i++ {
		h = sha256.Sum256(h[:])
	}
	return saltHex + "$" + hex.EncodeToString(h[:])
}

func verifyPassword(stored, pass string) bool {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 || parts[0] == "" {
		return false
	}
	expected := hashPassword(parts[0], pass)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(stored)) == 1
}

func (s *Server) saveAdminCredentials(user, pass string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		user = "admin"
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	h := hashPassword(hex.EncodeToString(salt), pass)
	if err := s.store.SetSetting("auth_user", user); err != nil {
		return err
	}
	return s.store.SetSetting("auth_pass_hash", h)
}

func (s *Server) authSecretBytes() []byte {
	sec := s.store.GetSetting("auth_secret")
	if sec == "" {
		sec = "mikromon-fallback-secret"
	}
	return []byte(sec)
}

func (s *Server) signSessionToken(user string, exp int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%d", user, exp)))
	mac := hmac.New(sha256.New, s.authSecretBytes())
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func (s *Server) verifySessionToken(tok string) (string, bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	mac := hmac.New(sha256.New, s.authSecretBytes())
	mac.Write([]byte(parts[0]))
	expectedSig := mac.Sum(nil)
	gotSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(gotSig, expectedSig) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	fields := strings.SplitN(string(raw), "|", 2)
	if len(fields) != 2 {
		return "", false
	}
	exp, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	return fields[0], true
}

func (s *Server) currentAuthUser(r *http.Request) (string, bool) {
	if os.Getenv("MIKROMON_AUTH_DISABLED") == "true" {
		return "admin", true
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return s.verifySessionToken(c.Value)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, user string, remember bool) {
	ttl := 24 * time.Hour
	if remember {
		ttl = 7 * 24 * time.Hour
	}
	exp := time.Now().Add(ttl).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSessionToken(user, exp),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/api/auth/status" ||
			r.URL.Path == "/api/auth/login" ||
			r.URL.Path == "/api/auth/setup" ||
			r.URL.Path == "/api/auth/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.currentAuthUser(r); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	disabled := os.Getenv("MIKROMON_AUTH_DISABLED") == "true"
	needsSetup := !disabled && s.store.GetSetting("auth_pass_hash") == ""
	user, authed := s.currentAuthUser(r)
	if needsSetup {
		authed = false
	}
	storedUser := s.store.GetSetting("auth_user")
	if storedUser == "" {
		storedUser = "admin"
	}
	writeJSON(w, 200, map[string]any{
		"authenticated": authed,
		"needs_setup":   needsSetup,
		"auth_disabled": disabled,
		"username":      user,
		"admin_user":    storedUser,
	})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if s.store.GetSetting("auth_pass_hash") != "" {
		writeJSON(w, 403, map[string]string{"error": "already initialized"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if len(req.Password) < 4 {
		writeJSON(w, 400, map[string]string{"error": "password must be at least 4 characters"})
		return
	}
	user := strings.TrimSpace(req.Username)
	if user == "" {
		user = "admin"
	}
	if err := s.saveAdminCredentials(user, req.Password); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w, user, true)
	writeJSON(w, 200, map[string]any{"ok": true, "username": user})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	storedUser := s.store.GetSetting("auth_user")
	if storedUser == "" {
		storedUser = "admin"
	}
	storedHash := s.store.GetSetting("auth_pass_hash")
	if storedHash == "" {
		writeJSON(w, 400, map[string]string{"error": "setup required"})
		return
	}
	if strings.TrimSpace(req.Username) != storedUser || !verifyPassword(storedHash, req.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid username or password"})
		return
	}
	s.setSessionCookie(w, storedUser, req.Remember)
	writeJSON(w, 200, map[string]any{"ok": true, "username": storedUser})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleAuthPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewUsername string `json:"new_username"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	storedHash := s.store.GetSetting("auth_pass_hash")
	if storedHash != "" && !verifyPassword(storedHash, req.OldPassword) {
		writeJSON(w, 400, map[string]string{"error": "current password incorrect"})
		return
	}
	if len(req.NewPassword) < 4 {
		writeJSON(w, 400, map[string]string{"error": "new password must be at least 4 characters"})
		return
	}
	user := strings.TrimSpace(req.NewUsername)
	if user == "" {
		user = s.store.GetSetting("auth_user")
		if user == "" {
			user = "admin"
		}
	}
	if err := s.saveAdminCredentials(user, req.NewPassword); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w, user, true)
	writeJSON(w, 200, map[string]any{"ok": true, "username": user})
}
