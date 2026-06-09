package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

var _ port.InsightRepository = (*InsightRepository)(nil)

type InsightRepository struct {
	db *sql.DB
}

func NewInsightRepository(db *sql.DB) *InsightRepository {
	return &InsightRepository{db: db}
}

func (r *InsightRepository) Save(ctx context.Context, i *entity.Insight) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO insights (id, user_id, type, severity, message, date, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		i.ID, i.UserID, string(i.Type), string(i.Severity), i.Message, i.Date, i.CreatedAt,
	)
	return err
}

func (r *InsightRepository) SaveAll(ctx context.Context, insights []*entity.Insight) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO insights (id, user_id, type, severity, message, date, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, i := range insights {
		_, err := stmt.ExecContext(ctx, i.ID, i.UserID, string(i.Type), string(i.Severity), i.Message, i.Date, i.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *InsightRepository) FindByUserID(ctx context.Context, userID string, from, to time.Time) ([]*entity.Insight, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, type, severity, message, date, created_at
		 FROM insights WHERE user_id = $1 AND date >= $2 AND date <= $3
		 ORDER BY date DESC
		 LIMIT 1000`,
		userID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []*entity.Insight
	for rows.Next() {
		var i entity.Insight
		var it, sev string
		err := rows.Scan(&i.ID, &i.UserID, &it, &sev, &i.Message, &i.Date, &i.CreatedAt)
		if err != nil {
			return nil, err
		}
		i.Type = valueobject.InsightType(it)
		i.Severity = valueobject.Severity(sev)
		insights = append(insights, &i)
	}
	return insights, rows.Err()
}
