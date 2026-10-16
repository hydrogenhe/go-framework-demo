package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrderExpirationIntervalConfiguration(t *testing.T) {
	cases := []struct {
		name, extra string
		want        int
		invalid     bool
	}{
		{"default", "", 3, false},
		{"configured", "order_expiration:\n  interval_seconds: 7\n", 7, false},
		{"negative", "order_expiration:\n  interval_seconds: -1\n", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			confDir := filepath.Join(directory, "conf")
			if err := os.Mkdir(confDir, 0700); err != nil {
				t.Fatal(err)
			}
			body := "http:\n  address: ':8080'\nfrontend:\n  api_base_url: http://localhost:8080\n  address: ':5500'\n" + tc.extra
			if err := os.WriteFile(filepath.Join(confDir, "app.yaml"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(directory)
			config, err := LoadAppConfig()
			if (err != nil) != tc.invalid {
				t.Fatalf("err=%v", err)
			}
			if !tc.invalid && config.OrderExpiration.IntervalSeconds != tc.want {
				t.Errorf("interval=%d want=%d", config.OrderExpiration.IntervalSeconds, tc.want)
			}
		})
	}
}
