package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"PullRequestAssigner/internal/domain"
)

// PRRepository — контракт для работы с PR.
type PRRepository interface {
	Create(ctx context.Context, pr *domain.PullRequest) error
	GetByID(ctx context.Context, id domain.PullRequestID) (*domain.PullRequest, error)
	UpdateReviewers(ctx context.Context, prID domain.PullRequestID, reviewers []domain.UserID) error
	MarkMerged(ctx context.Context, id domain.PullRequestID, mergedAt time.Time) error
	ListByReviewer(ctx context.Context, reviewerID domain.UserID) ([]domain.PullRequestShort, error)
}

// RandomSource — абстракция над rand.Intn, чтобы можно было подменять в тестах.
type RandomSource interface {
	Intn(n int) int
}

type defaultRandSource struct{}

func (defaultRandSource) Intn(n int) int {
	return rand.Intn(n)
}

// Ошибка, которая мапится на код NO_CANDIDATE.
var ErrNoCandidate = errors.New("no active replacement candidate in team")

// PRService — бизнес-логика вокруг PR: создание, назначение ревьюверов,
// merge, переназначение и выбор по ревьюверу.
type PRService struct {
	users UserRepository
	teams TeamRepository
	prs   PRRepository
	rand  RandomSource
}

func NewPRService(
	users UserRepository,
	teams TeamRepository,
	prs PRRepository,
) *PRService {
	return &PRService{
		users: users,
		teams: teams,
		prs:   prs,
		rand:  defaultRandSource{},
	}
}

// Для тестов можно подменить источник случайности.
func (s *PRService) WithRandomSource(src RandomSource) *PRService {
	s.rand = src
	return s
}

// CreatePR — /pullRequest/create
// 1. Проверяет, что автор существует.
// 2. Проверяет, что команда автора существует.
// 3. Выбирает до двух активных ревьюверов из команды автора (кроме автора).
// 4. Создаёт PR в репозитории.
func (s *PRService) CreatePR(
	ctx context.Context,
	prID domain.PullRequestID,
	name string,
	authorID domain.UserID,
) (*domain.PullRequest, error) {
	// Автор
	author, err := s.users.GetByID(ctx, authorID)
	if err != nil {
		return nil, err // domain.ErrNotFound - 404
	}

	// Команда автора
	team, err := s.teams.GetTeamWithMembers(ctx, author.TeamName)
	if err != nil {
		return nil, err // domain.ErrNotFound
	}

	var candidates []domain.UserID
	for _, m := range team.Members {
		if !m.IsActive {
			continue
		}
		if m.UserID == author.ID {
			continue
		}
		candidates = append(candidates, m.UserID)
	}

	reviewers := pickRandomWithoutReplacement(s.rand, candidates, domain.MaxReviewersPerPR)

	// Доменный объект PR
	pr, err := domain.NewPullRequest(prID, name, author.ID)
	if err != nil {
		return nil, err
	}
	if err := pr.SetReviewers(reviewers); err != nil {
		return nil, err
	}

	// Сохранение
	if err := s.prs.Create(ctx, pr); err != nil {
		return nil, err // domain.ErrPullRequestExists 409 PR_EXISTS
	}

	return pr, nil
}

// MergePR — /pullRequest/merge
// Идемпотентен: повторный вызов не даёт ошибку и возвращает актуальное состояние PR.
func (s *PRService) MergePR(
	ctx context.Context,
	prID domain.PullRequestID,
) (*domain.PullRequest, error) {
	now := time.Now().UTC()

	if err := s.prs.MarkMerged(ctx, prID, now); err != nil {
		return nil, err // domain.ErrNotFound 404
	}

	// Возвращаем актуальное состояние из БД.
	return s.prs.GetByID(ctx, prID)
}

// ReassignReviewer — /pullRequest/reassign
// Переназначает одного ревьювера на случайного активного участника из его команды.
func (s *PRService) ReassignReviewer(
	ctx context.Context,
	prID domain.PullRequestID,
	oldReviewerID domain.UserID,
) (*domain.PullRequest, domain.UserID, error) {
	// Берём PR
	pr, err := s.prs.GetByID(ctx, prID)
	if err != nil {
		return nil, "", err //domain.ErrNotFound
	}

	// Если PR уже MERGED то доменное правило запрещает менять ревьюверов.
	if !pr.CanModifyReviewers() {
		return nil, "", domain.ErrReviewersLocked //409 PR_MERGED
	}

	// Старый ревьювер как пользователь (нужна его команда).
	oldReviewer, err := s.users.GetByID(ctx, oldReviewerID)
	if err != nil {
		return nil, "", err // domain.ErrNotFound 404
	}

	// Команда заменяемого ревьювера
	team, err := s.teams.GetTeamWithMembers(ctx, oldReviewer.TeamName)
	if err != nil {
		return nil, "", err //domain.ErrNotFound
	}

	// Кандидаты: активные из команды, не старый ревьювер, не автор, не другой текущий ревьювер.
	var candidates []domain.UserID
	for _, m := range team.Members {
		if !m.IsActive {
			continue
		}
		if m.UserID == oldReviewer.ID {
			continue
		}
		if m.UserID == pr.AuthorID {
			continue
		}
		if isOtherReviewer(pr.Reviewers, oldReviewerID, m.UserID) {
			// чтобы не получить дубликата ревьювера
			continue
		}
		candidates = append(candidates, m.UserID)
	}

	if len(candidates) == 0 {
		return nil, "", ErrNoCandidate //409 NO_CANDIDATE
	}

	newReviewerID := candidates[s.rand.Intn(len(candidates))]

	// Доменная операция замены ревьювера.
	if err := pr.ReplaceReviewer(oldReviewerID, newReviewerID); err != nil {
		return nil, "", err
	}

	// Обновляем список ревьюверов в БД
	if err := s.prs.UpdateReviewers(ctx, pr.ID, pr.Reviewers); err != nil {
		return nil, "", err
	}

	return pr, newReviewerID, nil
}

// ListPRsByReviewer — /users/getReview
func (s *PRService) ListPRsByReviewer(
	ctx context.Context,
	reviewerID domain.UserID,
) ([]domain.PullRequestShort, error) {
	return s.prs.ListByReviewer(ctx, reviewerID)
}

// выбор до max элементов без повторов
func pickRandomWithoutReplacement(
	rnd RandomSource,
	items []domain.UserID,
	max int,
) []domain.UserID {
	n := len(items)
	if n == 0 || max <= 0 {
		return nil
	}
	if n <= max {
		// копируем, чтобы не мутировать исходный слайс
		res := make([]domain.UserID, n)
		copy(res, items)
		return res
	}

	// Фишер-Йетс: перемешиваем индексы и берём первые max.
	indexes := make([]int, n)
	for i := range items {
		indexes[i] = i
	}

	for i := n - 1; i > 0; i-- {
		j := rnd.Intn(i + 1)
		indexes[i], indexes[j] = indexes[j], indexes[i]
	}

	res := make([]domain.UserID, 0, max)
	for i := 0; i < max; i++ {
		res = append(res, items[indexes[i]])
	}
	return res
}

// isOtherReviewer — помогает исключить из кандидатов уже назначенного второго ревьювера.
func isOtherReviewer(
	currentReviewers []domain.UserID,
	oldReviewerID domain.UserID,
	candidate domain.UserID,
) bool {
	for _, r := range currentReviewers {
		if r == oldReviewerID {
			continue
		}
		if r == candidate {
			return true
		}
	}
	return false
}
