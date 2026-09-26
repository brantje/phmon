package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func Load() (Config, error) {
	c := Config{HTTPAddr: os.Getenv("HTTP_ADDR"), DatabaseURL: os.Getenv("DATABASE_URL")}
	if c.HTTPAddr == "" {
		c.HTTPAddr = "127.0.0.1:8081"
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
	return c, nil
}
