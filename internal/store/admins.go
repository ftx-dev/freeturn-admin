package store

import (
	"context"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
)

type Admin struct {
	ID           string `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"createdAt"`
	LastLoginAt  int64  `json:"lastLoginAt"`
}

type Session struct {
	ID        string // хэш токена
	AdminID   string
	CSRF      string
	ExpiresAt int64
}

const adminCols = `id, login, password_hash, role, created_at, last_login_at`

func scanAdmin(sc interface{ Scan(...any) error }) (Admin, error) {
	var a Admin
	err := sc.Scan(&a.ID, &a.Login, &a.PasswordHash, &a.Role, &a.CreatedAt, &a.LastLoginAt)
	return a, notFound(err)
}

func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n)
	return n, err
}

func (s *Store) CreateAdmin(ctx context.Context, login, passwordHash, role string) (Admin, error) {
	a := Admin{ID: auth.ID(), Login: login, PasswordHash: passwordHash, Role: role, CreatedAt: now()}
	_, err := s.exec(ctx, `INSERT INTO admins (`+adminCols+`) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.Login, a.PasswordHash, a.Role, a.CreatedAt, a.LastLoginAt)
	return a, err
}

func (s *Store) AdminByLogin(ctx context.Context, login string) (Admin, error) {
	return scanAdmin(s.queryRow(ctx, `SELECT `+adminCols+` FROM admins WHERE login = ?`, login))
}

func (s *Store) AdminByID(ctx context.Context, id string) (Admin, error) {
	return scanAdmin(s.queryRow(ctx, `SELECT `+adminCols+` FROM admins WHERE id = ?`, id))
}

func (s *Store) TouchLogin(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `UPDATE admins SET last_login_at = ? WHERE id = ?`, now(), id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash, adminID, csrf string, ttlSeconds int64, ip, ua string) error {
	t := now()
	_, err := s.exec(ctx, `INSERT INTO sessions (id, admin_id, csrf, created_at, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tokenHash, adminID, csrf, t, t+ttlSeconds, ip, ua)
	return err
}

// SessionByHash отдаёт только живую сессию.
func (s *Store) SessionByHash(ctx context.Context, tokenHash string) (Session, error) {
	var se Session
	err := s.queryRow(ctx, `SELECT id, admin_id, csrf, expires_at FROM sessions WHERE id = ? AND expires_at > ?`, tokenHash, now()).
		Scan(&se.ID, &se.AdminID, &se.CSRF, &se.ExpiresAt)
	return se, notFound(err)
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE id = ?`, tokenHash)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now())
	return err
}
