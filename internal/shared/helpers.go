// Package shared provides common helper functions used across commands, mcpserver, and tui packages.
package shared

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractNutritionBlock walks a JSON payload recursively and returns the first
// value whose key contains "nutrition", "nutri", or "odzyw" (case-insensitive).
func ExtractNutritionBlock(payload any) any {
	var walk func(v any) any
	walk = func(v any) any {
		switch t := v.(type) {
		case map[string]any:
			for k, value := range t {
				lk := strings.ToLower(k)
				if strings.Contains(lk, "nutrition") || strings.Contains(lk, "nutri") || strings.Contains(lk, "odzyw") {
					return value
				}
			}
			for _, value := range t {
				if found := walk(value); found != nil {
					return found
				}
			}
		case []any:
			for _, item := range t {
				if found := walk(item); found != nil {
					return found
				}
			}
		case json.RawMessage:
			var inner any
			if err := json.Unmarshal(t, &inner); err == nil {
				return walk(inner)
			}
		}
		return nil
	}
	return walk(payload)
}

// ProductNameFromMap extracts a human-readable product name from a map,
// trying common Frisco API key names in priority order.
func ProductNameFromMap(m map[string]any) string {
	for _, k := range []string{"name", "displayName", "productName", "title", "productTitle"} {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		if s := LocalizedString(v); s != "" {
			return s
		}
	}
	return ""
}

// LocalizedString extracts a string from a value that may be a plain string,
// a locale map (e.g. {"pl": "...", "en": "..."}), or another nested structure.
// It prefers "pl", then "en".
func LocalizedString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case map[string]any:
		for _, k := range []string{"pl", "en"} {
			if s := mapLocaleValue(x, k); s != "" {
				return s
			}
		}
		for _, raw := range x {
			if s := LocalizedString(raw); s != "" {
				return s
			}
		}
		return ""
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if strings.HasPrefix(s, "map[") {
			return ""
		}
		return s
	}
}

func mapLocaleValue(m map[string]any, key string) string {
	if key == "" {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// MoneyString converts a value (number, string, or nested price map) to a
// formatted money string like "12.34". Returns "" when no value can be extracted.
func MoneyString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case int:
		return fmt.Sprintf("%.2f", float64(x))
	case int32:
		return fmt.Sprintf("%.2f", float64(x))
	case int64:
		return fmt.Sprintf("%.2f", float64(x))
	case float32:
		return fmt.Sprintf("%.2f", float64(x))
	case float64:
		return fmt.Sprintf("%.2f", x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return ""
		}
		return s
	case map[string]any:
		for _, k := range []string{"price", "gross", "amount", "value", "_total", "FRS"} {
			if s := MoneyString(x[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

// FormatMoneyValue formats a single numeric or string value as a money string.
// Unlike MoneyString, this does not recurse into maps with domain-specific keys
// like "_total" or "FRS"; it only handles the leaf value types and a generic
// map with "price"/"gross"/"amount"/"value" keys.
func FormatMoneyValue(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case float64:
		return fmt.Sprintf("%.2f", x)
	case float32:
		return fmt.Sprintf("%.2f", float64(x))
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case string:
		return strings.TrimSpace(x)
	case map[string]any:
		if s := FormatMoneyValue(x["price"]); s != "" {
			return s
		}
		if s := FormatMoneyValue(x["gross"]); s != "" {
			return s
		}
		if s := FormatMoneyValue(x["amount"]); s != "" {
			return s
		}
		if s := FormatMoneyValue(x["value"]); s != "" {
			return s
		}
	}
	return ""
}

// TruncateText truncates a string to max runes, appending "..." if shortened.
func TruncateText(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

// StringFieldFromMap looks up the first non-empty string value from a map
// trying each key in order.
func StringFieldFromMap(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				if x != "" {
					return x
				}
			default:
				s := strings.TrimSpace(fmt.Sprint(x))
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}
