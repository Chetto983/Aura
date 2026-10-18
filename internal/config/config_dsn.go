package config

import (
	"net"
	"net/url"
)

// composeDSN returns "" when password is empty so callers can detect an
// unconfigured DSN and fail-fast instead of dialing with a blank credential.
func composeDSN(role, password, host, port, dbname, sslmode string) string {
	if password == "" {
		return ""
	}
	u := url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(role, password),
		Host:    net.JoinHostPort(host, port),
		Path:    "/" + dbname,
		RawPath: "/" + url.PathEscape(dbname),
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}
