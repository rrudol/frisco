package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/picker"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
	"github.com/rrudol/frisco/internal/tui"
)

func newCartCmd() *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Cart operations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			return tui.RunCart(s, uid)
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "")
	cmd.AddCommand(newCartShowCmd(), newCartAddCmd(), newCartAddBatchCmd(), newCartRemoveCmd(), newCartRemoveBatchCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	var userID string
	var sortBy string
	var top int
	c := &cobra.Command{
		Use:   "show",
		Short: "Fetch cart.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			opts := cartShowOpts{sortBy: sortBy, top: top}
			if err := printCartSummary(result, opts); err == nil {
				return nil
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().StringVar(&sortBy, "sort-by", "", "Sort items by: total, price-per-kg, name. Default: keep API order.")
	c.Flags().IntVar(&top, "top", 0, "Show only top N items (0 = show all).")
	return c
}

// cartShowOpts holds display options for printCartSummary.
type cartShowOpts struct {
	sortBy string // "total" | "price-per-kg" | "name" | ""
	top    int    // 0 = show all
}

// cartItem is a normalised, display-ready cart line.
type cartItem struct {
	pid        string
	name       string
	qty        int
	grammage   string
	unit       string
	unitPrice  float64
	total      float64
	pricePerKg float64
}

func printCartSummary(v any, opts cartShowOpts) error {
	root, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected cart payload")
	}
	rawProducts, ok := root["products"].([]any)
	if !ok {
		return fmt.Errorf("missing products list")
	}

	// Build normalised items.
	items := make([]cartItem, 0, len(rawProducts))
	for _, raw := range rawProducts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pid := asString(entry["productId"])
		qty := asInt(entry["quantity"])

		var product map[string]any
		if p, ok := entry["product"].(map[string]any); ok {
			product = p
		}

		name := shared.ProductNameFromMap(product)
		if name == "" {
			name = pid
		}

		// grammage and unit from product
		grammage := ""
		unit := ""
		if product != nil {
			if g, ok := product["grammage"].(string); ok {
				grammage = strings.TrimSpace(g)
			}
			if u, ok := product["unitOfMeasure"].(string); ok {
				unit = strings.TrimSpace(u)
			}
		}

		// unit price: prefer item-level price, fall back to product price
		unitPriceStr := shared.MoneyString(entry["price"])
		if unitPriceStr == "" && product != nil {
			unitPriceStr = shared.MoneyString(product["price"])
		}
		unitPrice, _ := parseMoneyFloat(unitPriceStr)

		// total
		totalStr := shared.MoneyString(entry["total"])
		var lineTotal float64
		if totalStr != "" {
			lineTotal, _ = parseMoneyFloat(totalStr)
		} else if unitPrice > 0 && qty > 0 {
			lineTotal = unitPrice * float64(qty)
		}

		// price per kg/litre — calculated when unit is weight/volume based
		pricePerKg := 0.0
		unitNorm := strings.ToLower(unit)
		if unitNorm == "kilogram" || unitNorm == "litre" {
			if g, ok := parseGrammageKg(grammage); ok && g > 0 && unitPrice > 0 {
				pricePerKg = unitPrice / g
			}
		}
		// also try pricePerUnit from product map as a direct fallback
		if pricePerKg == 0 && product != nil {
			if pkObj, ok := product["pricePerUnit"].(map[string]any); ok {
				if pv, ok := pkObj["price"].(float64); ok {
					pricePerKg = pv
				}
			}
			if pricePerKg == 0 {
				if pv, ok := product["pricePerKg"].(float64); ok {
					pricePerKg = pv
				}
			}
		}

		items = append(items, cartItem{
			pid:        pid,
			name:       name,
			qty:        qty,
			grammage:   grammage,
			unit:       unit,
			unitPrice:  unitPrice,
			total:      lineTotal,
			pricePerKg: pricePerKg,
		})
	}

	// Sort.
	switch strings.ToLower(strings.TrimSpace(opts.sortBy)) {
	case "total":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].total > items[j].total
		})
	case "price-per-kg":
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i].pricePerKg, items[j].pricePerKg
			if a == 0 {
				return false
			}
			if b == 0 {
				return true
			}
			return a > b
		})
	case "name":
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})
	}

	// Limit.
	if opts.top > 0 && opts.top < len(items) {
		items = items[:opts.top]
	}

	// Print table.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tPRODUCT ID\tQTY\tGRAMMAGE\tUNIT\tUNIT PRICE\tPRICE/KG\tTOTAL")
	grandTotal := 0.0
	for _, it := range items {
		ppkgStr := "-"
		if it.pricePerKg > 0 {
			ppkgStr = fmt.Sprintf("%.2f", it.pricePerKg)
		}
		unitPriceStr := "-"
		if it.unitPrice > 0 {
			unitPriceStr = fmt.Sprintf("%.2f", it.unitPrice)
		}
		totalStr := "-"
		if it.total > 0 {
			totalStr = fmt.Sprintf("%.2f", it.total)
			grandTotal += it.total
		}
		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			shared.TruncateText(it.name, 48),
			it.pid,
			it.qty,
			fallbackDash(it.grammage),
			fallbackDash(it.unit),
			unitPriceStr,
			ppkgStr,
			totalStr,
		)
	}
	_ = w.Flush()

	if totalByStore, ok := root["total"].(map[string]any); ok {
		if val := shared.MoneyString(totalByStore["_total"]); val != "" {
			_, _ = fmt.Printf("\nCart total: %s\n", val)
		}
	} else if grandTotal > 0 {
		_, _ = fmt.Printf("\nCart total: %.2f\n", grandTotal)
	}
	return nil
}

