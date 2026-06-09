package postgres

import (
	"context"
	"database/sql"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.DeviceRepository = (*DeviceRepository)(nil)

type DeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// Save faz upsert do dispositivo por token: um token sempre aponta para o
// usuário/plataforma mais recente que o registrou.
func (r *DeviceRepository) Save(ctx context.Context, userID, token, platform string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO device_tokens (user_id, token, platform, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())
		 ON CONFLICT (token) DO UPDATE SET
		   user_id    = EXCLUDED.user_id,
		   platform   = EXCLUDED.platform,
		   updated_at = NOW()`,
		userID, token, platform,
	)
	return err
}

func (r *DeviceRepository) FindByUser(ctx context.Context, userID string) ([]port.Device, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT user_id, token, platform FROM device_tokens WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []port.Device
	for rows.Next() {
		var d port.Device
		if err := rows.Scan(&d.UserID, &d.Token, &d.Platform); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}
