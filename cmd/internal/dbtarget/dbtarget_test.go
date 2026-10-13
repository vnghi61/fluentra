package dbtarget

import (
	"fmt"
	"strings"
	"testing"
)

// remoteDSN carries a placeholder password so the test can prove the error
// never repeats it; built at run time so it does not read as a credential.
var remoteDSN = fmt.Sprintf(
	"postgresql://user:%s@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres?sslmode=require", "pw")

func TestIsLocalHost(t *testing.T) {
	cases := map[string]bool{
		"localhost":                 true,
		"127.0.0.1":                 true,
		"::1":                       true,
		"app.localhost":             true,
		"host.docker.internal":      true,
		"postgres":                  true,
		"fluentra-postgres":         true,
		"/var/run/postgresql":       true,
		"db.example.com":            false,
		"10.0.0.5":                  false,
		"aws-0.pooler.supabase.com": false,
	}
	for host, want := range cases {
		if got := IsLocalHost(host); got != want {
			t.Errorf("IsLocalHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// The mismatch that happened: APP_ENV=local, DSN at production.
func TestCheck_RefusesARemoteDatabaseUnderALocalEnvironment(t *testing.T) {
	err := Check("local", remoteDSN, false)
	if err == nil {
		t.Fatal("Check accepted a remote DSN with APP_ENV=local")
	}
	if !strings.Contains(err.Error(), "aws-0-ap-southeast-1.pooler.supabase.com") {
		t.Fatalf("error does not name the host: %v", err)
	}
	if strings.Contains(err.Error(), "pw@") {
		t.Fatalf("error leaks the DSN's password: %v", err)
	}
}

func TestCheck_Allows(t *testing.T) {
	cases := []struct {
		name, env, dsn string
		allowRemote    bool
	}{
		{"local database", "local", "postgres://u:p@localhost:5434/fluentra", false},
		{"compose service", "ci", "postgres://u:p@postgres:5432/fluentra", false},
		{"production names itself", "production", remoteDSN, false},
		{"staging names itself", "Staging", remoteDSN, false},
		{"explicit override", "local", remoteDSN, true},
	}
	for _, tc := range cases {
		if err := Check(tc.env, tc.dsn, tc.allowRemote); err != nil {
			t.Errorf("%s: Check refused: %v", tc.name, err)
		}
	}
}

func TestCheck_ChecksFallbackHosts(t *testing.T) {
	dsn := "host=localhost,db.example.com user=u password=p dbname=fluentra"
	if err := Check("local", dsn, false); err == nil {
		t.Fatal("Check accepted a DSN whose fallback host is remote")
	}
}
