package service

import (
	"context"
	"math/rand"

	"PullRequestAssigner/internal/domain"
)

// TeamRepository — контракт, который реализует postgres.TeamRepository.
type TeamRepository interface {
	CreateTeamWithMembers(ctx context.Context, teamName domain.TeamName, members []domain.TeamMember) (*domain.Team, error)
	GetTeamWithMembers(ctx context.Context, teamName domain.TeamName) (*domain.Team, error)
	ListActiveMembers(ctx context.Context, teamName domain.TeamName) ([]domain.TeamMember, error)
}

// TeamService — бизнес-логика по командам + массовая деактивация.
type TeamService struct {
	teams TeamRepository
	users UserRepository
	prs   PRRepository
}

func NewTeamService(teams TeamRepository, users UserRepository, prs PRRepository) *TeamService {
	return &TeamService{
		teams: teams,
		users: users,
		prs:   prs,
	}
}

// CreateTeamWithMembers создание новой команды и upsert пользователей.
// Используется для /team/add.
func (s *TeamService) CreateTeamWithMembers(
	ctx context.Context,
	teamName domain.TeamName,
	members []domain.TeamMember,
) (*domain.Team, error) {
	return s.teams.CreateTeamWithMembers(ctx, teamName, members)
}

// GetTeam — команда + участники.
// Используется для /team/get.
func (s *TeamService) GetTeam(
	ctx context.Context,
	teamName domain.TeamName,
) (*domain.Team, error) {
	return s.teams.GetTeamWithMembers(ctx, teamName)
}

// ListActiveMembers — активные участники команды.
// Удобно использовать из PR-сервиса.
func (s *TeamService) ListActiveMembers(
	ctx context.Context,
	teamName domain.TeamName,
) ([]domain.TeamMember, error) {
	return s.teams.ListActiveMembers(ctx, teamName)
}

// МАССОВАЯ ДЕАКТИВАЦИЯ!

type BulkDeactivateResult struct {
	TeamName              domain.TeamName
	DeactivatedUserIDs    []domain.UserID
	UpdatedPullRequestIDs []domain.PullRequestID
}

