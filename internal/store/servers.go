package store

import (
	"context"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
)

// Server - VPS с FreeTurn. Поля *Enc зашифрованы secret.Box и наружу не отдаются.
type Server struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Host             string `json:"host"`
	SSHPort          int    `json:"sshPort"`
	SSHUser          string `json:"sshUser"`
	AuthType         string `json:"authType"` // password | key
	SSHSecretEnc     string `json:"-"`
	KeyPassphraseEnc string `json:"-"`
	SudoSecretEnc    string `json:"-"`
	HostFingerprint  string `json:"hostFingerprint"`
	RootMode         string `json:"rootMode"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

const serverCols = `id, name, host, ssh_port, ssh_user, auth_type, ssh_secret_enc, key_passphrase_enc, sudo_secret_enc,
	host_fingerprint, root_mode, created_at, updated_at`

func scanServer(sc interface{ Scan(...any) error }) (Server, error) {
	var v Server
	err := sc.Scan(&v.ID, &v.Name, &v.Host, &v.SSHPort, &v.SSHUser, &v.AuthType, &v.SSHSecretEnc, &v.KeyPassphraseEnc,
		&v.SudoSecretEnc, &v.HostFingerprint, &v.RootMode, &v.CreatedAt, &v.UpdatedAt)
	return v, notFound(err)
}

func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.query(ctx, `SELECT `+serverCols+` FROM servers ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		v, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	return scanServer(s.queryRow(ctx, `SELECT `+serverCols+` FROM servers WHERE id = ?`, id))
}

func (s *Store) CreateServer(ctx context.Context, v Server) (Server, error) {
	v.ID = auth.ID()
	v.CreatedAt = now()
	v.UpdatedAt = v.CreatedAt
	_, err := s.exec(ctx, `INSERT INTO servers (`+serverCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Name, v.Host, v.SSHPort, v.SSHUser, v.AuthType, v.SSHSecretEnc, v.KeyPassphraseEnc, v.SudoSecretEnc,
		v.HostFingerprint, v.RootMode, v.CreatedAt, v.UpdatedAt)
	return v, err
}

// UpdateServer пишет запись целиком. Смена адреса или порта сбрасывает отпечаток хоста: это уже другой ключ.
func (s *Store) UpdateServer(ctx context.Context, v Server) error {
	v.UpdatedAt = now()
	res, err := s.exec(ctx, `UPDATE servers SET name = ?, host = ?, ssh_port = ?, ssh_user = ?, auth_type = ?, ssh_secret_enc = ?,
		key_passphrase_enc = ?, sudo_secret_enc = ?, host_fingerprint = ?, root_mode = ?, updated_at = ? WHERE id = ?`,
		v.Name, v.Host, v.SSHPort, v.SSHUser, v.AuthType, v.SSHSecretEnc, v.KeyPassphraseEnc, v.SudoSecretEnc,
		v.HostFingerprint, v.RootMode, v.UpdatedAt, v.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteServer(ctx context.Context, id string) error {
	res, err := s.exec(ctx, `DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Calls / SetCalls - зашифрованный JSON со ссылками на звонок сервера.
func (s *Store) Calls(ctx context.Context, id string) (string, error) {
	var enc string
	err := s.queryRow(ctx, `SELECT calls_enc FROM servers WHERE id = ?`, id).Scan(&enc)
	return enc, notFound(err)
}

func (s *Store) SetCalls(ctx context.Context, id, enc string) error {
	return affected(s.exec(ctx, `UPDATE servers SET calls_enc = ? WHERE id = ?`, enc, id))
}
