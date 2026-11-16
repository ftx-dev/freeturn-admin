// Package store - БД админки: SQLite (по умолчанию) или PostgreSQL, одна схема для обоих.
// Запросы пишутся с «?», для postgres переписываются в $1, $2, ...
// Время - unix-секунды (BIGINT), id - случайный текст: так схема не зависит от диалекта.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("не найдено")

type Store struct {
	db     *sql.DB
	driver string
}

func Open(ctx context.Context, driver, dsn string) (*Store, error) {
	name := map[string]string{"sqlite": "sqlite", "postgres": "pgx"}[driver]
	if name == "" {
		return nil, fmt.Errorf("store: неизвестный драйвер %q", driver)
	}
	if driver == "sqlite" {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open(name, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		db.SetMaxOpenConns(1) // SQLite пишет одним соединением: так не бывает «database is locked».
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: нет соединения с БД: %w", err)
	}
	s := &Store{db: db, driver: driver}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// q переписывает «?» в $n для postgres. Знаков вопроса внутри строковых литералов в запросах нет.
func (s *Store) q(query string) string {
	if s.driver != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.q(query), args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.q(query), args...)
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.q(query), args...)
}

func now() int64 { return time.Now().Unix() }

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Миграции только добавляются в конец; применённые не меняются.
var migrations = []string{
	`CREATE TABLE admins (
		id TEXT PRIMARY KEY,
		login TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'admin',
		created_at BIGINT NOT NULL,
		last_login_at BIGINT NOT NULL DEFAULT 0
	);
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
		csrf TEXT NOT NULL,
		created_at BIGINT NOT NULL,
		expires_at BIGINT NOT NULL,
		ip TEXT NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX sessions_admin ON sessions(admin_id);
	CREATE TABLE servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		host TEXT NOT NULL,
		ssh_port INTEGER NOT NULL,
		ssh_user TEXT NOT NULL,
		auth_type TEXT NOT NULL,
		ssh_secret_enc TEXT NOT NULL DEFAULT '',
		key_passphrase_enc TEXT NOT NULL DEFAULT '',
		sudo_secret_enc TEXT NOT NULL DEFAULT '',
		host_fingerprint TEXT NOT NULL DEFAULT '',
		root_mode TEXT NOT NULL DEFAULT '',
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL
	);
	CREATE TABLE audit (
		id TEXT PRIMARY KEY,
		admin_id TEXT NOT NULL DEFAULT '',
		server_id TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL,
		details TEXT NOT NULL DEFAULT '{}',
		result TEXT NOT NULL DEFAULT 'ok',
		at BIGINT NOT NULL
	);
	CREATE INDEX audit_at ON audit(at);`,
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at BIGINT NOT NULL)`); err != nil {
		return fmt.Errorf("store: миграции: %w", err)
	}
	var current int
	if err := s.queryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}
	for v := current + 1; v <= len(migrations); v++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		// Драйвер SQLite выполняет за раз один оператор - режем по «;».
		for _, stmt := range strings.Split(migrations[v-1], ";") {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("store: миграция %d: %w", v, err)
			}
		}
		if _, err := tx.ExecContext(ctx, s.q(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`), v, now()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
