package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

// cartLine is one row from GET /cart (parsed defensively for varying API shapes).
type cartLine struct {
	productID string
	quantity  int
	name      string
	unitPrice string
}

// cartDataMsg carries the result of a GET cart (initial load or after PUT refresh).
type cartDataMsg struct {
	lines []cartLine
	err   error
}

// RunCart starts the interactive cart TUI (Bubble Tea: model / update / view).
func RunCart(s *session.Session, uid string) error {
	p := tea.NewProgram(initialModel(s, uid), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type cartModel struct {
	sess    *session.Session
	uid     string
	items   []cartLine
	cursor  int
	busy    bool
	errText string
}

func initialModel(s *session.Session, uid string) cartModel {
	return cartModel{
		sess:   s,
		uid:    uid,
		items:  nil,
		cursor: 0,
	}
}

func (m cartModel) Init() tea.Cmd {
	return m.loadCartCmd()
}

func (m cartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if len(m.items) > 0 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if len(m.items) > 0 && m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		case "+", "=":
			if len(m.items) == 0 {
				return m, nil
			}
			line := m.items[m.cursor]
			m.busy = true
			m.errText = ""
			return m, m.putQuantityCmd(line.productID, line.quantity+1)
		case "-":
			if len(m.items) == 0 {
				return m, nil
			}
			line := m.items[m.cursor]
			nq := line.quantity - 1
			if nq < 0 {
				nq = 0
			}
			m.busy = true
			m.errText = ""
			return m, m.putQuantityCmd(line.productID, nq)
		case "d":
			if len(m.items) == 0 {
				return m, nil
			}
			line := m.items[m.cursor]
			m.busy = true
			m.errText = ""
			return m, m.putQuantityCmd(line.productID, 0)
		case "r":
			m.busy = true
			m.errText = ""
			return m, m.loadCartCmd()
		}
		return m, nil

	case cartDataMsg:
		m.busy = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, nil
		}
		m.items = msg.lines
		m.errText = ""
		m.cursor = clampCursor(m.cursor, len(m.items))
		return m, nil
	}

	return m, nil
}

func (m cartModel) View() string {
	var b strings.Builder
	b.WriteString("Koszyk — ↑↓ wybór  +/− ilość  d usuń  r odśwież  q wyjście\n")
	if m.busy {
		b.WriteString("\nŁadowanie…\n")
	}
	b.WriteByte('\n')
	if len(m.items) == 0 && !m.busy && m.errText == "" {
		b.WriteString("(koszyk pusty)\n")
	} else {
		w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAZWA\tPRODUCT ID\tILOŚĆ\tCENA JEDN.")
		for i, line := range m.items {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			name := line.name
			if name == "" {
				name = "—"
			}
			price := line.unitPrice
			if price == "" {
				price = "—"
			}
			_, _ = fmt.Fprintf(w, "%s%s\t%s\t%d\t%s\n",
				prefix, truncate(name, 48), line.productID, line.quantity, price)
		}
		_ = w.Flush()
	}
	if m.errText != "" {
		b.WriteString("\nBłąd: ")
		b.WriteString(m.errText)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m cartModel) loadCartCmd() tea.Cmd {
	sess := m.sess
	uid := m.uid
	return func() tea.Msg {
		path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
		result, err := httpclient.RequestJSON(sess, "GET", path, httpclient.RequestOpts{})
		if err != nil {
			return cartDataMsg{err: err}
		}
		lines, perr := parseCartPayload(result)
		if perr != nil {
			return cartDataMsg{err: perr}
		}
		return cartDataMsg{lines: lines}
	}
}

func (m cartModel) putQuantityCmd(productID string, quantity int) tea.Cmd {
	sess := m.sess
	uid := m.uid
	return func() tea.Msg {
		path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
		body := map[string]any{
			"products": []any{
				map[string]any{"productId": productID, "quantity": quantity},
			},
		}
		_, err := httpclient.RequestJSON(sess, "PUT", path, httpclient.RequestOpts{
			Data:       body,
			DataFormat: httpclient.FormatJSON,
		})
		if err != nil {
			return cartDataMsg{err: err}
		}
		result, err := httpclient.RequestJSON(sess, "GET", path, httpclient.RequestOpts{})
		if err != nil {
			return cartDataMsg{err: err}
		}
		lines, perr := parseCartPayload(result)
		if perr != nil {
			return cartDataMsg{err: perr}
		}
		return cartDataMsg{lines: lines}
	}
}

func clampCursor(c, n int) int {
	if n == 0 {
		return 0
	}
	if c >= n {
		return n - 1
	}
	if c < 0 {
		return 0
	}
	return c
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func parseCartPayload(data any) ([]cartLine, error) {
	if data == nil {
		return nil, nil
	}
	root, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("oczekiwano obiektu JSON koszyka")
	}
	arr := firstArray(root,
		"products", "items", "lineItems", "cartItems", "lines", "Lines",
	)
	if arr == nil {
		return nil, nil
	}
	out := make([]cartLine, 0, len(arr))
	for _, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, lineFromMap(m))
	}
	return out, nil
}

func firstArray(root map[string]any, keys ...string) []any {
	for _, k := range keys {
		v, ok := root[k]
		if !ok {
			continue
		}
		if a, ok := v.([]any); ok {
			return a
		}
	}
	return nil
}

func lineFromMap(m map[string]any) cartLine {
	id := stringField(m, "productId", "product_id", "id", "productID", "ProductId")
	qty, _ := intField(m, "quantity", "Quantity", "qty", "count")
	name := stringField(m, "name", "productName", "title", "displayName", "productTitle")
	price := formatUnitPrice(m)
	return cartLine{
		productID: id,
		quantity:  qty,
		name:      name,
		unitPrice: price,
	}
}

func stringField(m map[string]any, keys ...string) string {
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

func intField(m map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if n, ok := anyToInt(v); ok {
				return n, true
			}
		}
	}
	return 0, false
}

func anyToInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case float32:
		return int(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func formatUnitPrice(m map[string]any) string {
	for _, k := range []string{"unitPrice", "unitGrossPrice", "grossUnitPrice", "priceGross", "grossPrice", "price"} {
		if v, ok := m[k]; ok {
			if s := formatMoneyValue(v); s != "" {
				return s
			}
		}
	}
	// Nested price objects
	for _, k := range []string{"price", "unitPrice", "gross", "net"} {
		if v, ok := m[k]; ok {
			if nested, ok := v.(map[string]any); ok {
				for _, nk := range []string{"gross", "amount", "value", "net"} {
					if s := formatMoneyValue(nested[nk]); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func formatMoneyValue(v any) string {
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
		if s := formatMoneyValue(x["gross"]); s != "" {
			return s
		}
		if s := formatMoneyValue(x["amount"]); s != "" {
			return s
		}
		if s := formatMoneyValue(x["value"]); s != "" {
			return s
		}
	}
	return ""
}
