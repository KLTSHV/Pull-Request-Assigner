package postgres

import (
	"context"
	"database/sql"
	"time"

	"PullRequestAssigner/internal/domain"
)

// PullRequestRepository — операции с PR.
//
// Таблица pull_requests:
//
//	pull_request_id   TEXT PRIMARY KEY
//	pull_request_name TEXT NOT NULL
//	author_id         TEXT NOT NULL REFERENCES users(user_id)
//	status            TEXT NOT NULL
//	created_at        TIMESTAMPTZ
//	merged_at         TIMESTAMPTZ
//
// Таблица pull_request_reviewers:
//
//	pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE
//	reviewer_id     TEXT NOT NULL REFERENCES users(user_id)
//	PRIMARY KEY (pull_request_id, reviewer_id)
type PullRequestRepository struct {
	db *sql.DB
}

func NewPullRequestRepository(db *sql.DB) *PullRequestRepository {
	return &PullRequestRepository{db: db}
}

// Create создаёт PR и его ревьюверов.
// Если PR с таким ID уже есть — ErrPullRequestExists.
// Ответом считается успешно созданный PR (сам объект мы уже имеем в аргументе).
func (r *PullRequestRepository) Create(ctx context.Context, pr *domain.PullRequest) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Проверяем, что PR ещё не существует.
	const checkPR = `SELECT 1 FROM pull_requests WHERE pull_request_id = $1`
	var dummy int
	err = tx.QueryRowContext(ctx, checkPR, pr.ID).Scan(&dummy)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		return domain.ErrPullRequestExists
	}

	// Если CreatedAt не проставлен — задаём сейчас.
	if pr.CreatedAt == nil {
		now := time.Now().UTC()
		pr.CreatedAt = &now
	}

	const insertPR = `
INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at, merged_at)
VALUES ($1, $2, $3, $4, $5, $6)
`
	if _, err = tx.ExecContext(ctx, insertPR,
		pr.ID,
		pr.Name,
		pr.AuthorID,
		pr.Status,
		pr.CreatedAt,
		pr.MergedAt,
	); err != nil {
		return err
	}

	if len(pr.Reviewers) > 0 {
		const insertReviewer = `
INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id)
VALUES ($1, $2)
`
		stmt, errStmt := tx.PrepareContext(ctx, insertReviewer)
		if errStmt != nil {
			err = errStmt
			return err
		}
		defer stmt.Close()

		for _, rID := range pr.Reviewers {
			if _, err = stmt.ExecContext(ctx, pr.ID, rID); err != nil {
				return err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetByID возвращает PR с ревьюверами или ErrNotFound.
func (r *PullRequestRepository) GetByID(
	ctx context.Context,
	id domain.PullRequestID,
) (*domain.PullRequest, error) {
	const prQuery = `
SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
FROM pull_requests
WHERE pull_request_id = $1
`
	row := r.db.QueryRowContext(ctx, prQuery, id)

	var pr domain.PullRequest
	var status string
	var createdAt, mergedAt sql.NullTime
	if err := row.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &status, &createdAt, &mergedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	pr.Status = domain.PRStatus(status)
	if createdAt.Valid {
		t := createdAt.Time
		pr.CreatedAt = &t
	}
	if mergedAt.Valid {
		t := mergedAt.Time
		pr.MergedAt = &t
	}

	// Ревьюверы.
	const reviewersQuery = `
SELECT reviewer_id
FROM pull_request_reviewers
WHERE pull_request_id = $1
ORDER BY reviewer_id
`
	rows, err := r.db.QueryContext(ctx, reviewersQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviewers []domain.UserID
	for rows.Next() {
		var rid domain.UserID
		if err := rows.Scan(&rid); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, rid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pr.Reviewers = reviewers

	return &pr, nil
}

// UpdateReviewers полностью заменяет список ревьюверов для PR.
// Статус не проверяется — это делает сервис по доменным правилам.
func (r *PullRequestRepository) UpdateReviewers(
	ctx context.Context,
	prID domain.PullRequestID,
	reviewers []domain.UserID,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Проверяем, что PR существует.
	const checkPR = `SELECT 1 FROM pull_requests WHERE pull_request_id = $1`
	var dummy int
	if err = tx.QueryRowContext(ctx, checkPR, prID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		return err
	}

	// Удаляем старые ревьюверы.
	const deleteReviewers = `DELETE FROM pull_request_reviewers WHERE pull_request_id = $1`
	if _, err = tx.ExecContext(ctx, deleteReviewers, prID); err != nil {
		return err
	}

	// Вставляем новых (если есть).
	if len(reviewers) > 0 {
		const insertReviewer = `
INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id)
VALUES ($1, $2)
`
		stmt, errStmt := tx.PrepareContext(ctx, insertReviewer)
		if errStmt != nil {
			err = errStmt
			return err
		}
		defer stmt.Close()

		for _, rID := range reviewers {
			if _, err = stmt.ExecContext(ctx, prID, rID); err != nil {
				return err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// MarkMerged помечает PR как MERGED и устанавливает merged_at.
// Операция идемпотентная: merged_at ставится только при первом вызове,
// дальше не меняется
func (r *PullRequestRepository) MarkMerged(
	ctx context.Context,
	id domain.PullRequestID,
	mergedAt time.Time,
) error {
	const q = `
UPDATE pull_requests
SET status   = $2,
    merged_at = COALESCE(merged_at, $3)
WHERE pull_request_id = $1
`
	res, err := r.db.ExecContext(ctx, q, id, domain.PRStatusMerged, mergedAt.UTC())
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Возвращает список PR'ов (коротких), где пользователь — ревьювер.
// Используется в /users/getReview.
func (r *PullRequestRepository) ListByReviewer(
	ctx context.Context,
	reviewerID domain.UserID,
) ([]domain.PullRequestShort, error) {
	const q = `
SELECT p.pull_request_id,
       p.pull_request_name,
       p.author_id,
       p.status
FROM pull_requests p
JOIN pull_request_reviewers r
  ON p.pull_request_id = r.pull_request_id
WHERE r.reviewer_id = $1
ORDER BY p.created_at DESC
`
	rows, err := r.db.QueryContext(ctx, q, reviewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.PullRequestShort
	for rows.Next() {
		var s domain.PullRequestShort
		var status string
		if err := rows.Scan(&s.ID, &s.Name, &s.AuthorID, &status); err != nil {
			return nil, err
		}
		s.Status = domain.PRStatus(status)
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
