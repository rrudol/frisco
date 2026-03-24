package commands

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/picker"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
)

func newProductsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "products",
		Short: "Product operations.",
	}
	cmd.AddCommand(newProductsSearchCmd(), newProductsByIDsCmd(), newProductsNutritionCmd(), newProductsPickCmd())
	return cmd
}

func newProductsSearchCmd() *cobra.Command {
	var (
		search, deliveryMethod, userID, categoryID string
		pageIndex, pageSize                        int
		rawOutput                                  bool
	)
	c := &cobra.Command{
		Use:   "search",
		Short: "Search products.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
			q := []string{
				"purpose=Listing",
				fmt.Sprintf("pageIndex=%d", pageIndex),
				fmt.Sprintf("search=%s", search),
				"includeFacets=true",
				fmt.Sprintf("deliveryMethod=%s", deliveryMethod),
				fmt.Sprintf("pageSize=%d", pageSize),
				"language=pl",
				"disableAutocorrect=false",
			}
			if strings.TrimSpace(categoryID) != "" {
				q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(categoryID)))
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			if rawOutput || strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			return printProductSearchTable(result)
		},
	}
	c.Flags().BoolVar(&rawOutput, "raw", false, "Show full API response")
	c.Flags().StringVar(&search, "search", "", "Search phrase.")
	c.Flags().StringVar(&categoryID, "category-id", "", "Frisco categoryId (narrows listing, e.g. 18703 Warzywa i owoce).")
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 84, "")
	c.Flags().StringVar(&deliveryMethod, "delivery-method", "Van", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("search")
	return c
}

