package stats

import (
	"context"

	"PullRequestAssigner/internal/domain"
)

// ByUserStat агрегированная статистика по пользователям.
type ByUserStat struct {
	UserID             domain.UserID
	AssignedAsReviewer int // сколько раз назначался ревьювером
	Authored           int // сколько PR создал
}

// ByPRStat агрегированная статистика по PR.
type ByPRStat struct {
	PRID          domain.PullRequestID
	ReviewerCount int // сколько ревьюверов назначено на PR
}

// Snapshot снимок всей статистики, который удобно отдавать в одном ответе
type Snapshot struct {
	ByUser []ByUserStat
	ByPR   []ByPRStat
}

// Repository — контракт для уровня хранилища
// Реализацию можно сделать в postgres-пакете, отдельно от обычных репозиториев.
type Repository interface {
	// CountAssignmentsByUser возвращает статистику по пользователям.
	CountAssignmentsByUser(ctx context.Context) ([]ByUserStat, error)

	// CountAssignmentsByPR возвращает статистику по PR.
	CountAssignmentsByPR(ctx context.Context) ([]ByPRStat, error)
}

// Service — сервис статистики, опирающийся на Repository.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetSnapshot — получить полный снимок статистики.
// Можно повесить на /stats (GET), если будешь делать доп. ручку.
func (s *Service) GetSnapshot(ctx context.Context) (*Snapshot, error) {
	byUser, err := s.repo.CountAssignmentsByUser(ctx)
	if err != nil {
		return nil, err
	}

	byPR, err := s.repo.CountAssignmentsByPR(ctx)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		ByUser: byUser,
		ByPR:   byPR,
	}, nil
}
