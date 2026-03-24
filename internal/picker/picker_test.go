package picker

import (
	"math"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func makeProduct(id, name string, price, pricePerKg float64, available bool) Product {
	return Product{
		ProductID:  id,
		Name:       name,
		Price:      price,
		PricePerKg: pricePerKg,
		Available:  available,
	}
}

// ── Score ─────────────────────────────────────────────────────────────────────

func TestScore_ExactMatch(t *testing.T) {
	s := Score("mleko UHT 3.2%", "mleko uht 3.2%")
	if !floatEq(s, 1.0) {
		t.Fatalf("expected 1.0, got %v", s)
	}
}

func TestScore_PartialMatch(t *testing.T) {
	// "mleko" is one of two query tokens → 0.5
	s := Score("mleko UHT", "mleko ekologiczne")
	if !floatEq(s, 0.5) {
		t.Fatalf("expected 0.5, got %v", s)
	}
}

func TestScore_NoMatch(t *testing.T) {
	s := Score("chleb żytni", "mleko UHT")
	if s != 0 {
		t.Fatalf("expected 0, got %v", s)
	}
}

func TestScore_EmptyPhrase(t *testing.T) {
	s := Score("mleko UHT", "")
	if s != 0 {
		t.Fatalf("expected 0 for empty phrase, got %v", s)
	}
}

func TestScore_EmptyProductName(t *testing.T) {
	s := Score("", "mleko")
	if s != 0 {
		t.Fatalf("expected 0 for empty product name, got %v", s)
	}
}

func TestScore_BothEmpty(t *testing.T) {
	s := Score("", "")
	if s != 0 {
		t.Fatalf("expected 0 for both empty, got %v", s)
	}
}

func TestScore_CaseInsensitive(t *testing.T) {
	s1 := Score("Jogurt Naturalny", "JOGURT NATURALNY")
	s2 := Score("jogurt naturalny", "jogurt naturalny")
	if !floatEq(s1, s2) || !floatEq(s1, 1.0) {
		t.Fatalf("expected both to be 1.0, got %v and %v", s1, s2)
	}
}

func TestScore_PunctuationStripped(t *testing.T) {
	// punctuation around query tokens should be stripped
	s := Score("mleko UHT", "mleko, UHT.")
	if !floatEq(s, 1.0) {
		t.Fatalf("expected 1.0 after punctuation stripping, got %v", s)
	}
}

func TestScore_AllQueryTokensPresent(t *testing.T) {
	s := Score("Masło extra 82% tłuszczu", "masło extra")
	if !floatEq(s, 1.0) {
		t.Fatalf("expected 1.0 when all query tokens are in name, got %v", s)
	}
}

func TestScore_DuplicateQueryTokens(t *testing.T) {
	// "mleko mleko" — two tokens, product name has "mleko" once
	// matched = 1 (second lookup hits same set entry), total query tokens = 2 → 0.5
	// Actually the implementation checks nameSet so both duplicates hit → matched=2
	// The fraction is matched/len(queryTokens) = 2/2 = 1.0
	s := Score("mleko UHT", "mleko mleko")
	if !floatEq(s, 1.0) {
		t.Fatalf("expected 1.0 for duplicate query tokens all in name, got %v", s)
	}
}

// ── tokenise (unexported but exercised via Score) ─────────────────────────────

func TestScore_WhitespaceOnlyPhrase(t *testing.T) {
	s := Score("mleko", "   ")
	if s != 0 {
		t.Fatalf("expected 0 for whitespace-only phrase, got %v", s)
	}
}

// ── Pick ──────────────────────────────────────────────────────────────────────

func TestPick_EmptyProducts(t *testing.T) {
	_, top, ok := Pick(nil, "mleko", 0.5)
	if ok {
		t.Fatal("expected ok=false for empty product list")
	}
	if len(top) != 0 {
		t.Fatalf("expected empty top list, got %v", top)
	}
}

func TestPick_NoAvailableProducts(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko UHT", 3.99, 3.99, false),
		makeProduct("2", "mleko ekologiczne", 6.50, 6.50, false),
	}
	_, _, ok := Pick(products, "mleko", 0.5)
	if ok {
		t.Fatal("expected ok=false when no available products")
	}
}

