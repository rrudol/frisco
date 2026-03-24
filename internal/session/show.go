package session

import (
	"encoding/json"
	"maps"
	"strings"
)

// RedactedCopy returns a deep-ish copy safe to print (Python cmd_session_show).
func RedactedCopy(s *Session) map[string]any {
	if s == nil {
		return nil
	}
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if t, ok := m["token"]; ok && t != nil {
		m["token"] = "***"
	}
	if t, ok := m["refresh_token"]; ok && t != nil {
		m["refresh_token"] = "***"
	}
	if h, ok := m["headers"].(map[string]any); ok {
		red := maps.Clone(h)
		for k := range red {
			kl := strings.ToLower(k)
			if kl == "authorization" || kl == "cookie" {
				red[k] = "***"
			}
		}
		m["headers"] = red
	}
	return m
}
