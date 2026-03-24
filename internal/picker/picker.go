// Package picker provides a simple best-match product picker for Frisco search results.
package picker

import (
	"fmt"
	"strings"

	"github.com/rrudol/frisco/internal/shared"
)

// Product is a normalised view of a single product entry from the search API.
type Product struct {
	ProductID  string
	Name       string
	Brand      string
	Price      float64 // price per unit (from price.price)
	Grammage   string  // e.g. "500 g"
	PricePerKg float64 // price / weight in kg; 0 when unavailable
	Available  bool

	// raw entry so callers can pass it forward if needed
	Raw map[string]any
}

// NormaliseProducts converts a raw products slice (as returned by the search
// API) into a slice of Product values.
func NormaliseProducts(rawProducts []any) []Product {
	out := make([]Product, 0, len(rawProducts))
	for _, r := range rawProducts {
		entry, ok := r.(map[string]any)
		if !ok {
			continue
		}
		pid := strings.TrimSpace(fmt.Sprint(entry["productId"]))
		inner, _ := entry["product"].(map[string]any)
		if inner == nil {
			inner = entry
		}

		name := shared.ProductNameFromMap(inner)
		brand, _ := inner["brand"].(string)

		var price float64
		if priceObj, ok := inner["price"].(map[string]any); ok {
			if pv, ok := priceObj["price"].(float64); ok {
				price = pv
			}
		}

		var available bool
		if av, ok := inner["isAvailable"].(bool); ok {
			available = av
		}

		// grammage / price-per-kg
		grammage := ""
		pricePerKg := 0.0
		if g, ok := inner["grammage"].(string); ok {
			grammage = strings.TrimSpace(g)
		}
		if pkObj, ok := inner["pricePerUnit"].(map[string]any); ok {
			if pv, ok := pkObj["price"].(float64); ok {
				pricePerKg = pv
			}
		}
		if pricePerKg == 0 {
			// fallback: try "pricePerKg" key directly
			if pv, ok := inner["pricePerKg"].(float64); ok {
				pricePerKg = pv
			}
		}

		_ = brand // kept in struct for completeness

		out = append(out, Product{
			ProductID:  pid,
			Name:       name,
			Brand:      brand,
			Price:      price,
			Grammage:   grammage,
			PricePerKg: pricePerKg,
			Available:  available,
			Raw:        entry,
		})
	}
	return out
}

// Score returns a match score in [0,1] for a product name against a search
// phrase.  It tokenises both strings (lowercase, split on whitespace) and
// counts what fraction of the query tokens appear in the name tokens.
func Score(productName, searchPhrase string) float64 {
	queryTokens := tokenise(searchPhrase)
	if len(queryTokens) == 0 {
		return 0
	}
	nameTokens := tokenise(productName)
	nameSet := make(map[string]struct{}, len(nameTokens))
	for _, t := range nameTokens {
		nameSet[t] = struct{}{}
	}
	matched := 0
	for _, qt := range queryTokens {
		if _, ok := nameSet[qt]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(queryTokens))
}

func tokenise(s string) []string {
	parts := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		// strip common punctuation
		p = strings.Trim(p, ".,;:!?\"'()")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Result pairs a product with its match score.
type Result struct {
	Product Product
	Score   float64
}

// Pick finds the best-matching available product from products for the given
// search phrase.
//
// Rules (in order of priority):
//  1. Only available products are considered.
//  2. Best match score wins.
//  3. Ties broken by lowest PricePerKg (then lowest Price).
//
// Returns (best, topN, ok).
//   - ok is false when no available product scores >= minScore.
//   - topN always contains up to 3 candidates (available, scored, sorted), even
//     when ok is false, so callers can display them to the user.
func Pick(products []Product, phrase string, minScore float64) (Product, []Result, bool) {
	var candidates []Result
	for _, p := range products {
		if !p.Available {
			continue
		}
		s := Score(p.Name, phrase)
		if s > 0 {
			candidates = append(candidates, Result{Product: p, Score: s})
		}
	}

	// stable sort: descending score, then ascending pricePerKg, then ascending price
	sortResults(candidates)

	// top-3 for display
	top3 := candidates
	if len(top3) > 3 {
		top3 = top3[:3]
	}

	if len(candidates) == 0 || candidates[0].Score < minScore {
		return Product{}, top3, false
	}
	return candidates[0].Product, top3, true
}

// sortResults sorts in place: descending score, ascending pricePerKg, ascending price.
func sortResults(rs []Result) {
	n := len(rs)
	// simple insertion sort – result sets are small
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(rs[j], rs[j-1]); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func less(a, b Result) bool {
	if a.Score != b.Score {
		return a.Score > b.Score // higher score wins
	}
	if a.Product.PricePerKg != b.Product.PricePerKg {
		if a.Product.PricePerKg == 0 {
			return false
		}
		if b.Product.PricePerKg == 0 {
			return true
		}
		return a.Product.PricePerKg < b.Product.PricePerKg
	}
	return a.Product.Price < b.Product.Price
}