func TestPick_NoBelowMinScore(t *testing.T) {
	products := []Product{
		makeProduct("1", "chleb żytni", 4.00, 8.00, true),
	}
	_, top, ok := Pick(products, "mleko", 0.5)
	if ok {
		t.Fatal("expected ok=false when no product scores >= minScore")
	}
	// top may be empty (score=0 excluded) — just must not panic
	_ = top
}

func TestPick_SingleMatch(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko UHT 3.2%", 3.99, 3.99, true),
	}
	best, top, ok := Pick(products, "mleko", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best.ProductID != "1" {
		t.Fatalf("expected product 1, got %s", best.ProductID)
	}
	if len(top) == 0 {
		t.Fatal("expected non-empty top list")
	}
}

func TestPick_BestScoreWins(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko UHT 3.2%", 3.99, 3.99, true),          // matches "mleko uht" → 1.0
		makeProduct("2", "mleko ekologiczne pełne", 6.50, 10.00, true), // matches "mleko" → 0.33
	}
	best, _, ok := Pick(products, "mleko UHT", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best.ProductID != "1" {
		t.Fatalf("expected product 1 (higher score), got %s", best.ProductID)
	}
}

func TestPick_TieBrokenByPricePerKg(t *testing.T) {
	// Both products score 1.0 for "mleko"
	products := []Product{
		makeProduct("expensive", "mleko", 5.00, 10.00, true),
		makeProduct("cheap", "mleko", 3.00, 6.00, true),
	}
	best, _, ok := Pick(products, "mleko", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best.ProductID != "cheap" {
		t.Fatalf("expected cheaper pricePerKg product, got %s", best.ProductID)
	}
}

func TestPick_TieBrokenByPriceFallback(t *testing.T) {
	// Both have same score and same PricePerKg=0 (unknown) → fall back to Price
	products := []Product{
		makeProduct("pricey", "mleko", 5.00, 0, true),
		makeProduct("cheap", "mleko", 3.00, 0, true),
	}
	best, _, ok := Pick(products, "mleko", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best.ProductID != "cheap" {
		t.Fatalf("expected cheaper price product, got %s", best.ProductID)
	}
}

func TestPick_ZeroPricePerKgRankedLast(t *testing.T) {
	// When one product has PricePerKg=0 and another has a real value,
	// the real value should rank higher (less() treats 0 as "no data" → loses tie)
	products := []Product{
		makeProduct("no-ppk", "mleko", 3.00, 0, true),    // ppk unknown
		makeProduct("has-ppk", "mleko", 9.00, 5.00, true), // ppk known but price is higher
	}
	best, _, ok := Pick(products, "mleko", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// has-ppk should win because ppk=0 is treated as last
	if best.ProductID != "has-ppk" {
		t.Fatalf("expected has-ppk (known pricePerKg), got %s", best.ProductID)
	}
}

func TestPick_AvailableOnlyConsidered(t *testing.T) {
	products := []Product{
		makeProduct("unavailable", "mleko UHT tanie", 1.00, 1.00, false), // would win on price
		makeProduct("available", "mleko UHT", 9.99, 9.99, true),
	}
	best, _, ok := Pick(products, "mleko UHT", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if best.ProductID != "available" {
		t.Fatalf("expected available product, got %s", best.ProductID)
	}
}

func TestPick_TopNAtMostThree(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko A", 1.00, 1.00, true),
		makeProduct("2", "mleko B", 2.00, 2.00, true),
		makeProduct("3", "mleko C", 3.00, 3.00, true),
		makeProduct("4", "mleko D", 4.00, 4.00, true),
		makeProduct("5", "mleko E", 5.00, 5.00, true),
	}
	_, top, ok := Pick(products, "mleko", 0.5)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(top) > 3 {
		t.Fatalf("expected at most 3 results, got %d", len(top))
	}
}

func TestPick_MinScoreBoundary(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko UHT", 3.99, 3.99, true), // score=1.0 for "mleko UHT"
	}

	// exactly at minScore boundary — should succeed
	_, _, ok := Pick(products, "mleko UHT", 1.0)
	if !ok {
		t.Fatal("expected ok=true when score equals minScore")
	}

	// just above maxScore boundary — should fail
	_, _, ok = Pick(products, "mleko UHT", 1.1)
	if ok {
		t.Fatal("expected ok=false when minScore > max possible score")
	}
}

func TestPick_EmptyPhrase(t *testing.T) {
	products := []Product{
		makeProduct("1", "mleko UHT", 3.99, 3.99, true),
	}
	// Score always returns 0 for empty phrase → no candidates
	_, _, ok := Pick(products, "", 0.0)
	if ok {
		t.Fatal("expected ok=false for empty phrase (score=0 filters all candidates)")
	}
}

// ── NormaliseProducts ─────────────────────────────────────────────────────────

func rawEntry(productId string, name string, price, pricePerKg float64, available bool) map[string]any {
	return map[string]any{
		"productId": productId,
		"product": map[string]any{
			"name":        name,
			"brand":       "TestBrand",
			"isAvailable": available,
			"grammage":    "500 g",
			"price": map[string]any{
				"price": price,
			},
			"pricePerUnit": map[string]any{
				"price": pricePerKg,
			},
		},
	}
}

func TestNormaliseProducts_Empty(t *testing.T) {
	result := NormaliseProducts(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result for nil input, got %v", result)
	}
	result = NormaliseProducts([]any{})
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty slice, got %v", result)
	}
}

func TestNormaliseProducts_SkipsNonMapEntries(t *testing.T) {
	raw := []any{
		"not a map",
		42,
		nil,
		rawEntry("1", "mleko", 3.99, 7.98, true),
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 valid product, got %d", len(result))
	}
}

func TestNormaliseProducts_BasicFields(t *testing.T) {
	raw := []any{
		rawEntry("abc123", "Mleko UHT 3.2%", 4.99, 9.98, true),
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product, got %d", len(result))
	}
	p := result[0]
	if p.ProductID != "abc123" {
		t.Errorf("ProductID: expected abc123, got %s", p.ProductID)
	}
	if p.Name != "Mleko UHT 3.2%" {
		t.Errorf("Name: expected 'Mleko UHT 3.2%%', got %s", p.Name)
	}
	if p.Brand != "TestBrand" {
		t.Errorf("Brand: expected TestBrand, got %s", p.Brand)
	}
	if !floatEq(p.Price, 4.99) {
		t.Errorf("Price: expected 4.99, got %v", p.Price)
	}
	if !floatEq(p.PricePerKg, 9.98) {
		t.Errorf("PricePerKg: expected 9.98, got %v", p.PricePerKg)
	}
	if p.Grammage != "500 g" {
		t.Errorf("Grammage: expected '500 g', got %s", p.Grammage)
	}
	if !p.Available {
		t.Errorf("Available: expected true")
	}
	if p.Raw == nil {
		t.Errorf("Raw: expected non-nil")
	}
}

func TestNormaliseProducts_UnavailableProduct(t *testing.T) {
	raw := []any{rawEntry("2", "jogurt", 2.50, 5.00, false)}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product")
	}
	if result[0].Available {
		t.Error("expected Available=false")
	}
}

