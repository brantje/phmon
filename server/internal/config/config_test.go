package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, addr, database string
		valid                bool
	}{
		{"defaults", "", "postgres://user:secret@localhost/phmon", true},
		{"explicit", "0.0.0.0:8081", "postgresql://localhost/phmon", true},
		{"missing database", "", "", false},
		{"wrong scheme", "", "https://localhost/phmon", false},
		{"missing host", "", "postgres:///phmon", false},
		{"missing database name", "", "postgres://localhost/", false},
		{"invalid address", "localhost", "postgres://localhost/phmon", false},
		{"invalid port", "localhost:0", "postgres://localhost/phmon", false},
		{"invalid URL", "", "postgres://user:secret%@localhost/phmon", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", tc.addr)
			t.Setenv("DATABASE_URL", tc.database)
			t.Setenv("OPERATOR_ACCESS_SECRET", "0123456789abcdef0123456789abcdef")
			t.Setenv("OPERATOR_ALLOWED_ORIGINS", "http://127.0.0.1:3005")
			t.Setenv("OPERATOR_ALLOW_INSECURE_HTTP", "true")
			c, err := Load()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret@") {
				t.Fatal("credentials exposed")
			}
			if tc.valid && tc.addr == "" && c.HTTPAddr != "127.0.0.1:8081" {
				t.Fatal("unsafe default address")
			}
		})
	}
}

func TestLoadRequiresOperatorConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/phmon")
	t.Setenv("OPERATOR_ALLOWED_ORIGINS", "http://127.0.0.1:3005")
	t.Setenv("OPERATOR_ALLOW_INSECURE_HTTP", "true")
	if _, err := Load(); err == nil {
		t.Fatal("missing operator secret accepted")
	}
	t.Setenv("OPERATOR_ACCESS_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OPERATOR_ALLOWED_ORIGINS", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing operator origins accepted")
	}
}
