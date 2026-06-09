package postgres

import (
	"context"
	"database/sql"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

var _ port.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Save(ctx context.Context, user *entity.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, birth_date, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET name = $2, email = $3, birth_date = $4`,
		user.ID, user.Name, user.Email, user.BirthDate, user.CreatedAt,
	)
	return err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, birth_date, created_at FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, birth_date, created_at FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*entity.User, error) {
	var u entity.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.BirthDate, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