// DeactivateMembersAndReassignOpenPRs
//
// Массово деактивирует пользователей команды и безопасно переназначает ревьюверов
// во всех OPEN PR, где они были назначены.
func (s *TeamService) DeactivateMembersAndReassignOpenPRs(
	ctx context.Context,
	teamName domain.TeamName,
	userIDs []domain.UserID,
) (*BulkDeactivateResult, error) {
	// 1. Получаем команду и участников.
	team, err := s.teams.GetTeamWithMembers(ctx, teamName)
	if err != nil {
		return nil, err
	}

	// 2. Определяем, кого деактивировать
	var targets []domain.UserID
	if len(userIDs) == 0 {
		// Деактивируем всех активных.
		for _, m := range team.Members {
			if m.IsActive {
				targets = append(targets, m.UserID)
			}
		}
	} else {
		// Деактивируем только пересечение (только тех, кто реально в этой команде).
		memberSet := make(map[domain.UserID]bool, len(team.Members))
		for _, m := range team.Members {
			memberSet[m.UserID] = m.IsActive // внимание на isActive
		}
		for _, id := range userIDs {
			if memberSet[id] {
				targets = append(targets, id)
			}
		}
	}

	if len(targets) == 0 {
		// Никого деактивировать не надо — можно вернуть пустой результат.
		return &BulkDeactivateResult{
			TeamName:              teamName,
			DeactivatedUserIDs:    nil,
			UpdatedPullRequestIDs: nil,
		}, nil
	}

	// 3. Деактивируем пользователей
	for _, uid := range targets {
		if _, err := s.users.SetIsActive(ctx, uid, false); err != nil {
			return nil, err
		}
	}

	// 4. Собираем множество деактивированных для быстрых проверок.
	targetsSet := make(map[domain.UserID]struct{}, len(targets))
	for _, id := range targets {
		targetsSet[id] = struct{}{}
	}

	// 5. Собираем множество PR, где эти пользователи были ревьюверами.
	prIDSet := make(map[domain.PullRequestID]struct{})
	for _, uid := range targets {
		prs, err := s.prs.ListByReviewer(ctx, uid)
		if err != nil {
			return nil, err
		}
		for _, p := range prs {
			prIDSet[p.ID] = struct{}{}
		}
	}

	if len(prIDSet) == 0 {
		// Никто из деактивированных не участвовал в OPEN PR.
		return &BulkDeactivateResult{
			TeamName:              teamName,
			DeactivatedUserIDs:    targets,
			UpdatedPullRequestIDs: nil,
		}, nil
	}

	// 6. Обновляем данные команды после деактивации, чтобы получить актуальных активных членов.
	teamAfter, err := s.teams.GetTeamWithMembers(ctx, teamName)
	if err != nil {
		return nil, err
	}

	// Список активных кандидатов
	var activeCandidates []domain.UserID
	for _, m := range teamAfter.Members {
		if m.IsActive {
			activeCandidates = append(activeCandidates, m.UserID)
		}
	}

	// 7. Для каждого PR пересобираем список ревьюверов
	updatedPRs := make([]domain.PullRequestID, 0, len(prIDSet))

	for prID := range prIDSet {
		pr, err := s.prs.GetByID(ctx, prID)
		if err != nil {
			// Если PR удалён или не найден — пропускаем
			if err == domain.ErrNotFound {
				continue
			}
			return nil, err
		}

		// Менять список можно только для OPEN.
		if !pr.CanModifyReviewers() {
			continue
		}

		originalReviewers := append([]domain.UserID(nil), pr.Reviewers...)

		newReviewers := make([]domain.UserID, 0, len(pr.Reviewers))

		//Мы не знаем заранее, кто должен быть заменён — просто:
		//- если ревьювер не деактивирован то оставляем;
		//- если деактивирован то пробуем подобрать кандидата;
		for _, r := range originalReviewers {
			if _, toDeactivate := targetsSet[r]; !toDeactivate {
				// Ревьювер остаётся.
				newReviewers = append(newReviewers, r)
				continue
			}

			// Пытаемся подобрать замену.
			candidate, ok := pickCandidateForPR(pr, newReviewers, activeCandidates)
			if ok {
				newReviewers = append(newReviewers, candidate)
			}
			// если кандидата нет — мы просто не добавляем никого,
			// тем самым уменьшив кол-во ревьюверов
		}

		// Ограничиваем количеством MaxReviewersPerPR
		if len(newReviewers) > domain.MaxReviewersPerPR {
			newReviewers = newReviewers[:domain.MaxReviewersPerPR]
		}

		// Обновляем доменный объект
		if err := pr.SetReviewers(newReviewers); err != nil {
			// Если параллельно кто-то замёржил PR — просто пропускаем.
			if err == domain.ErrReviewersLocked {
				continue
			}
			return nil, err
		}

		// Сохраняем в БД.
		if err := s.prs.UpdateReviewers(ctx, pr.ID, pr.Reviewers); err != nil {
			return nil, err
		}

		updatedPRs = append(updatedPRs, pr.ID)
	}

	return &BulkDeactivateResult{
		TeamName:              teamName,
		DeactivatedUserIDs:    targets,
		UpdatedPullRequestIDs: updatedPRs,
	}, nil
}

// pickCandidateForPR — выбирает случайного кандидата для PR
// из списка активных кандидатов, исключая автора и уже выбранных ревьюверов.
func pickCandidateForPR(
	pr *domain.PullRequest,
	currentReviewers []domain.UserID,
	candidates []domain.UserID,
) (domain.UserID, bool) {
	if len(candidates) == 0 {
		return "", false
	}

	// Перемешиваем кандидатов и идём по ним по порядку,
	// чтобы не дёргать rand.Intn в цикле и не зависеть от порядка.
	indexes := rand.Perm(len(candidates))

	for _, idx := range indexes {
		c := candidates[idx]
		if c == pr.AuthorID {
			continue
		}
		if containsUserID(currentReviewers, c) {
			continue
		}
		return c, true
	}

	return "", false
}

func containsUserID(list []domain.UserID, id domain.UserID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}