func TestNormaliseProducts_MissingPriceFields(t *testing.T) {
	// No price / pricePerUnit keys at all
	raw := []any{
		map[string]any{
			"productId": "3",
			"product": map[string]any{
				"name":        "chleb",
				"isAvailable": true,
			},
		},
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product")
	}
	if result[0].Price != 0 || result[0].PricePerKg != 0 {
		t.Errorf("expected zero prices, got price=%v ppk=%v", result[0].Price, result[0].PricePerKg)
	}
}

func TestNormaliseProducts_PricePerKgFallback(t *testing.T) {
	// pricePerUnit missing but pricePerKg key present directly on inner map
	raw := []any{
		map[string]any{
			"productId": "4",
			"product": map[string]any{
				"name":        "masło",
				"isAvailable": true,
				"price":       map[string]any{"price": float64(5.99)},
				"pricePerKg":  float64(23.96),
			},
		},
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product")
	}
	if !floatEq(result[0].PricePerKg, 23.96) {
		t.Errorf("expected pricePerKg=23.96 from fallback, got %v", result[0].PricePerKg)
	}
}

func TestNormaliseProducts_FlatEntryWithoutProductWrapper(t *testing.T) {
	// When there is no "product" sub-map, the entry itself is used as inner map
	raw := []any{
		map[string]any{
			"productId":   "5",
			"name":        "ser gouda",
			"isAvailable": true,
			"price":       map[string]any{"price": float64(8.99)},
		},
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product")
	}
	if result[0].Name != "ser gouda" {
		t.Errorf("expected 'ser gouda', got %s", result[0].Name)
	}
	if !floatEq(result[0].Price, 8.99) {
		t.Errorf("expected price=8.99, got %v", result[0].Price)
	}
}

