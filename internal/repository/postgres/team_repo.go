package postgres

import (
	"context"
	"database/sql"

	"PullRequestAssigner/internal/domain"
)

// TeamRepository — операции над командами и их участниками.
//
// Таблица teams:
//
//	team_name TEXT PRIMARY KEY
//
// Пользователи команды берутся из users по team_name.
type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Cоздаёт новую команду и upsert'ит пользователей.
// Если команда уже существует — ErrTeamExists.
// Используется в /team/add.
func (r *TeamRepository) CreateTeamWithMembers(
	ctx context.Context,
	teamName domain.TeamName,
	members []domain.TeamMember,
) (*domain.Team, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Проверяем что команды ещё нет
	const checkTeam = `SELECT 1 FROM teams WHERE team_name = $1`
	var dummy int
	err = tx.QueryRowContext(ctx, checkTeam, teamName).Scan(&dummy)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		// Команда уже существует
		return nil, domain.ErrTeamExists
	}

	// Создаём команду.
	const insertTeam = `INSERT INTO teams (team_name) VALUES ($1)`
	if _, err := tx.ExecContext(ctx, insertTeam, teamName); err != nil {
		return nil, err
	}

	// Upsert пользователей.
	const upsertUser = `
INSERT INTO users (user_id, username, team_name, is_active)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE
SET username = EXCLUDED.username,
    team_name = EXCLUDED.team_name,
    is_active = EXCLUDED.is_active
`
	stmt, err := tx.PrepareContext(ctx, upsertUser)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for _, m := range members {
		if _, err := stmt.ExecContext(ctx, m.UserID, m.Username, teamName, m.IsActive); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	t := &domain.Team{
		Name:    teamName,
		Members: append([]domain.TeamMember(nil), members...),
	}
	return t, nil
}

// GetTeamWithMembers возвращает команду и её участников или ErrNotFound.
// Используется в /team/get.
func (r *TeamRepository) GetTeamWithMembers(
	ctx context.Context,
	teamName domain.TeamName,
) (*domain.Team, error) {
	// Сначала проверяем, что команда существует.
	const checkTeam = `SELECT team_name FROM teams WHERE team_name = $1`
	row := r.db.QueryRowContext(ctx, checkTeam, teamName)

	var name string
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	// Затем берём участников.
	const membersQuery = `
SELECT user_id, username, is_active
FROM users
WHERE team_name = $1
ORDER BY user_id
`
	rows, err := r.db.QueryContext(ctx, membersQuery, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &domain.Team{
		Name:    domain.TeamName(name),
		Members: members,
	}, nil
}

// ListActiveMembers возвращает активных участников команды.
// Удобно для назначения/переназначения ревьюверов.
func (r *TeamRepository) ListActiveMembers(
	ctx context.Context,
	teamName domain.TeamName,
) ([]domain.TeamMember, error) {
	const q = `
SELECT user_id, username, is_active
FROM users
WHERE team_name = $1 AND is_active = TRUE
`
	rows, err := r.db.QueryContext(ctx, q, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}
