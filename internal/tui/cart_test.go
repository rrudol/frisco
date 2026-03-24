package tui

import (
	"testing"

	"github.com/rrudol/frisco/internal/shared"
)

func TestLineFromMap_UsesNestedProductFallback(t *testing.T) {
	line := lineFromMap(map[string]any{
		"quantity": 2,
		"product": map[string]any{
			"id": "40046",
			"name": map[string]any{
				"en": "Butter",
				"pl": "Maslo extra",
			},
			"price": map[string]any{
				"price": 7.49,
			},
		},
	})

	if line.productID != "40046" {
		t.Fatalf("expected productID 40046, got %q", line.productID)
	}
	if line.name != "Butter" {
		t.Fatalf("expected name Butter, got %q", line.name)
	}
	if line.unitPrice != "7.49" {
		t.Fatalf("expected unitPrice 7.49, got %q", line.unitPrice)
	}
}

func TestProductNameFromMap_LocalizedNameObject(t *testing.T) {
	got := shared.ProductNameFromMap(map[string]any{
		"name": map[string]any{
			"en": "Mountain oat flakes",
			"pl": "Platki owsiane gorskie",
		},
	})
	if got != "Mountain oat flakes" {
		t.Fatalf("expected localized EN name, got %q", got)
	}
}

func TestFormatUnitPrice_PriceObjectWithPriceField(t *testing.T) {
	got := formatUnitPrice(map[string]any{
		"price": map[string]any{
			"price": 4.59,
		},
	})
	if got != "4.59" {
		t.Fatalf("expected 4.59, got %q", got)
	}
}

func TestParseProductDetailsPayload_ExtractsAllowedProducts(t *testing.T) {
	payload := map[string]any{
		"products": []any{
			map[string]any{
				"productId":   "134932",
				"displayName": "Mleko 2%",
				"unitPrice": map[string]any{
					"gross": 5.39,
				},
			},
			map[string]any{
				"id":   "154002",
				"name": "Chleb razowy",
				"price": map[string]any{
					"amount": "8.99",
				},
			},
			map[string]any{
				"productId":   "999999",
				"displayName": "Ignore me",
				"unitPrice":   1.11,
			},
		},
	}

	got := parseProductDetailsPayload(payload, []string{"134932", "154002"})
	if len(got) != 2 {
		t.Fatalf("expected 2 products, got %d", len(got))
	}

	if got["134932"].name != "Mleko 2%" || got["134932"].unitPrice != "5.39" {
		t.Fatalf("unexpected 134932 details: %+v", got["134932"])
	}
	if got["154002"].name != "Chleb razowy" || got["154002"].unitPrice != "8.99" {
		t.Fatalf("unexpected 154002 details: %+v", got["154002"])
	}
}

func TestMissingDetailsProductIDs_DeduplicatesAndSkipsCompleteRows(t *testing.T) {
	lines := []cartLine{
		{productID: "134932", quantity: 1},
		{productID: "154002", quantity: 1, name: "Has name only"},
		{productID: "154002", quantity: 2},
		{productID: "291", quantity: 1, name: "Done", unitPrice: "10.00"},
		{productID: "", quantity: 1},
	}

	got := missingDetailsProductIDs(lines)
	if len(got) != 2 {
		t.Fatalf("expected 2 ids, got %d (%v)", len(got), got)
	}
	if got[0] != "134932" || got[1] != "154002" {
		t.Fatalf("unexpected ids order/content: %v", got)
	}
}
