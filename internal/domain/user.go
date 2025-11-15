package domain

import (
	"errors"
	"strings"
)

// UserID — отдельный тип, чтобы не путать с другими ID.
type UserID string

type User struct {
	ID       UserID
	Username string
	TeamName TeamName
	IsActive bool
}

var (
	ErrEmptyUserID   = errors.New("user ID cannot be empty")
	ErrEmptyUserName = errors.New("user name cannot be empty")
)

// NewUser создаёт активного пользователя.
// ID обычно проставит репозиторий после вставки в БД.
func NewUser(id UserID, username string, teamName TeamName, isActive bool) (*User, error) {
	username = strings.TrimSpace(username)
	if id == "" {
		return nil, ErrEmptyUserID
	}
	if username == "" {
		return nil, ErrEmptyUserName
	}

	return &User{
		ID:       id,
		Username: username,
		TeamName: teamName,
		IsActive: true,
	}, nil
}

// Rename меняет имя пользователя с валидацией.
func (u *User) Rename(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return ErrEmptyUserName
	}
	u.Username = newName
	return nil
}

// Activate/Deactivate инкапсулируют управление флагом активности.

func (u *User) Activate() {
	u.IsActive = true
}

func (u *User) Deactivate() {
	u.IsActive = false
}

// Геттер для проверки активности
func (u User) Active() bool {
	return u.IsActive
}

func (u User) ToTeamMember() TeamMember {
	return TeamMember{
		UserID:   u.ID,
		Username: u.Username,
		IsActive: u.IsActive,
	}
}
