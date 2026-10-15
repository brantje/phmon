package chat

import "testing"

func TestClassifyGlobalOfferRequiresExplicitPrefix(t *testing.T) {
	cases := map[string]string{
		"WTB elixir":        "WTB",
		"wts: +7 staff":     "WTS",
		" WTT Blade":        "WTT",
		"WTS":               "WTS",
		"WTS-grade sword":   "WTS",
		"WTB1234":           "unclassified",
		"selling silk":      "unclassified",
		"[WTS] Sword":       "unclassified",
		"Any WTB?":          "unclassified",
		"WTX unsupported":   "unclassified",
		"":                  "unclassified",
	}
	for input, want := range cases {
		if got := GlobalOfferType(input); got != want {
			t.Errorf("GlobalOfferType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOfferFiltersFailClosed(t *testing.T) {
	valid := OfferFilter{Server: "SRO", Limit: 30, Kind: "WTB"}
	if !validOfferFilter(valid) {
		t.Fatal("valid filter rejected")
	}
	for _, value := range []string{"WTS; DROP TABLE chat_messages", "WTA", "unknown"} {
		invalid := valid
		invalid.Kind = value
		if validOfferFilter(invalid) {
			t.Fatalf("accepted unknown offer classification %q", value)
		}
	}
	invalid := valid
	invalid.Limit = 101
	if validOfferFilter(invalid) {
		t.Fatal("accepted unbounded offer page")
	}
}
