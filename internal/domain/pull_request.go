package domain

import (
	"errors"
	"strings"
	"time"
)

// PullRequestID — тип идентификатора PR.
type PullRequestID string

// PRStatus — статус PR
type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"

	// MaxReviewersPerPR — бизнес-ограничение из условия.
	MaxReviewersPerPR = 2
)

type PullRequest struct {
	ID        PullRequestID
	Name      string
	AuthorID  UserID
	Status    PRStatus
	Reviewers []UserID
	CreatedAt *time.Time
	MergedAt  *time.Time
}

type PullRequestShort struct {
	ID       PullRequestID
	Name     string
	AuthorID UserID
	Status   PRStatus
}

var (
	ErrReviewersLocked   = errors.New("cannot modify reviewers of merged pull request")
	ErrEmptyPRName       = errors.New("pull request name cannot be empty")
	ErrReviewerIsAuthor  = errors.New("reviewer cannot be the author of the pull request")
	ErrDuplicateReviewer = errors.New("duplicate reviewer in pull request")
	ErrReviewerNotFound  = errors.New("reviewer not found in pull request")
	ErrInvalidPRStatus   = errors.New("invalid pull request status")
	ErrTooManyReviewers  = errors.New("too many reviewers for pull request")
	ErrEmptyPRID         = errors.New("pull request ID cannot be empty")
	ErrPullRequestExists = errors.New("pull request with this ID already exists")
)

// Cоздаёт PR в статусе OPEN без ревьюверов.
func NewPullRequest(id PullRequestID, name string, authorID UserID) (*PullRequest, error) {
	name = strings.TrimSpace(name)
	if id == "" {
		return nil, ErrEmptyPRID
	}
	if name == "" {
		return nil, ErrEmptyPRName
	}
	return &PullRequest{
		ID:       id,
		Name:     name,
		AuthorID: authorID,
		Status:   PRStatusOpen,
	}, nil
}

// Геттер можно ли менять ревьюверов в текущем статусе PR.
func (pr PullRequest) CanModifyReviewers() bool {
	return pr.Status == PRStatusOpen
}

// Полностью заменяет список ревьюверов, проверяя инварианты.
func (pr *PullRequest) SetReviewers(reviewers []UserID) error {
	if !pr.CanModifyReviewers() {
		return ErrReviewersLocked
	}

	if len(reviewers) > MaxReviewersPerPR {
		return ErrTooManyReviewers
	}

	if hasDuplicateUserID(reviewers) {
		return ErrDuplicateReviewer
	}

	for _, r := range reviewers {
		if r == pr.AuthorID {
			return ErrReviewerIsAuthor
		}
	}

	// Копируем слайс, чтобы не зависеть от внешних изменений
	pr.Reviewers = append([]UserID(nil), reviewers...)
	return nil
}

// Реализует переназначение одного ревьювера
// Доменный метод только проверяет инварианты и заменяет одного на другого.
func (pr *PullRequest) ReplaceReviewer(oldReviewer, newReviewer UserID) error {
	if !pr.CanModifyReviewers() {
		return ErrReviewersLocked
	}

	if newReviewer == pr.AuthorID {
		return ErrReviewerIsAuthor
	}

	// Если новый такой же, что и старый то нет смысла что-то менять.
	if oldReviewer == newReviewer {
		return nil
	}

	idx := -1
	for i, r := range pr.Reviewers {
		if r == oldReviewer {
			idx = i
			break
		}
	}

	if idx == -1 {
		return ErrReviewerNotFound
	}

	// Проверим, что newReviewer ещё не в списке (кроме oldReviewer)
	for _, r := range pr.Reviewers {
		if r == newReviewer {
			return ErrDuplicateReviewer
		}
	}

	pr.Reviewers[idx] = newReviewer
	return nil
}

// Делает merge идемпотентным: повторный вызов не приводит к ошибке.
func (pr *PullRequest) Merge(at time.Time) {
	if pr.Status == PRStatusMerged {
		return
	}
	pr.Status = PRStatusMerged
	t := at.UTC()
	pr.MergedAt = &t

}

// Вспомогательная функция для проверки дубликатов
func hasDuplicateUserID(ids []UserID) bool {
	if len(ids) <= 1 {
		return false
	}
	seen := make(map[UserID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}
