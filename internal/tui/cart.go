package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"log"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
)

// cartLine is one row from GET /cart (parsed defensively for varying API shapes).
type cartLine struct {
	productID string
	quantity  int
	name      string
	unitPrice string
}

type productDetails struct {
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
	sess          *session.Session
	uid           string
	items         []cartLine
	cursor        int
	busy          bool
	errText       string
	confirmDelete bool
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
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				if len(m.items) == 0 {
					m.confirmDelete = false
					return m, nil
				}
				line := m.items[m.cursor]
				m.busy = true
				m.errText = ""
				m.confirmDelete = false
				return m, m.putQuantityCmd(line.productID, 0)
			case "n", "N", "esc":
				m.confirmDelete = false
				return m, nil
			default:
				return m, nil
			}
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
			m.confirmDelete = true
			return m, nil
		case "r":
			m.busy = true
			m.errText = ""
			return m, m.loadCartCmd()
		}
		return m, nil

	case cartDataMsg:
		m.busy = false
		m.confirmDelete = false
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
	b.WriteString("Cart — ↑↓ select  +/− quantity  d delete  r refresh  q quit\n")
	if m.confirmDelete {
		b.WriteString("Confirm delete: y=yes  n=cancel\n")
	}
	if m.busy {
		b.WriteString("\nLoading...\n")
	}
	b.WriteByte('\n')
	if len(m.items) == 0 && !m.busy && m.errText == "" {
		b.WriteString("(cart is empty)\n")
	} else {
		w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tQTY\tUNIT PRICE\tTOTAL PRICE")
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
			total := lineTotalPrice(line.quantity, line.unitPrice)
			_, _ = fmt.Fprintf(w, "%s%s\t%d\t%s\t%s\n",
				prefix, shared.TruncateText(name, 48), line.quantity, price, total)
		}
		_ = w.Flush()
	}
	if m.errText != "" {
		b.WriteString("\nError: ")
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
		lines = enrichCartLines(sess, uid, lines)
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
		lines = enrichCartLines(sess, uid, lines)
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


func parseCartPayload(data any) ([]cartLine, error) {
	if data == nil {
		return nil, nil
	}
	root, ok := data.(map[string]any)
	if !ok {
		return nil, errors.New("expected cart JSON object")
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

func enrichCartLines(sess *session.Session, uid string, lines []cartLine) []cartLine {
	if len(lines) == 0 || sess == nil || uid == "" {
		return lines
	}
	ids := missingDetailsProductIDs(lines)
	if len(ids) == 0 {
		return lines
	}
	details := fetchProductDetailsByIDs(sess, uid, ids)
	if len(details) == 0 {
		return lines
	}
	out := make([]cartLine, len(lines))
	copy(out, lines)
	for i, line := range out {
		d, ok := details[line.productID]
		if !ok {
			continue
		}
		if line.name == "" {
			out[i].name = d.name
		}
		if line.unitPrice == "" {
			out[i].unitPrice = d.unitPrice
		}
	}
	return out
}

func missingDetailsProductIDs(lines []cartLine) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line.productID == "" {
			continue
		}
		if line.name != "" && line.unitPrice != "" {
			continue
		}
		if _, exists := seen[line.productID]; exists {
			continue
		}
		seen[line.productID] = struct{}{}
		out = append(out, line.productID)
	}
	return out
}

func fetchProductDetailsByIDs(sess *session.Session, uid string, productIDs []string) map[string]productDetails {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
	query := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		query = append(query, fmt.Sprintf("productIds=%s", id))
	}
	result, err := httpclient.RequestJSON(sess, "GET", path, httpclient.RequestOpts{Query: query})
	if err != nil {
		log.Printf("fetchProductDetailsByIDs: %v", err)
		return nil
	}
	return parseProductDetailsPayload(result, productIDs)
}

func parseProductDetailsPayload(data any, productIDs []string) map[string]productDetails {
	if data == nil || len(productIDs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(productIDs))
	for _, id := range productIDs {
		allowed[id] = struct{}{}
	}
	out := map[string]productDetails{}

	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			id := shared.StringFieldFromMap(t, "productId", "id", "productID", "ProductId")
			if _, ok := allowed[id]; ok {
				cur := out[id]
				if cur.name == "" {
					cur.name = shared.ProductNameFromMap(t)
				}
				if cur.unitPrice == "" {
					cur.unitPrice = formatUnitPrice(t)
				}
				out[id] = cur
			}
			for _, nested := range t {
				walk(nested)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(data)

	if len(out) == 0 {
		return nil
	}
	return out
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
	id := shared.StringFieldFromMap(m, "productId", "product_id", "id", "productID", "ProductId")
	qty, _ := intField(m, "quantity", "Quantity", "qty", "count")
	name := shared.ProductNameFromMap(m)
	price := formatUnitPrice(m)
	if product, ok := m["product"].(map[string]any); ok {
		if id == "" {
			id = shared.StringFieldFromMap(product, "productId", "product_id", "id", "productID", "ProductId")
		}
		if name == "" {
			name = shared.ProductNameFromMap(product)
		}
		if price == "" {
			price = formatUnitPrice(product)
		}
	}
	return cartLine{
		productID: id,
		quantity:  qty,
		name:      name,
		unitPrice: price,
	}
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
			if s := shared.FormatMoneyValue(v); s != "" {
				return s
			}
		}
	}
	// Nested price objects
	for _, k := range []string{"price", "unitPrice", "gross", "net"} {
		if v, ok := m[k]; ok {
			if nested, ok := v.(map[string]any); ok {
				for _, nk := range []string{"price", "gross", "amount", "value", "net"} {
					if s := shared.FormatMoneyValue(nested[nk]); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}


func lineTotalPrice(quantity int, unitPrice string) string {
	if quantity <= 0 {
		return "—"
	}
	raw := strings.TrimSpace(strings.ReplaceAll(unitPrice, ",", "."))
	if raw == "" || raw == "—" || raw == "-" {
		return "—"
	}
	price, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "—"
	}
	return fmt.Sprintf("%.2f", price*float64(quantity))
}

