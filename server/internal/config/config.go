package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr                      string
	DatabaseURL                   string
	OperatorAccessSecret          string
	OperatorSessionCookie         string
	OperatorAllowedOrigins        []string
	OperatorAllowInsecureLoopback bool
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:              os.Getenv("HTTP_ADDR"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		OperatorAccessSecret:  os.Getenv("OPERATOR_ACCESS_SECRET"),
		OperatorSessionCookie: os.Getenv("OPERATOR_SESSION_COOKIE"),
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = "127.0.0.1:8081"
	}
	if c.OperatorSessionCookie == "" {
		c.OperatorSessionCookie = "phmon_operator"
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR must be a host:port address")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("HTTP_ADDR must use a port between 1 and 65535")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/" {
		return Config{}, errors.New("DATABASE_URL must be a PostgreSQL URL with a host and database name")
	}
	if len(c.OperatorAccessSecret) < 32 || len(c.OperatorAccessSecret) > 1024 || strings.TrimSpace(c.OperatorAccessSecret) != c.OperatorAccessSecret {
		return Config{}, errors.New("OPERATOR_ACCESS_SECRET must contain 32 to 1024 characters")
	}
	for _, raw := range strings.Split(os.Getenv("OPERATOR_ALLOWED_ORIGINS"), ",") {
		if origin := strings.TrimSpace(raw); origin != "" {
			c.OperatorAllowedOrigins = append(c.OperatorAllowedOrigins, origin)
		}
	}
	if len(c.OperatorAllowedOrigins) == 0 {
		return Config{}, errors.New("OPERATOR_ALLOWED_ORIGINS must contain at least one origin")
	}
	if raw := strings.TrimSpace(os.Getenv("OPERATOR_ALLOW_INSECURE_LOOPBACK")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("OPERATOR_ALLOW_INSECURE_LOOPBACK must be true or false")
		}
		c.OperatorAllowInsecureLoopback = value
	}
	return c, nil
}
