package shared

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// ExtractNutritionBlock
// ---------------------------------------------------------------------------

func TestExtractNutritionBlock_Nil(t *testing.T) {
	if got := ExtractNutritionBlock(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractNutritionBlock_NoMatch(t *testing.T) {
	m := map[string]any{"brand": "Acme", "price": 9.99}
	if got := ExtractNutritionBlock(m); got != nil {
		t.Errorf("expected nil for map with no nutrition key, got %v", got)
	}
}

func TestExtractNutritionBlock_TopLevelNutrition(t *testing.T) {
	want := map[string]any{"calories": "200kcal"}
	m := map[string]any{"nutritionInfo": want}
	got := ExtractNutritionBlock(m)
	if got == nil {
		t.Fatal("expected nutrition block, got nil")
	}
	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if gotMap["calories"] != "200kcal" {
		t.Errorf("unexpected value: %v", gotMap)
	}
}

func TestExtractNutritionBlock_KeyContainsNutri(t *testing.T) {
	want := "per 100g data"
	m := map[string]any{"nutriValues": want}
	got := ExtractNutritionBlock(m)
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_KeyContainsOdzyw(t *testing.T) {
	want := "wartosci odzywcze"
	m := map[string]any{"odzywcze": want}
	got := ExtractNutritionBlock(m)
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_CaseInsensitive(t *testing.T) {
	want := "data"
	m := map[string]any{"NUTRITIONDATA": want}
	got := ExtractNutritionBlock(m)
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_NestedMap(t *testing.T) {
	want := "nested nutrition"
	m := map[string]any{
		"product": map[string]any{
			"details": map[string]any{
				"nutritionFacts": want,
			},
		},
	}
	got := ExtractNutritionBlock(m)
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_InSlice(t *testing.T) {
	want := "slice nutrition"
	payload := []any{
		map[string]any{"name": "unrelated"},
		map[string]any{"nutritionBlock": want},
	}
	got := ExtractNutritionBlock(payload)
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_RawMessage(t *testing.T) {
	want := "raw nutrition"
	inner := map[string]any{"nutrition": want}
	b, _ := json.Marshal(inner)
	got := ExtractNutritionBlock(json.RawMessage(b))
	if got != want {
		t.Errorf("expected %q, got %v", want, got)
	}
}

func TestExtractNutritionBlock_RawMessageInvalid(t *testing.T) {
	got := ExtractNutritionBlock(json.RawMessage([]byte("not-json")))
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}

func TestExtractNutritionBlock_EmptyMap(t *testing.T) {
	got := ExtractNutritionBlock(map[string]any{})
	if got != nil {
		t.Errorf("expected nil for empty map, got %v", got)
	}
}

func TestExtractNutritionBlock_EmptySlice(t *testing.T) {
	got := ExtractNutritionBlock([]any{})
	if got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// ProductNameFromMap
// ---------------------------------------------------------------------------

func TestProductNameFromMap_NameKey(t *testing.T) {
	m := map[string]any{"name": "Mleko UHT"}
	if got := ProductNameFromMap(m); got != "Mleko UHT" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestProductNameFromMap_DisplayNameFallback(t *testing.T) {
	m := map[string]any{"displayName": "Masło Extra"}
	if got := ProductNameFromMap(m); got != "Masło Extra" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestProductNameFromMap_PriorityOrder(t *testing.T) {
	m := map[string]any{
		"name":         "First",
		"displayName":  "Second",
		"productName":  "Third",
	}
	if got := ProductNameFromMap(m); got != "First" {
		t.Errorf("expected 'First' (highest priority), got %q", got)
	}
}

func TestProductNameFromMap_LocalizedName(t *testing.T) {
	m := map[string]any{
		"name": map[string]any{"pl": "Chleb razowy", "en": "Wholemeal bread"},
	}
	if got := ProductNameFromMap(m); got != "Chleb razowy" {
		t.Errorf("expected Polish name, got %q", got)
	}
}

func TestProductNameFromMap_SkipsNilValues(t *testing.T) {
	m := map[string]any{
		"name":        nil,
		"displayName": "Fallback",
	}
	if got := ProductNameFromMap(m); got != "Fallback" {
		t.Errorf("expected 'Fallback', got %q", got)
	}
}

func TestProductNameFromMap_EmptyMap(t *testing.T) {
	if got := ProductNameFromMap(map[string]any{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestProductNameFromMap_AllKeysEmpty(t *testing.T) {
	m := map[string]any{
		"name":        "",
		"displayName": "",
	}
	if got := ProductNameFromMap(m); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestProductNameFromMap_ProductNameKey(t *testing.T) {
	m := map[string]any{"productName": "Ser żółty"}
	if got := ProductNameFromMap(m); got != "Ser żółty" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestProductNameFromMap_TitleKey(t *testing.T) {
	m := map[string]any{"title": "Jogurt naturalny"}
	if got := ProductNameFromMap(m); got != "Jogurt naturalny" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestProductNameFromMap_ProductTitleKey(t *testing.T) {
	m := map[string]any{"productTitle": "Woda mineralna"}
	if got := ProductNameFromMap(m); got != "Woda mineralna" {
		t.Errorf("unexpected: %q", got)
	}
}

// ---------------------------------------------------------------------------
// LocalizedString
// ---------------------------------------------------------------------------

func TestLocalizedString_PlainString(t *testing.T) {
	if got := LocalizedString("hello"); got != "hello" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestLocalizedString_PlainStringTrimmed(t *testing.T) {
	if got := LocalizedString("  trimmed  "); got != "trimmed" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestLocalizedString_EmptyString(t *testing.T) {
	if got := LocalizedString(""); got != "" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestLocalizedString_PrefersPLOverEN(t *testing.T) {
	m := map[string]any{"pl": "Jabłko", "en": "Apple"}
	if got := LocalizedString(m); got != "Jabłko" {
		t.Errorf("expected 'Jabłko' (pl preferred), got %q", got)
	}
}

func TestLocalizedString_FallsBackToEN(t *testing.T) {
	m := map[string]any{"en": "Apple"}
	if got := LocalizedString(m); got != "Apple" {
		t.Errorf("expected 'Apple', got %q", got)
	}
}

func TestLocalizedString_PLEmptyFallsBackToEN(t *testing.T) {
	m := map[string]any{"pl": "", "en": "Apple"}
	if got := LocalizedString(m); got != "Apple" {
		t.Errorf("expected 'Apple' when pl is empty, got %q", got)
	}
}

func TestLocalizedString_FallsBackToAnyKey(t *testing.T) {
	m := map[string]any{"de": "Apfel"}
	got := LocalizedString(m)
	if got != "Apfel" {
		t.Errorf("expected fallback to any locale value, got %q", got)
	}
}

func TestLocalizedString_EmptyMap(t *testing.T) {
	if got := LocalizedString(map[string]any{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestLocalizedString_MapWithNilPL(t *testing.T) {
	m := map[string]any{"pl": nil, "en": "Apple"}
	if got := LocalizedString(m); got != "Apple" {
		t.Errorf("expected 'Apple' when pl is nil, got %q", got)
	}
}

func TestLocalizedString_NonStringType_Int(t *testing.T) {
	got := LocalizedString(42)
	if got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}

func TestLocalizedString_NonStringType_MapPrefix(t *testing.T) {
	// A raw Go map value printed by fmt.Sprint starts with "map[" — should return ""
	type dummy struct{}
	// We need something that fmt.Sprint produces starting with "map["
	// Use a nested map[int]int which is not map[string]any
	val := map[int]int{1: 2}
	got := LocalizedString(val)
	if got != "" {
		t.Errorf("expected empty string for map-like Sprint output, got %q", got)
	}
}

func TestLocalizedString_Nil(t *testing.T) {
	// nil goes through default branch: fmt.Sprint(nil) == "<nil>", TrimSpace -> "<nil>", no "map[" prefix
	got := LocalizedString(nil)
	if got != "<nil>" {
		t.Errorf("expected '<nil>', got %q", got)
	}
}

func TestLocalizedString_NestedLocaleMap(t *testing.T) {
	// Map without pl/en but with a nested string value under another locale
	m := map[string]any{"fr": "Pomme"}
	got := LocalizedString(m)
	if got != "Pomme" {
		t.Errorf("expected 'Pomme', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// MoneyString
// ---------------------------------------------------------------------------

func TestMoneyString_Nil(t *testing.T) {
	if got := MoneyString(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMoneyString_Int(t *testing.T) {
	if got := MoneyString(5); got != "5.00" {
		t.Errorf("expected '5.00', got %q", got)
	}
}

func TestMoneyString_Int32(t *testing.T) {
	if got := MoneyString(int32(10)); got != "10.00" {
		t.Errorf("expected '10.00', got %q", got)
	}
}

func TestMoneyString_Int64(t *testing.T) {
	if got := MoneyString(int64(20)); got != "20.00" {
		t.Errorf("expected '20.00', got %q", got)
	}
}

func TestMoneyString_Float32(t *testing.T) {
	got := MoneyString(float32(3.5))
	// float32 precision may give "3.50"
	if got != "3.50" {
		t.Errorf("expected '3.50', got %q", got)
	}
}

func TestMoneyString_Float64(t *testing.T) {
	if got := MoneyString(12.34); got != "12.34" {
		t.Errorf("expected '12.34', got %q", got)
	}
}

func TestMoneyString_StringPassthrough(t *testing.T) {
	if got := MoneyString("9.99"); got != "9.99" {
		t.Errorf("expected '9.99', got %q", got)
	}
}

func TestMoneyString_StringTrimmed(t *testing.T) {
	if got := MoneyString("  4.50  "); got != "4.50" {
		t.Errorf("expected '4.50', got %q", got)
	}
}

func TestMoneyString_EmptyString(t *testing.T) {
	if got := MoneyString(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMoneyString_StringWhitespaceOnly(t *testing.T) {
	if got := MoneyString("   "); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMoneyString_MapPriceKey(t *testing.T) {
	m := map[string]any{"price": 7.49}
	if got := MoneyString(m); got != "7.49" {
		t.Errorf("expected '7.49', got %q", got)
	}
}

func TestMoneyString_MapGrossKey(t *testing.T) {
	m := map[string]any{"gross": 3.99}
	if got := MoneyString(m); got != "3.99" {
		t.Errorf("expected '3.99', got %q", got)
	}
}

func TestMoneyString_MapAmountKey(t *testing.T) {
	m := map[string]any{"amount": 1.23}
	if got := MoneyString(m); got != "1.23" {
		t.Errorf("expected '1.23', got %q", got)
	}
}

func TestMoneyString_MapValueKey(t *testing.T) {
	m := map[string]any{"value": 5.55}
	if got := MoneyString(m); got != "5.55" {
		t.Errorf("expected '5.55', got %q", got)
	}
}

func TestMoneyString_MapTotalKey(t *testing.T) {
	m := map[string]any{"_total": 19.99}
	if got := MoneyString(m); got != "19.99" {
		t.Errorf("expected '19.99', got %q", got)
	}
}

func TestMoneyString_MapFRSKey(t *testing.T) {
	m := map[string]any{"FRS": 2.5}
	if got := MoneyString(m); got != "2.50" {
		t.Errorf("expected '2.50', got %q", got)
	}
}

func TestMoneyString_MapPriorityPriceOverGross(t *testing.T) {
	m := map[string]any{"price": 10.0, "gross": 12.0}
	if got := MoneyString(m); got != "10.00" {
		t.Errorf("expected 'price' key to win, got %q", got)
	}
}

func TestMoneyString_MapNoMatchingKey(t *testing.T) {
	m := map[string]any{"unknown": 99.9}
	if got := MoneyString(m); got != "" {
		t.Errorf("expected empty string for unknown map key, got %q", got)
	}
}

func TestMoneyString_EmptyMap(t *testing.T) {
	if got := MoneyString(map[string]any{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMoneyString_MapWithNilPrice(t *testing.T) {
	m := map[string]any{"price": nil, "gross": 8.0}
	if got := MoneyString(m); got != "8.00" {
		t.Errorf("expected fallback to gross when price is nil, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// FormatMoneyValue
// ---------------------------------------------------------------------------

func TestFormatMoneyValue_Nil(t *testing.T) {
	if got := FormatMoneyValue(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatMoneyValue_Float64(t *testing.T) {
	if got := FormatMoneyValue(float64(6.78)); got != "6.78" {
		t.Errorf("expected '6.78', got %q", got)
	}
}

func TestFormatMoneyValue_Float32(t *testing.T) {
	got := FormatMoneyValue(float32(1.5))
	if got != "1.50" {
		t.Errorf("expected '1.50', got %q", got)
	}
}

func TestFormatMoneyValue_Int(t *testing.T) {
	// FormatMoneyValue uses %d for int (no decimal places — unlike MoneyString)
	if got := FormatMoneyValue(7); got != "7" {
		t.Errorf("expected '7', got %q", got)
	}
}

func TestFormatMoneyValue_Int64(t *testing.T) {
	if got := FormatMoneyValue(int64(42)); got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}

func TestFormatMoneyValue_String(t *testing.T) {
	if got := FormatMoneyValue("  14.99  "); got != "14.99" {
		t.Errorf("expected '14.99', got %q", got)
	}
}

func TestFormatMoneyValue_EmptyString(t *testing.T) {
	if got := FormatMoneyValue(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatMoneyValue_MapPriceKey(t *testing.T) {
	m := map[string]any{"price": 3.14}
	if got := FormatMoneyValue(m); got != "3.14" {
		t.Errorf("expected '3.14', got %q", got)
	}
}

func TestFormatMoneyValue_MapGrossKey(t *testing.T) {
	m := map[string]any{"gross": 9.0}
	if got := FormatMoneyValue(m); got != "9.00" {
		t.Errorf("expected '9.00', got %q", got)
	}
}

func TestFormatMoneyValue_MapAmountKey(t *testing.T) {
	m := map[string]any{"amount": 0.5}
	if got := FormatMoneyValue(m); got != "0.50" {
		t.Errorf("expected '0.50', got %q", got)
	}
}

func TestFormatMoneyValue_MapValueKey(t *testing.T) {
	m := map[string]any{"value": 100.0}
	if got := FormatMoneyValue(m); got != "100.00" {
		t.Errorf("expected '100.00', got %q", got)
	}
}

func TestFormatMoneyValue_MapNoTotalOrFRS(t *testing.T) {
	// FormatMoneyValue does NOT handle "_total" or "FRS" keys
	m := map[string]any{"_total": 50.0}
	if got := FormatMoneyValue(m); got != "" {
		t.Errorf("expected empty string (no _total support), got %q", got)
	}
}

func TestFormatMoneyValue_MapPricePriorityOverGross(t *testing.T) {
	m := map[string]any{"price": 5.0, "gross": 6.0}
	if got := FormatMoneyValue(m); got != "5.00" {
		t.Errorf("expected price to take priority, got %q", got)
	}
}

func TestFormatMoneyValue_EmptyMap(t *testing.T) {
	if got := FormatMoneyValue(map[string]any{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatMoneyValue_MapWithNilPrice(t *testing.T) {
	m := map[string]any{"price": nil, "gross": 7.77}
	if got := FormatMoneyValue(m); got != "7.77" {
		t.Errorf("expected fallback to gross, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TruncateText
// ---------------------------------------------------------------------------

func TestTruncateText_ShortString(t *testing.T) {
	if got := TruncateText("hello", 10); got != "hello" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestTruncateText_ExactLength(t *testing.T) {
	if got := TruncateText("hello", 5); got != "hello" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestTruncateText_Truncated(t *testing.T) {
	if got := TruncateText("hello world", 8); got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
}

func TestTruncateText_MaxLessThanOrEqualTo3(t *testing.T) {
	// max <= 3: no ellipsis, just cut
	if got := TruncateText("hello", 3); got != "hel" {
		t.Errorf("expected 'hel', got %q", got)
	}
}

func TestTruncateText_Max1(t *testing.T) {
	if got := TruncateText("hello", 1); got != "h" {
		t.Errorf("expected 'h', got %q", got)
	}
}

func TestTruncateText_Max0(t *testing.T) {
	if got := TruncateText("hello", 0); got != "" {
		t.Errorf("expected '', got %q", got)
	}
}

func TestTruncateText_EmptyString(t *testing.T) {
	if got := TruncateText("", 10); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateText_LeadingTrailingSpacesTrimmed(t *testing.T) {
	// TruncateText trims whitespace before truncating
	if got := TruncateText("  hi  ", 10); got != "hi" {
		t.Errorf("expected 'hi', got %q", got)
	}
}

func TestTruncateText_WhitespaceOnlyString(t *testing.T) {
	if got := TruncateText("     ", 10); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateText_UnicodeRunes(t *testing.T) {
	// "żółty ser" is 9 runes; truncate to 7 -> "żółty..." (4 runes + "...")
	s := "żółty ser"
	got := TruncateText(s, 7)
	if got != "żółt..." {
		t.Errorf("expected 'żółt...', got %q", got)
	}
}

func TestTruncateText_UnicodeExactLength(t *testing.T) {
	s := "żółty"
	got := TruncateText(s, 5)
	if got != "żółty" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestTruncateText_EllipsisIsThreeChars(t *testing.T) {
	// max=4: result should be 1 rune + "..." = 4 chars total
	got := TruncateText("abcdef", 4)
	if got != "a..." {
		t.Errorf("expected 'a...', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// StringFieldFromMap
// ---------------------------------------------------------------------------

func TestStringFieldFromMap_SingleKeyFound(t *testing.T) {
	m := map[string]any{"brand": "Łaciate"}
	if got := StringFieldFromMap(m, "brand"); got != "Łaciate" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestStringFieldFromMap_FirstKeyWins(t *testing.T) {
	m := map[string]any{"a": "first", "b": "second"}
	if got := StringFieldFromMap(m, "a", "b"); got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
}

func TestStringFieldFromMap_FallsBackToSecondKey(t *testing.T) {
	m := map[string]any{"a": "", "b": "second"}
	if got := StringFieldFromMap(m, "a", "b"); got != "second" {
		t.Errorf("expected 'second', got %q", got)
	}
}

func TestStringFieldFromMap_SkipsNilValue(t *testing.T) {
	m := map[string]any{"a": nil, "b": "found"}
	if got := StringFieldFromMap(m, "a", "b"); got != "found" {
		t.Errorf("expected 'found', got %q", got)
	}
}

func TestStringFieldFromMap_MissingKey(t *testing.T) {
	m := map[string]any{"other": "value"}
	if got := StringFieldFromMap(m, "missing"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStringFieldFromMap_EmptyMap(t *testing.T) {
	if got := StringFieldFromMap(map[string]any{}, "key"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStringFieldFromMap_NoKeys(t *testing.T) {
	m := map[string]any{"a": "value"}
	if got := StringFieldFromMap(m); got != "" {
		t.Errorf("expected empty string when no keys passed, got %q", got)
	}
}

func TestStringFieldFromMap_NonStringValueConverted(t *testing.T) {
	m := map[string]any{"count": 42}
	got := StringFieldFromMap(m, "count")
	if got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}

func TestStringFieldFromMap_NonStringValueFloat(t *testing.T) {
	m := map[string]any{"price": 9.99}
	got := StringFieldFromMap(m, "price")
	if got != "9.99" {
		t.Errorf("expected '9.99', got %q", got)
	}
}

func TestStringFieldFromMap_NonStringEmptyAfterTrim(t *testing.T) {
	// fmt.Sprint of a bool false is "false", not empty — it should be returned
	m := map[string]any{"flag": false}
	got := StringFieldFromMap(m, "flag")
	if got != "false" {
		t.Errorf("expected 'false', got %q", got)
	}
}

func TestStringFieldFromMap_AllKeysMissing(t *testing.T) {
	m := map[string]any{"x": "value"}
	if got := StringFieldFromMap(m, "a", "b", "c"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStringFieldFromMap_AllKeysEmpty(t *testing.T) {
	m := map[string]any{"a": "", "b": "", "c": ""}
	if got := StringFieldFromMap(m, "a", "b", "c"); got != "" {
		t.Errorf("expected empty string when all values are empty, got %q", got)
	}
}
