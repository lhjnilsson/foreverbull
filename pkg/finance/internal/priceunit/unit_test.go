package priceunit

import "testing"

func TestCanonical(t *testing.T) {
	got, err := Canonical("GBP", "GBp")
	if err != nil || got != "GBX" {
		t.Fatalf("GBp: got %q %v", got, err)
	}
	got, err = Canonical("GBP", "gbx")
	if err != nil || got != "GBX" {
		t.Fatalf("gbx: got %q %v", got, err)
	}
	got, err = Canonical("GBP", "gbp")
	if err != nil || got != "GBP" {
		t.Fatalf("gbp: got %q %v", got, err)
	}
	got, err = Canonical("USD", "")
	if err != nil || got != "" {
		t.Fatalf("empty: got %q %v", got, err)
	}
	got, err = Canonical("USD", "USD")
	if err != nil || got != "USD" {
		t.Fatalf("USD: got %q %v", got, err)
	}
	_, err = Canonical("USD", "USX")
	if err == nil || err.Error() != "price unit 'USX' is not a unit of USD" {
		t.Fatalf("USX: %v", err)
	}
	_, err = Canonical("USD", "uSx")
	if err == nil || err.Error() != "price unit 'uSx' is not a unit of USD" {
		t.Fatalf("uSx: %v", err)
	}
	_, err = Canonical("USD", "GBX")
	if err == nil || err.Error() != "price unit 'GBX' is not a unit of USD" {
		t.Fatalf("GBX of USD: %v", err)
	}
	_, err = Canonical("", "USX")
	if err == nil || err.Error() != "price unit 'USX' is not a unit of a currency" {
		t.Fatalf("no currency: %v", err)
	}
}
