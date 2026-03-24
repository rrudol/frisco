// Package session manages the persistent Frisco CLI session stored at
// ~/.frisco-cli/session.json (base URL, auth token, refresh token, user ID, headers).
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultBaseURL = "https://www.frisco.pl"

var (
	sessionDir  string
	sessionFile string
)

func init() {
	home, _ := os.UserHomeDir()
	sessionDir = filepath.Join(home, ".frisco-cli")
	sessionFile = filepath.Join(sessionDir, "session.json")
}

// Dir returns ~/.frisco-cli (after init).
func Dir() string { return sessionDir }

// FilePath returns ~/.frisco-cli/session.json.
func FilePath() string { return sessionFile }

// Session is persisted as ~/.frisco-cli/session.json.
type Session struct {
	BaseURL      string            `json:"base_url"`
	Headers      map[string]string `json:"headers"`
	Token        any               `json:"token"`
	UserID       any               `json:"user_id"`
	RefreshToken any               `json:"refresh_token"`
}

func defaultSession() *Session {
	return &Session{
		BaseURL:   DefaultBaseURL,
		Headers:   map[string]string{},
		Token:  nil,
		UserID: nil,
	}
}

// EnsureDir creates ~/.frisco-cli with 0700 permissions if it does not exist.
func EnsureDir() error {
	return os.MkdirAll(sessionDir, 0o700)
}

// Load reads the session file and returns a Session. Returns a default session
// when the file does not yet exist.
func Load() (*Session, error) {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSession(), nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.BaseURL == "" {
		s.BaseURL = DefaultBaseURL
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers = NormalizeHeaders(s.Headers)
	return &s, nil
}

// Save persists s to the session file with 0600 permissions.
func Save(s *Session) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers = NormalizeHeaders(s.Headers)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionFile, data, 0o600)
}

// UserIDString returns session user_id as string or empty.
func UserIDString(s *Session) string {
	if s == nil || s.UserID == nil {
		return ""
	}
	switch v := s.UserID.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

// RequireUserID returns the explicit user ID if given, otherwise falls back to the session user ID.
func RequireUserID(s *Session, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	uid := UserIDString(s)
	if uid == "" {
		return "", errors.New("Missing user_id. Import session with 'session from-curl' using /users/{id}/... endpoint or pass --user-id.")
	}
	return uid, nil
}

// TokenString returns bearer token as string (JSON may unmarshal as string).
func TokenString(s *Session) string {
	if s == nil || s.Token == nil {
		return ""
	}
	switch v := s.Token.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// RefreshTokenString returns refresh token from session.
func RefreshTokenString(s *Session) string {
	if s == nil || s.RefreshToken == nil {
		return ""
	}
	switch v := s.RefreshToken.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// NormalizeHeaders deduplicates headers case-insensitively and uses canonical keys.
func NormalizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	type candidate struct {
		value string
		rank  int
	}
	best := map[string]candidate{}
	orig := map[string]string{}

	for k, v := range headers {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			continue
		}
		canon := canonicalHeaderKey(lk, k)
		rank := 0
		if k == canon {
			rank = 2
		} else if strings.EqualFold(k, canon) {
			rank = 1
		}
		if prev, ok := best[lk]; !ok || rank > prev.rank || (rank == prev.rank && len(v) > len(prev.value)) {
			best[lk] = candidate{value: v, rank: rank}
			orig[lk] = canon
		}
	}

	out := make(map[string]string, len(best))
	for lk, cand := range best {
		out[orig[lk]] = cand.value
	}
	return out
}

// canonicalHeaderKey returns the canonical form of a known header name, or
// original when the key is not in the recognised set.
func canonicalHeaderKey(lowerKey, original string) string {
	switch lowerKey {
	case "authorization":
		return "Authorization"
	case "cookie":
		return "Cookie"
	case "content-type":
		return "Content-Type"
	case "accept":
		return "Accept"
	case "origin":
		return "Origin"
	case "referer":
		return "Referer"
	case "x-api-version":
		return "X-Api-Version"
	case "x-requested-with":
		return "X-Requested-With"
	default:
		return original
	}
}
