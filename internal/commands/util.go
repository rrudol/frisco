package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/rrudol/frisco/internal/session"
)

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(b))
	return err
}

func loadJSONFile(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// parseJSONOrKV mirrors Python parse_json_or_kv_data (no trailing newline-only edge cases).
func parseJSONOrKV(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return raw, nil
	}
	if len(vals) == 0 {
		return raw, nil
	}
	m := make(map[string]any, len(vals))
	for k, vs := range vals {
		if len(vs) == 1 {
			m[k] = vs[0]
		} else {
			m[k] = vs
		}
	}
	return m, nil
}

func stringField(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		return s, s != ""
	}
}

func refreshTokenString(s *session.Session) string {
	if s == nil || s.RefreshToken == nil {
		return ""
	}
	switch t := s.RefreshToken.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}
