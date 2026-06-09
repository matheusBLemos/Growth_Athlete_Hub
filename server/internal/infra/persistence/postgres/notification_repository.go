package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.NotificationRepository = (*NotificationRepository)(nil)

// NotificationRepository persiste o histórico de notificações na tabela
// relacional notifications.
type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Save grava um registro de histórico. Gera um ID quando record.ID vier vazio.
func (r *NotificationRepository) Save(ctx context.Context, record port.NotificationRecord) error {
	if record.ID == "" {
		record.ID = generateNotificationID()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications
		   (id, user_id, insight_id, type, severity, title, body, status, error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
		record.ID, record.UserID, record.InsightID, record.Type, record.Severity,
		record.Title, record.Body, record.Status, record.Error,
	)
	return err
}

// ListByUser retorna os registros mais recentes do usuário, ordenados do mais
// novo para o mais antigo. Limit <= 0 cai num padrão seguro.
func (r *NotificationRepository) ListByUser(ctx context.Context, userID string, limit int) ([]port.NotificationRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, insight_id, type, severity, title, body, status, error, created_at
		   FROM notifications
		  WHERE user_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []port.NotificationRecord
	for rows.Next() {
		var rec port.NotificationRecord
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.InsightID, &rec.Type, &rec.Severity,
			&rec.Title, &rec.Body, &rec.Status, &rec.Error, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func generateNotificationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
}
