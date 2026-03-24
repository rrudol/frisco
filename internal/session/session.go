package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rrudol/frisco/internal/i18n"
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

// DefaultHARPath matches Python DEFAULT_HAR_PATH pattern (~ /Downloads/www.frisco.pl.har).
func DefaultHARPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads", "www.frisco.pl.har")
}

// Endpoint matches entries stored from HAR import (Python session["endpoints"]).
type Endpoint struct {
	Method       string `json:"method"`
	Host         string `json:"host"`
	Path         string `json:"path"`
	PathTemplate string `json:"path_template"`
	HasQuery     bool   `json:"has_query"`
	URL          string `json:"url"`
}

// Session is persisted as ~/.frisco-cli/session.json.
type Session struct {
	BaseURL      string            `json:"base_url"`
	Headers      map[string]string `json:"headers"`
	Token        any               `json:"token"`
	UserID       any               `json:"user_id"`
	RefreshToken any               `json:"refresh_token"`
	Endpoints    []Endpoint        `json:"endpoints"`
	HarPath      any               `json:"har_path"`
}

func defaultSession() *Session {
	return &Session{
		BaseURL:   DefaultBaseURL,
		Headers:   map[string]string{},
		Token:     nil,
		UserID:    nil,
		Endpoints: nil,
		HarPath:   nil,
	}
}

func EnsureDir() error {
	return os.MkdirAll(sessionDir, 0o700)
}

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

// RequireUserID returns explicit or session user id or errors (Polish message like Python).
func RequireUserID(s *Session, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	uid := UserIDString(s)
	if uid == "" {
		return "", errors.New(i18n.T(
			"Missing user_id. Import session with 'session from-curl' using /users/{id}/... endpoint or pass --user-id.",
			"Brak user_id. Wklej curl z endpointem /users/{id}/... przez 'session from-curl' albo podaj --user-id.",
		))
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
