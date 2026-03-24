package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rrudol/frisco/internal/i18n"
	"github.com/rrudol/frisco/internal/session"
)

var outputFormat = "table"

func printJSON(v any) error {
	if strings.EqualFold(outputFormat, "json") {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Println(string(b))
		return err
	}
	return printPretty(v)
}

func printPretty(v any) error {
	switch t := v.(type) {
	case map[string]any:
		return printPrettyMap(t)
	case []any:
		return printPrettyList(t, "")
	default:
		_, err := fmt.Println(fmt.Sprint(v))
		return err
	}
}

func printPrettyMap(m map[string]any) error {
	// Special-case the most common list payload shapes.
	for _, key := range []string{"orders", "items", "products", "slots"} {
		if raw, ok := m[key]; ok {
			if list, ok := raw.([]any); ok && len(list) > 0 {
				if key == "slots" {
					return printPrettySlots(map[string]any{"days": []any{m}})
				}
				printScalarMap(m, []string{key})
				return printPrettyList(list, key)
			}
		}
	}
	if rawDays, ok := m["days"]; ok {
		if days, ok := rawDays.([]any); ok {
			return printPrettySlots(map[string]any{"days": days})
		}
	}
	return printScalarMap(m, nil)
}

func printScalarMap(m map[string]any, skip []string) error {
	skipSet := map[string]struct{}{}
	for _, k := range skip {
		skipSet[k] = struct{}{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if _, exists := skipSet[k]; !exists {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, k := range keys {
		val := m[k]
		if isScalar(val) {
			_, _ = fmt.Fprintf(w, "%s\t%v\n", k, val)
		}
	}
	_ = w.Flush()
	for _, k := range keys {
		val := m[k]
		if !isScalar(val) {
			_, _ = fmt.Printf("\n%s:\n", k)
			switch nested := val.(type) {
			case map[string]any:
				_ = printScalarMap(nested, nil)
			case []any:
				_ = printPrettyList(nested, k)
			default:
				_, _ = fmt.Println(fmt.Sprint(nested))
			}
		}
	}
	return nil
}

func printPrettyList(list []any, name string) error {
	rows := listOfMaps(list)
	if len(rows) > 0 {
		cols := inferColumns(rows)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, strings.Join(cols, "\t"))
		for _, row := range rows {
			cells := make([]string, 0, len(cols))
			for _, col := range cols {
				cells = append(cells, cellValue(row[col]))
			}
			_, _ = fmt.Fprintln(w, strings.Join(cells, "\t"))
		}
		_ = w.Flush()
		return nil
	}

	for i, item := range list {
		if isScalar(item) {
			_, _ = fmt.Printf("- %v\n", item)
			continue
		}
		_, _ = fmt.Printf("[%d]\n", i+1)
		switch t := item.(type) {
		case map[string]any:
			_ = printScalarMap(t, nil)
		default:
			_, _ = fmt.Println(fmt.Sprint(t))
		}
	}
	return nil
}

func printPrettySlots(payload map[string]any) error {
	raw, ok := payload["days"].([]any)
	if !ok {
		return printScalarMap(payload, nil)
	}
	for _, d := range raw {
		day, ok := d.(map[string]any)
		if !ok {
			continue
		}
		date := cellValue(day["date"])
		_, _ = fmt.Printf("%s\n", date)
		slots, _ := day["slots"].([]any)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "from\tto\tmethod\twarehouse")
		for _, s := range slots {
			slot, ok := s.(map[string]any)
			if !ok {
				continue
			}
			from := hhmm(slot["startsAt"])
			to := hhmm(slot["endsAt"])
			_, _ = fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\n",
				from,
				to,
				cellValue(slot["deliveryMethod"]),
				cellValue(slot["warehouse"]),
			)
		}
		_ = w.Flush()
		_, _ = fmt.Println()
	}
	return nil
}

func listOfMaps(list []any) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func inferColumns(rows []map[string]any) []string {
	if len(rows) == 0 {
		return []string{"value"}
	}
	seen := map[string]struct{}{}
	cols := make([]string, 0, 12)
	priority := []string{"id", "status", "createdAt", "name", "productId", "quantity", "totalPLN", "startsAt", "endsAt"}
	for _, p := range priority {
		for _, row := range rows {
			if _, ok := row[p]; ok {
				if _, exists := seen[p]; !exists {
					cols = append(cols, p)
					seen[p] = struct{}{}
				}
				break
			}
		}
	}
	for _, row := range rows {
		for k, v := range row {
			if !isScalar(v) {
				continue
			}
			if _, exists := seen[k]; !exists {
				seen[k] = struct{}{}
				cols = append(cols, k)
			}
		}
	}
	if len(cols) == 0 {
		return []string{"value"}
	}
	return cols
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, int, int32, int64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func cellValue(v any) string {
	if v == nil {
		return "—"
	}
	if s, ok := v.(string); ok {
		if strings.TrimSpace(s) == "" {
			return "—"
		}
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil && len(b) < 80 {
		return string(b)
	}
	return fmt.Sprint(v)
}

func hhmm(v any) string {
	s := cellValue(v)
	if s == "—" {
		return s
	}
	parts := strings.SplitN(s, "T", 2)
	if len(parts) != 2 {
		return s
	}
	if len(parts[1]) >= 5 {
		return parts[1][:5]
	}
	return parts[1]
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

func tr(en, pl string) string {
	return i18n.T(en, pl)
}
