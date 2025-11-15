package postgres

import (
	"context"
	"database/sql"

	"PullRequestAssigner/internal/domain"
)

// UserRepository отвечает за операции над users
// Таблица users:
//
//	user_id   TEXT PRIMARY KEY
//	username  TEXT NOT NULL
//	team_name TEXT NOT NULL REFERENCES teams(team_name)
//	is_active BOOLEAN NOT NULL
type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Возвращает пользователя по ID или ErrNotFound.
func (r *UserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	const query = `
SELECT user_id, username, team_name, is_active
FROM users
WHERE user_id = $1
`
	row := r.db.QueryRowContext(ctx, query, id)

	var u domain.User
	var teamName string
	if err := row.Scan(&u.ID, &u.Username, &teamName, &u.IsActive); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	u.TeamName = domain.TeamName(teamName)
	return &u, nil
}

// Обновляет флаг активности и возвращает обновлённого пользователя.
// Используется для /users/setIsActive.
func (r *UserRepository) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (*domain.User, error) {
	const query = `
UPDATE users
SET is_active = $2
WHERE user_id = $1
RETURNING user_id, username, team_name, is_active
`
	row := r.db.QueryRowContext(ctx, query, id, isActive)

	var u domain.User
	var teamName string
	if err := row.Scan(&u.ID, &u.Username, &teamName, &u.IsActive); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	u.TeamName = domain.TeamName(teamName)
	return &u, nil
}
