package service

import (
	"context"

	"PullRequestAssigner/internal/domain"
)

// UserRepository то, что нужно сервису от слоя хранилища.
type UserRepository interface {
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)
	SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (*domain.User, error)
}

// UserService бизнес-логика вокруг пользователей.
type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users: users}
}

// Установка флага активности пользователя
// Используется для /users/setIsActive.
func (s *UserService) SetIsActive(
	ctx context.Context,
	userID domain.UserID,
	isActive bool,
) (*domain.User, error) {
	return s.users.SetIsActive(ctx, userID, isActive)
}

// Просто обёртка над репозиторием
func (s *UserService) GetUser(
	ctx context.Context,
	userID domain.UserID,
) (*domain.User, error) {
	return s.users.GetByID(ctx, userID)
}
