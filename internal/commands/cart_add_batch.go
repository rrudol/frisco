package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

// cartBatchLine is one row after parsing and optional aggregation.
type cartBatchLine struct {
	productID string
	quantity  int
}

func newCartAddBatchCmd() *cobra.Command {
	var userID, file string
	var dryRun bool
	c := &cobra.Command{
		Use:   "add-batch",
		Short: tr(
			"Add many products from a JSON file (search for IDs first).",
			"Dodaj wiele produktów z pliku JSON (najpierw wyszukaj ID produktów).",
		),
		Long: tr(
			"JSON file: array or {\"items\":[...]}. product_id/productId, quantity/qty (default 1). Duplicates in file: quantities summed. Loads current cart (GET), applies batch quantities on top, then one PUT with full cart so nothing is wiped.",
			"Plik JSON: tablica lub {\"items\":[...]}. product_id/productId, quantity/qty (domyślnie 1). Duplikaty w pliku: suma ilości. Pobiera koszyk (GET), nakłada ilości z batcha, jeden PUT z całą zawartością — nic nie ginie jak przy wielu PUTach.",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			lines, err := parseCartBatchFile(file)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				return errors.New(tr("No products in file.", "Brak produktów w pliku."))
			}
			if dryRun {
				return printCartBatchDryRun(lines)
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			// Frisco PUT replaces the entire cart — merge with current GET, then one PUT.
			current, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			qtyMap := quantitiesFromCartGET(current)
			for _, line := range lines {
				qtyMap[line.productID] = line.quantity
			}
			products := mergedCartProductsSlice(qtyMap)
			if len(products) == 0 {
				return errors.New(tr("No products to put in cart.", "Brak produktów do zapisania w koszyku."))
			}
			body := map[string]any{"products": products}
			last, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(map[string]any{
					"added":       lines,
					"mergedLines": len(products),
					"putCart":     last,
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), tr("Merged cart: %d line(s) (batch touched %d product id(s)).\n", "Scalono koszyk: %d pozycji (batch: %d productId).\n"), len(products), len(lines))
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return printJSON(last)
			}
			if err := printCartSummary(result); err == nil {
				return nil
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&file, "file", "", tr("Path to JSON file.", "Ścieżka do pliku JSON."))
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, tr("Parse file and print lines; do not call API.", "Parsuj plik i wypisz wiersze; bez wywołań API."))
	_ = c.MarkFlagRequired("file")
	return c
}

func printCartBatchDryRun(lines []cartBatchLine) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, tr("PRODUCT ID\tQTY", "PRODUCT ID\tILOŚĆ"))
	for _, line := range lines {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", line.productID, line.quantity)
	}
	_ = w.Flush()
	return nil
}

func parseCartBatchFile(path string) ([]cartBatchLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	merged, err := parseCartBatchJSON(data)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(merged))
	for id := range merged {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]cartBatchLine, 0, len(ids))
	for _, id := range ids {
		out = append(out, cartBatchLine{productID: id, quantity: merged[id]})
	}
	return out, nil
}

func parseCartBatchJSON(data []byte) (map[string]int, error) {
	var top any
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}
	var rawList []any
	switch t := top.(type) {
	case []any:
		rawList = t
	case map[string]any:
		if items, ok := t["items"].([]any); ok {
			rawList = items
		} else {
			return nil, errors.New(
				tr("JSON must be an array or an object with \"items\" array.", "JSON musi być tablicą albo obiektem z tablicą \"items\"."),
			)
		}
	default:
		return nil, errors.New(
			tr("JSON must be an array or an object with \"items\" array.", "JSON musi być tablicą albo obiektem z tablicą \"items\"."),
		)
	}
	out := make(map[string]int)
	for i, el := range rawList {
		m, ok := el.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s", fmt.Sprintf(tr("item %d: expected object", "pozycja %d: oczekiwano obiektu"), i+1))
		}
		pid := batchProductID(m)
		if pid == "" {
			return nil, fmt.Errorf("%s", fmt.Sprintf(tr("item %d: missing product_id / productId", "pozycja %d: brak product_id / productId"), i+1))
		}
		q, err := batchQuantity(m, i+1)
		if err != nil {
			return nil, err
		}
		out[pid] += q
	}
	return out, nil
}

func batchProductID(m map[string]any) string {
	for _, k := range []string{"product_id", "productId", "productid"} {
		if s := strings.TrimSpace(asString(m[k])); s != "" {
			return s
		}
	}
	return ""
}

func batchQuantity(m map[string]any, itemIndex int) (int, error) {
	for _, k := range []string{"quantity", "qty"} {
		if _, ok := m[k]; !ok {
			continue
		}
		q := asInt(m[k])
		if q < 1 {
			return 0, fmt.Errorf("%s", fmt.Sprintf(tr("item %d: quantity must be >= 1", "pozycja %d: ilość musi być >= 1"), itemIndex))
		}
		return q, nil
	}
	return 1, nil
}

// quantitiesFromCartGET maps productId -> quantity from GET /cart (best-effort for API shapes).
func quantitiesFromCartGET(data any) map[string]int {
	out := make(map[string]int)
	root, ok := data.(map[string]any)
	if !ok {
		return out
	}
	arr, _ := root["products"].([]any)
	for _, el := range arr {
		row, ok := el.(map[string]any)
		if !ok {
			continue
		}
		pid := asString(row["productId"])
		if pid == "" {
			if p, ok := row["product"].(map[string]any); ok {
				pid = asString(p["productId"])
			}
		}
		if pid == "" {
			continue
		}
		q := asInt(row["quantity"])
		if q > 0 {
			out[pid] = q
		}
	}
	return out
}

func mergedCartProductsSlice(qtyMap map[string]int) []any {
	ids := make([]string, 0, len(qtyMap))
	for id, q := range qtyMap {
		if q > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"productId": id, "quantity": qtyMap[id]})
	}
	return out
}
