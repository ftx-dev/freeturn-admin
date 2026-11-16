package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Тесты гоняются на SQLite всегда и на PostgreSQL, если задан FTA_TEST_PG_DSN (task test-pg).
func stores(t *testing.T) map[string]*Store {
	t.Helper()
	ctx := context.Background()
	out := map[string]*Store{}
	s, err := Open(ctx, "sqlite", "file:"+filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	out["sqlite"] = s
	if dsn := os.Getenv("FTA_TEST_PG_DSN"); dsn != "" {
		p, err := Open(ctx, "postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		for _, tbl := range []string{"audit", "sessions", "servers", "admins"} {
			p.exec(ctx, "DELETE FROM "+tbl)
		}
		out["postgres"] = p
	}
	t.Cleanup(func() {
		for _, s := range out {
			s.Close()
		}
	})
	return out
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			if err := s.migrate(ctx); err != nil {
				t.Fatal("повторная миграция:", err)
			}
			a, err := s.CreateAdmin(ctx, "root", "hash", "admin")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.CreateAdmin(ctx, "root", "hash", "admin"); err == nil {
				t.Fatal("дубликат логина принят")
			}
			if n, _ := s.CountAdmins(ctx); n != 1 {
				t.Fatal("count", n)
			}
			if got, err := s.AdminByLogin(ctx, "root"); err != nil || got.ID != a.ID {
				t.Fatal(got, err)
			}
			if _, err := s.AdminByLogin(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}

			if err := s.CreateSession(ctx, "h1", a.ID, "csrf", 60, "1.2.3.4", "ua"); err != nil {
				t.Fatal(err)
			}
			if se, err := s.SessionByHash(ctx, "h1"); err != nil || se.CSRF != "csrf" {
				t.Fatal(se, err)
			}
			s.CreateSession(ctx, "old", a.ID, "c", -1, "", "")
			if _, err := s.SessionByHash(ctx, "old"); !errors.Is(err, ErrNotFound) {
				t.Fatal("истёкшая сессия жива")
			}
			s.DeleteSession(ctx, "h1")
			if _, err := s.SessionByHash(ctx, "h1"); !errors.Is(err, ErrNotFound) {
				t.Fatal("сессия не удалена")
			}

			v, err := s.CreateServer(ctx, Server{Name: "nl", Host: "1.2.3.4", SSHPort: 22, SSHUser: "root", AuthType: "password", SSHSecretEnc: "x"})
			if err != nil {
				t.Fatal(err)
			}
			v.Name = "Нидерланды"
			if err := s.UpdateServer(ctx, v); err != nil {
				t.Fatal(err)
			}
			list, _ := s.ListServers(ctx)
			if len(list) != 1 || list[0].Name != "Нидерланды" || list[0].SSHSecretEnc != "x" {
				t.Fatal(list)
			}
			if err := s.DeleteServer(ctx, v.ID); err != nil {
				t.Fatal(err)
			}
			if err := s.DeleteServer(ctx, v.ID); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}

			s.Audit(ctx, a.ID, "", "login", "", "ok")
			if e, _ := s.ListAudit(ctx, 10); len(e) != 1 || e[0].Login != "root" || e[0].Details != "{}" {
				t.Fatal(e)
			}
		})
	}
}
