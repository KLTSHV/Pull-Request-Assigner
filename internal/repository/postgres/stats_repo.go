package postgres

import (
	"context"
	"database/sql"

	"PullRequestAssigner/internal/stats"
)

// StatsRepository реализует stats.Repository.
type StatsRepository struct {
	db *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// CountAssignmentsByUser — сколько раз пользователь был автором и ревьювером
func (r *StatsRepository) CountAssignmentsByUser(ctx context.Context) ([]stats.ByUserStat, error) {
	const q = `
SELECT
  u.user_id,
  COALESCE(COUNT(DISTINCT p_author.pull_request_id), 0) AS authored,
  COALESCE(COUNT(DISTINCT p_rev.pull_request_id), 0)    AS assigned_as_reviewer
FROM users u
LEFT JOIN pull_requests p_author
  ON p_author.author_id = u.user_id
LEFT JOIN pull_request_reviewers p_rev
  ON p_rev.reviewer_id = u.user_id
GROUP BY u.user_id
ORDER BY u.user_id;
`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []stats.ByUserStat
	for rows.Next() {
		var s stats.ByUserStat
		if err := rows.Scan(&s.UserID, &s.Authored, &s.AssignedAsReviewer); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// CountAssignmentsByPR — сколько ревьюверов у каждого PR.
func (r *StatsRepository) CountAssignmentsByPR(ctx context.Context) ([]stats.ByPRStat, error) {
	const q = `
SELECT
  p.pull_request_id,
  COUNT(r.reviewer_id) AS reviewer_count
FROM pull_requests p
LEFT JOIN pull_request_reviewers r
  ON r.pull_request_id = p.pull_request_id
GROUP BY p.pull_request_id
ORDER BY p.pull_request_id;
`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []stats.ByPRStat
	for rows.Next() {
		var s stats.ByPRStat
		if err := rows.Scan(&s.PRID, &s.ReviewerCount); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
