package commands

import (
	"fmt"
	"os"
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
	cmd.AddCommand(newCartShowCmd(), newCartAddCmd(), newCartAddBatchCmd(), newCartRemoveCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	var userID string
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
			if err := printCartSummary(result); err == nil {
				return nil
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func printCartSummary(v any) error {
	root, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected cart payload")
	}
	rawProducts, ok := root["products"].([]any)
	if !ok {
		return fmt.Errorf("missing products list")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tPRODUCT ID\tQTY\tUNIT PRICE\tTOTAL")
	grandTotal := 0.0
	for _, raw := range rawProducts {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pid := asString(item["productId"])
		qty := asInt(item["quantity"])

		var product map[string]any
		if p, ok := item["product"].(map[string]any); ok {
			product = p
		}
		name := shared.ProductNameFromMap(product)
		if name == "" {
			name = pid
		}

		unitPrice := shared.MoneyString(item["price"])
		if unitPrice == "" && product != nil {
			unitPrice = shared.MoneyString(product["price"])
		}
		total := shared.MoneyString(item["total"])
		if total == "" {
			if p, ok := parseMoneyFloat(unitPrice); ok && qty > 0 {
				lineTotal := p * float64(qty)
				total = fmt.Sprintf("%.2f", lineTotal)
				grandTotal += lineTotal
			}
		} else if p, ok := parseMoneyFloat(total); ok {
			grandTotal += p
		}

		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\n",
			shared.TruncateText(name, 54),
			pid,
			qty,
			fallbackDash(unitPrice),
			fallbackDash(total),
		)
	}
	_ = w.Flush()

	if totalByStore, ok := root["total"].(map[string]any); ok {
		if val := shared.MoneyString(totalByStore["_total"]); val != "" {
			_, _ = fmt.Printf("\n%s %s\n", "Cart total:", val)
		}
	} else if grandTotal > 0 {
		_, _ = fmt.Printf("\n%s %.2f\n", "Cart total:", grandTotal)
	}
	return nil
}


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