// parseGrammageKg parses a grammage string like "500 g", "1 kg", "250ml", "1.5 l"
// and returns the value in kilograms (or litres, treated equally).
func parseGrammageKg(s string) (float64, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	var val float64
	var unit string
	// try "val unit" with optional space
	if n, _ := fmt.Sscanf(s, "%f %s", &val, &unit); n < 2 {
		if n2, _ := fmt.Sscanf(s, "%f%s", &val, &unit); n2 < 2 {
			return 0, false
		}
	}
	unit = strings.TrimSpace(unit)
	switch unit {
	case "kg", "l", "litre", "litres", "liter", "liters":
		return val, true
	case "g", "ml":
		return val / 1000.0, true
	}
	return 0, false
}


// asString coerces any value to a trimmed string, returning "" for nil.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// asInt coerces a numeric any value to int, returning 0 for unrecognised types.
func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}


// parseMoneyFloat parses a money string (comma or dot decimal) to float64.
func parseMoneyFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" || s == "-" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, false
	}
	return f, true
}


// fallbackDash returns s unchanged, or "-" when s is blank.
func fallbackDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func newCartAddCmd() *cobra.Command {
	var userID, productID, searchPhrase, categoryID string
	var quantity int
	c := &cobra.Command{
		Use:   "add",
		Short: "Add/set product quantity in cart.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutual exclusivity: exactly one of --product-id / --search required.
			hasProductID := strings.TrimSpace(productID) != ""
			hasSearch := strings.TrimSpace(searchPhrase) != ""
			if hasProductID && hasSearch {
				return fmt.Errorf("--product-id and --search are mutually exclusive; provide only one")
			}
			if !hasProductID && !hasSearch {
				return fmt.Errorf("one of --product-id or --search is required")
			}

			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}

			// Resolve product ID via search when --search is given.
			if hasSearch {
				pid, err := resolveProductBySearch(s, uid, searchPhrase, categoryID)
				if err != nil {
					return err
				}
				productID = pid
			}

			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			body := map[string]any{
				"products": []any{
					map[string]any{"productId": productID, "quantity": quantity},
				},
			}
			result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "Product ID to add.")
	c.Flags().StringVar(&searchPhrase, "search", "", "Search phrase to find a product (mutually exclusive with --product-id).")
	c.Flags().StringVar(&categoryID, "category-id", "", "Category ID to narrow search results (only used with --search).")
	c.Flags().IntVar(&quantity, "quantity", 1, "Quantity to set in cart.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

// searchMinScore is the minimum picker score required to auto-select a product.
const searchMinScore = 0.5

// resolveProductBySearch searches for products matching phrase, picks the best
// available match and returns its product ID. When no good match is found it
// prints up to 3 candidates and returns an error asking the user to retry with
// --product-id.
func resolveProductBySearch(s *session.Session, uid, phrase, categoryID string) (string, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
	q := []string{
		"purpose=Listing",
		"pageIndex=1",
		fmt.Sprintf("search=%s", phrase),
		"includeFacets=false",
		"deliveryMethod=Van",
		"pageSize=84",
		"language=pl",
		"disableAutocorrect=false",
	}
	if strings.TrimSpace(categoryID) != "" {
		q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(categoryID)))
	}

	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
	if err != nil {
		return "", fmt.Errorf("product search failed: %w", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected search response format")
	}
	rawProducts, _ := m["products"].([]any)
	if len(rawProducts) == 0 {
		return "", fmt.Errorf("no products found for search phrase %q", phrase)
	}

	products := picker.NormaliseProducts(rawProducts)
	best, top3, ok := picker.Pick(products, phrase, searchMinScore)

	if !ok {
		fmt.Printf("No strong match found for %q (score < %.1f).\n\n", phrase, searchMinScore)
		if len(top3) > 0 {
			fmt.Println("Top results (use --product-id to add one of these):")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "PRODUCT ID\tNAME\tPRICE\tGRAMMAGE\tPRICE/KG")
			for _, r := range top3 {
				p := r.Product
				priceStr := fmt.Sprintf("%.2f", p.Price)
				ppkgStr := "-"
				if p.PricePerKg > 0 {
					ppkgStr = fmt.Sprintf("%.2f", p.PricePerKg)
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ProductID,
					shared.TruncateText(p.Name, 50),
					priceStr,
					fallbackDash(p.Grammage),
					ppkgStr,
				)
			}
			_ = w.Flush()
		}
		return "", fmt.Errorf("use --product-id with one of the product IDs above")
	}

	// Print the picked product before adding.
	ppkgStr := "-"
	if best.PricePerKg > 0 {
		ppkgStr = fmt.Sprintf("%.2f /kg", best.PricePerKg)
	}
	fmt.Printf("Picked: %s  [%s]  %.2f PLN  %s  %s\n",
		best.Name,
		best.ProductID,
		best.Price,
		fallbackDash(best.Grammage),
		ppkgStr,
	)

	return best.ProductID, nil
}

func newCartRemoveCmd() *cobra.Command {
	var userID, productID string
	c := &cobra.Command{
		Use:   "remove",
		Short: "Remove product from cart (quantity=0).",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			body := map[string]any{
				"products": []any{
					map[string]any{"productId": productID, "quantity": 0},
				},
			}
			result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-id")
	return c
}
