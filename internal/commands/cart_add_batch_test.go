package commands

import (
	"testing"
)

func TestParseCartBatchJSON(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		raw := []byte(`[
			{"product_id": "10", "quantity": 2},
			{"productId": "20", "qty": 1}
		]`)
		m, err := parseCartBatchJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		if m["10"] != 2 || m["20"] != 1 {
			t.Fatalf("got %#v", m)
		}
	})

	t.Run("items wrapper", func(t *testing.T) {
		raw := []byte(`{"items":[{"product_id":"5"}]}`)
		m, err := parseCartBatchJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		if m["5"] != 1 {
			t.Fatalf("expected default qty 1, got %#v", m)
		}
	})

	t.Run("merge duplicate ids", func(t *testing.T) {
		raw := []byte(`[
			{"product_id": "1", "quantity": 2},
			{"product_id": "1", "qty": 3}
		]`)
		m, err := parseCartBatchJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		if m["1"] != 5 {
			t.Fatalf("got %#v", m)
		}
	})

	t.Run("reject bad quantity", func(t *testing.T) {
		raw := []byte(`[{"product_id":"1","quantity":0}]`)
		_, err := parseCartBatchJSON(raw)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