func TestNormaliseProducts_ProductIdTrimmed(t *testing.T) {
	raw := []any{
		map[string]any{
			"productId": "  99  ",
			"product":   map[string]any{"name": "test"},
		},
	}
	result := NormaliseProducts(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 product")
	}
	if result[0].ProductID != "99" {
		t.Errorf("expected trimmed id '99', got %q", result[0].ProductID)
	}
}

func TestNormaliseProducts_MultipleProducts(t *testing.T) {
	raw := []any{
		rawEntry("10", "mleko", 3.00, 6.00, true),
		rawEntry("11", "jogurt", 2.50, 10.00, false),
		rawEntry("12", "masło", 6.00, 24.00, true),
	}
	result := NormaliseProducts(raw)
	if len(result) != 3 {
		t.Fatalf("expected 3 products, got %d", len(result))
	}
}

// ── sortResults ───────────────────────────────────────────────────────────────

func TestSortResults_ByScore(t *testing.T) {
	rs := []Result{
		{Product: makeProduct("low", "x", 1, 1, true), Score: 0.3},
		{Product: makeProduct("high", "x", 1, 1, true), Score: 1.0},
		{Product: makeProduct("mid", "x", 1, 1, true), Score: 0.5},
	}
	sortResults(rs)
	if rs[0].Product.ProductID != "high" || rs[1].Product.ProductID != "mid" || rs[2].Product.ProductID != "low" {
		t.Fatalf("unexpected order: %v %v %v", rs[0].Product.ProductID, rs[1].Product.ProductID, rs[2].Product.ProductID)
	}
}

func TestSortResults_TieByPricePerKg(t *testing.T) {
	rs := []Result{
		{Product: makeProduct("expensive", "x", 5, 10, true), Score: 1.0},
		{Product: makeProduct("cheap", "x", 3, 4, true), Score: 1.0},
	}
	sortResults(rs)
	if rs[0].Product.ProductID != "cheap" {
		t.Fatalf("expected cheap first, got %s", rs[0].Product.ProductID)
	}
}

func TestSortResults_TieByPrice(t *testing.T) {
	rs := []Result{
		{Product: makeProduct("expensive", "x", 9, 0, true), Score: 1.0},
		{Product: makeProduct("cheap", "x", 3, 0, true), Score: 1.0},
	}
	sortResults(rs)
	if rs[0].Product.ProductID != "cheap" {
		t.Fatalf("expected cheap first, got %s", rs[0].Product.ProductID)
	}
}

func TestSortResults_Empty(t *testing.T) {
	// must not panic
	sortResults(nil)
	sortResults([]Result{})
}

func TestSortResults_SingleElement(t *testing.T) {
	rs := []Result{
		{Product: makeProduct("1", "x", 5, 5, true), Score: 0.8},
	}
	sortResults(rs) // must not panic, element stays
	if rs[0].Product.ProductID != "1" {
		t.Fatal("single element changed")
	}
}
