package store

import (
	"context"
	"fmt"
	"time"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
)

type AuditEntry struct {
	ID       string `json:"id"`
	AdminID  string `json:"adminId"`
	Login    string `json:"login"`
	ServerID string `json:"serverId"`
	Action   string `json:"action"`
	Details  string `json:"details"`
	Result   string `json:"result"`
	At       int64  `json:"at"`
}

func (s *Store) Audit(ctx context.Context, adminID, serverID, action, details, result string) error {
	if details == "" {
		details = "{}"
	}
	_, err := s.exec(ctx, `INSERT INTO audit (id, admin_id, server_id, action, details, result, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		// id начинается со времени в наносекундах: порядок записей внутри одной секунды сохраняется.
		fmt.Sprintf("%016x", time.Now().UnixNano())+auth.ID()[:8], adminID, serverID, action, details, result, now())
	return err
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.query(ctx, `SELECT a.id, a.admin_id, COALESCE(ad.login, ''), a.server_id, a.action, a.details, a.result, a.at
		FROM audit a LEFT JOIN admins ad ON ad.id = a.admin_id ORDER BY a.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.AdminID, &e.Login, &e.ServerID, &e.Action, &e.Details, &e.Result, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
