// Package dbtarget stops an operator command from writing to a database its
// environment does not name.
//
// The commands read .env like the server does, and .env is edited by hand. On
// 2026-10-12 it said APP_ENV=local while DB_DSN pointed at production, and the
// only thing between that and the demo admin (whose password is printed in a
// public guide) landing in production was that nobody ran `make seed`. The seed
// refused production by name, but the name was wrong. This compares the name
// with where the DSN actually goes.
package dbtarget

import (
	"fmt"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
)

// deployedEnvironments are the APP_ENV values that say "a remote database is
// expected here".
var deployedEnvironments = map[string]bool{"production": true, "staging": true}

// Check returns an error when environment says local but dsn reaches a remote
// host. allowRemote is the explicit, per-run override.
func Check(environment, dsn string, allowRemote bool) error {
	if allowRemote || deployedEnvironments[strings.ToLower(strings.TrimSpace(environment))] {
		return nil
	}
	hosts, err := Hosts(dsn)
	if err != nil {
		return fmt.Errorf("read the database host from DB_DSN: %w", err)
	}
	for _, host := range hosts {
		if !IsLocalHost(host) {
			return fmt.Errorf(
				"DB_DSN points at %s, a remote host, but APP_ENV is %q: if this is production, "+
					"set APP_ENV=production; if it is not, point DB_DSN at your local database "+
					"(or pass -allow-remote-db to say you mean it)",
				host, environment)
		}
	}
	return nil
}

// Hosts lists every host a DSN may connect to, fallbacks included.
func Hosts(dsn string) ([]string, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	hosts := []string{cfg.Host}
	for _, fallback := range cfg.Fallbacks {
		hosts = append(hosts, fallback.Host)
	}
	return hosts, nil
}

// IsLocalHost reports whether host is this machine or a container beside it:
// loopback, a unix socket, *.localhost, Docker's host alias, or a single-label
// name such as a compose service ("postgres").
func IsLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "" || strings.HasPrefix(host, "/"):
		return true
	case host == "localhost" || strings.HasSuffix(host, ".localhost"):
		return true
	case host == "host.docker.internal":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return !strings.Contains(host, ".")
}