func newProductsByIDsCmd() *cobra.Command {
	var userID string
	var productIDs []string
	c := &cobra.Command{
		Use:   "by-ids",
		Short: "Fetch products by productIds.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
			var q []string
			for _, pid := range productIDs {
				q = append(q, fmt.Sprintf("productIds=%s", pid))
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringArrayVar(&productIDs, "product-id", nil, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-id")
	return c
}

func newProductsNutritionCmd() *cobra.Command {
	var productID string
	var rawOutput bool
	c := &cobra.Command{
		Use:   "nutrition",
		Short: "Fetch product nutrition values (if available).",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/content/api/v1/products/get/%s", productID)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			if rawOutput {
				return printJSON(result)
			}

			nutrition := shared.ExtractNutritionBlock(result)
			if nutrition == nil {
				return printJSON(map[string]any{
					"productId": productID,
					"message": "No explicit nutrition values found in this endpoint. Use --raw to inspect full response.",
				})
			}
			return printJSON(map[string]any{
				"productId":  productID,
				"nutrition":  nutrition,
				"sourcePath": "/app/content/api/v1/products/get/{id}",
			})
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "Product ID")
	c.Flags().BoolVar(&rawOutput, "raw", false, "Show full API response")
	_ = c.MarkFlagRequired("product-id")
	return c
}

// ---------------------------------------------------------------------------
// products pick
// ---------------------------------------------------------------------------

// pickCandidate holds a scored product candidate for the pick subcommand.
type pickCandidate struct {
	productID    string
	name         string
	price        float64
	grammage     float64
	unit         string
	pricePerUnit float64
	matchScore   float64
	packBonus    float64
	bulkPenalty  float64
	finalScore   float64
}

var bulkKeywords = []string{"zestaw", "multipack", "pakiet", "opakowanie zbiorcze"}

// packBonusScore returns a bonus (0-1) for pack sizes in the preferred range.
// If preferSize > 0, it favours packs nearest to that size instead.
func packBonusScore(grammage, preferSize float64) float64 {
	if grammage <= 0 {
		return 0
	}
	if grammage < 0.05 || grammage > 3.0 {
		return 0
	}
	if preferSize > 0 {
		ratio := grammage / preferSize
		if ratio > 1 {
			ratio = 1 / ratio
		}
		return ratio
	}
	// Default: prefer 0.1–1.5 kg.
	if grammage >= 0.1 && grammage <= 1.5 {
		return 1.0
	}
	if grammage < 0.1 {
		return grammage / 0.1
	}
	// 1.5–3.0: linear decay to 0.
	return 1.0 - (grammage-1.5)/1.5
}

func hasBulkKeyword(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range bulkKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func scorePick(c *pickCandidate, searchPhrase string, preferSize float64) {
	// Use picker.Score for name matching (tokenised, case-folded).
	c.matchScore = picker.Score(c.name, searchPhrase)
	c.packBonus = packBonusScore(c.grammage, preferSize)
	if hasBulkKeyword(c.name) {
		c.bulkPenalty = 1.0
	}
	ppu := c.pricePerUnit
	if ppu <= 0 {
		ppu = 999
	}
	// Normalise price contribution 0-1 (cap at 50 PLN/kg).
	priceScore := math.Max(0, 1.0-ppu/50.0)
	c.finalScore = c.matchScore*3.0 + priceScore*0.5 + c.packBonus*0.3 - c.bulkPenalty*2.0
}

func extractPickCandidates(result any, searchPhrase string, preferSize float64) []pickCandidate {
	m, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	rawProducts, _ := m["products"].([]any)
	var candidates []pickCandidate
	for _, raw := range rawProducts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := entry["product"].(map[string]any)
		if inner == nil {
			continue
		}
		// Availability filter.
		if av, ok := inner["isAvailable"].(bool); ok && !av {
			continue
		}
		if st, ok := inner["isStocked"].(bool); ok && !st {
			continue
		}

		pid := asString(entry["productId"])
		// Prefer PL name for scoring; fall back to LocalizedString.
		name := ""
		if nameMap, ok := inner["name"].(map[string]any); ok {
			if pl, ok := nameMap["pl"].(string); ok && pl != "" {
				name = pl
			}
		}
		if name == "" {
			name = shared.LocalizedString(inner["name"])
		}
		unit, _ := inner["unit"].(string) // "unit" is null in many Frisco responses
		var price float64
		if priceObj, ok := inner["price"].(map[string]any); ok {
			if pv, ok := priceObj["price"].(float64); ok {
				price = pv
			}
		}
		var grammage float64
		if gv, ok := inner["grammage"].(float64); ok {
			grammage = gv
		}

		// When the API returns a null unit but a numeric grammage, treat it as kg.
		if unit == "" && grammage > 0 {
			unit = "kg"
		}

		var pricePerUnit float64
		u := strings.ToLower(unit)
		if (u == "kilogram" || u == "kg" || u == "litre") && grammage > 0 {
			pricePerUnit = price / grammage
		} else {
			pricePerUnit = price
		}

		c := pickCandidate{
			productID:    pid,
			name:         name,
			price:        price,
			grammage:     grammage,
			unit:         unit,
			pricePerUnit: pricePerUnit,
		}
		scorePick(&c, searchPhrase, preferSize)
		candidates = append(candidates, c)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].finalScore > candidates[j].finalScore
	})
	return candidates
}

