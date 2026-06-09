package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

var _ port.ActivityRepository = (*ActivityRepository)(nil)

type ActivityRepository struct {
	db *sql.DB
}

func NewActivityRepository(db *sql.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Save(ctx context.Context, a *entity.Activity) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO activities (id, user_id, type, date, duration_ns, avg_heart_rate, external_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.UserID, string(a.Type), a.Date, a.Duration.Nanoseconds(), a.AvgHeartRate, nullString(a.ExternalID), a.CreatedAt,
	)
	return err
}

func (r *ActivityRepository) FindByID(ctx context.Context, id string) (*entity.Activity, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, type, date, duration_ns, avg_heart_rate, COALESCE(external_id, ''), created_at
		 FROM activities WHERE id = $1`, id,
	)
	return scanActivity(row)
}

func (r *ActivityRepository) FindByUserID(ctx context.Context, userID string, from, to time.Time) ([]*entity.Activity, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, type, date, duration_ns, avg_heart_rate, COALESCE(external_id, ''), created_at
		 FROM activities WHERE user_id = $1 AND date >= $2 AND date <= $3
		 ORDER BY date DESC
		 LIMIT 1000`,
		userID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*entity.Activity
	for rows.Next() {
		a, err := scanActivityRow(rows)
		if err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

func (r *ActivityRepository) FindByExternalID(ctx context.Context, externalID string) (*entity.Activity, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, type, date, duration_ns, avg_heart_rate, COALESCE(external_id, ''), created_at
		 FROM activities WHERE external_id = $1`, externalID,
	)
	return scanActivity(row)
}

func scanActivity(row *sql.Row) (*entity.Activity, error) {
	var a entity.Activity
	var actType string
	var durationNs int64
	err := row.Scan(&a.ID, &a.UserID, &actType, &a.Date, &durationNs, &a.AvgHeartRate, &a.ExternalID, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Type = valueobject.ActivityType(actType)
	a.Duration = time.Duration(durationNs)
	return &a, nil
}

func scanActivityRow(rows *sql.Rows) (*entity.Activity, error) {
	var a entity.Activity
	var actType string
	var durationNs int64
	err := rows.Scan(&a.ID, &a.UserID, &actType, &a.Date, &durationNs, &a.AvgHeartRate, &a.ExternalID, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.Type = valueobject.ActivityType(actType)
	a.Duration = time.Duration(durationNs)
	return &a, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
