package refreshapi

import "testing"

func TestMaskCustomerCode(t *testing.T) {
	tests := map[string]string{
		"8291722A":   "****722A",
		" abcd1234 ": "****1234",
		"1234":       "****",
		"12":         "**",
		"":           "",
	}
	for input, want := range tests {
		if got := MaskCustomerCode(input); got != want {
			t.Errorf("MaskCustomerCode(%q) = %q, want %q", input, got, want)
		}
	}
}