func newProductsPickCmd() *cobra.Command {
	var (
		search, categoryID, userID, deliveryMethod string
		topN                                        int
		preferSize                                  float64
	)
	c := &cobra.Command{
		Use:   "pick",
		Short: "Search for a product and return the best match based on name, price/kg and pack size.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
			q := []string{
				"purpose=Listing",
				"pageIndex=1",
				fmt.Sprintf("search=%s", search),
				"includeFacets=false",
				fmt.Sprintf("deliveryMethod=%s", deliveryMethod),
				"pageSize=84",
				"language=pl",
				"disableAutocorrect=false",
			}
			if strings.TrimSpace(categoryID) != "" {
				q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(categoryID)))
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}

			candidates := extractPickCandidates(result, search, preferSize)
			if len(candidates) == 0 {
				return fmt.Errorf("no available products found for search %q", search)
			}
			if topN < 1 {
				topN = 1
			}
			if topN > len(candidates) {
				topN = len(candidates)
			}
			top := candidates[:topN]

			if strings.EqualFold(outputFormat, "json") {
				best := top[0]
				return printJSON(map[string]any{
					"product_id":     best.productID,
					"name":           best.name,
					"price":          best.price,
					"grammage":       best.grammage,
					"unit":           best.unit,
					"price_per_unit": math.Round(best.pricePerUnit*100) / 100,
					"match_score":    math.Round(best.matchScore*100) / 100,
				})
			}

			// Table output.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "BEST\tid\tname\tprice\tgrammage\tunit\tprice/kg\tscore")
			for i, c := range top {
				marker := ""
				if i == 0 {
					marker = "*"
				}
				ppuStr := "-"
				if c.pricePerUnit > 0 {
					ppuStr = fmt.Sprintf("%.2f", c.pricePerUnit)
				}
				gramStr := "-"
				if c.grammage > 0 {
					gramStr = fmt.Sprintf("%.3g", c.grammage)
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%s\t%s\t%s\t%.2f\n",
					marker, c.productID, c.name, c.price, gramStr, c.unit, ppuStr,
					c.matchScore)
			}
			return w.Flush()
		},
	}
	c.Flags().StringVar(&search, "search", "", "Search phrase (required).")
	c.Flags().StringVar(&categoryID, "category-id", "", "Frisco categoryId to narrow listing.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().StringVar(&deliveryMethod, "delivery-method", "Van", "")
	c.Flags().IntVar(&topN, "top", 1, "Show top N candidates (default 1).")
	c.Flags().Float64Var(&preferSize, "prefer-size", 0, "Preferred pack size in kg (e.g. 0.5). 0 = use default range heuristic.")
	_ = c.MarkFlagRequired("search")
	return c
}

func printProductSearchTable(result any) error {
	m, ok := result.(map[string]any)
	if !ok {
		return printPretty(result)
	}

	// Pagination info.
	pageIndex, _ := m["pageIndex"].(float64)
	pageCount, _ := m["pageCount"].(float64)
	totalCount, _ := m["totalCount"].(float64)
	fmt.Printf("Page %.0f/%.0f (%.0f results)\n\n", pageIndex, pageCount, totalCount)

	rawProducts, _ := m["products"].([]any)
	if len(rawProducts) == 0 {
		fmt.Println("No products found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "id\tname\tbrand\tprice\tgrammage\tunit\tprice/kg\tavailable")
	for _, raw := range rawProducts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := cellValue(entry["productId"])
		inner, _ := entry["product"].(map[string]any)

		name := ""
		brand := ""
		price := ""
		grammage := ""
		unit := ""
		pricePerKg := ""
		available := ""
		if inner != nil {
			name = shared.LocalizedString(inner["name"])
			brand, _ = inner["brand"].(string)
			var priceVal float64
			if priceObj, ok := inner["price"].(map[string]any); ok {
				if pv, ok := priceObj["price"].(float64); ok {
					priceVal = pv
					price = fmt.Sprintf("%.2f", pv)
				}
			}
			var gramVal float64
			if gv, ok := inner["grammage"].(float64); ok && gv > 0 {
				gramVal = gv
				grammage = fmt.Sprintf("%.3g", gv)
			}
			unit, _ = inner["unit"].(string)
			// Default unit to "kg" when grammage is present but unit is null.
			if unit == "" && gramVal > 0 {
				unit = "kg"
			}
			if priceVal > 0 && gramVal > 0 {
				u := strings.ToLower(unit)
				if u == "kilogram" || u == "kg" || u == "litre" {
					pricePerKg = fmt.Sprintf("%.2f", priceVal/gramVal)
				}
			}
			if av, ok := inner["isAvailable"].(bool); ok {
				available = fmt.Sprintf("%v", av)
			}
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id, name, brand, price, grammage, unit, pricePerKg, available)
	}
	return w.Flush()
}

