// Package config читает настройки админки из переменных окружения FTA_*.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Listen        string
	DBDriver      string // sqlite | postgres
	DBDSN         string
	DataDir       string
	TLSCert       string
	TLSKey        string
	AdminLogin    string
	AdminPassword string
	CoreVersion   string
	MasterKey     []byte
	SecureCookies bool
	TrustProxy    bool
}

// shortcut: только env, без config.yaml из плана; добавить yaml, когда понадобится больше пары ключей.
func Load() (Config, error) {
	c := Config{
		Listen:        env("FTA_LISTEN", ":8080"),
		DBDriver:      env("FTA_DB_DRIVER", "sqlite"),
		DataDir:       env("FTA_DATA_DIR", "data"),
		TLSCert:       os.Getenv("FTA_TLS_CERT"),
		TLSKey:        os.Getenv("FTA_TLS_KEY"),
		AdminLogin:    os.Getenv("FTA_ADMIN_LOGIN"),
		AdminPassword: os.Getenv("FTA_ADMIN_PASSWORD"),
		CoreVersion:   env("FTA_CORE_VERSION", "latest"),
		SecureCookies: env("FTA_COOKIE_SECURE", "true") != "false",
		TrustProxy:    os.Getenv("FTA_TRUST_PROXY") == "true",
	}
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return c, fmt.Errorf("FTA_DB_DRIVER: sqlite или postgres, а не %q", c.DBDriver)
	}
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return c, err
	}
	c.DBDSN = os.Getenv("FTA_DB_DSN")
	if c.DBDSN == "" {
		if c.DBDriver == "postgres" {
			return c, errors.New("FTA_DB_DSN обязателен для postgres")
		}
		c.DBDSN = "file:" + filepath.Join(c.DataDir, "freeturn-admin.db")
	}
	key, err := masterKey(c.DataDir)
	if err != nil {
		return c, err
	}
	c.MasterKey = key
	return c, nil
}

// masterKey: FTA_MASTER_KEY, затем FTA_MASTER_KEY_FILE, иначе ключ создаётся в data/master.key (0600).
func masterKey(dataDir string) ([]byte, error) {
	raw := os.Getenv("FTA_MASTER_KEY")
	if raw == "" {
		path := os.Getenv("FTA_MASTER_KEY_FILE")
		if path == "" {
			path = filepath.Join(dataDir, "master.key")
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				k := make([]byte, 32)
				if _, err := rand.Read(k); err != nil {
					return nil, err
				}
				if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(k)+"\n"), 0o600); err != nil {
					return nil, err
				}
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("мастер-ключ: %w", err)
		}
		raw = string(b)
	}
	k, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(k) != 32 {
		return nil, errors.New("мастер-ключ: нужно 32 байта в base64")
	}
	return k, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
