package domain

import (
	"errors"
	"strings"
)

// UserID — отдельный тип, чтобы не путать с другими ID.
type UserID int64

type User struct {
	ID       UserID
	Name     string
	IsActive bool
}

var (
	ErrEmptyUserName = errors.New("user name cannot be empty")
)

// NewUser создаёт активного пользователя.
// ID обычно проставит репозиторий после вставки в БД.
func NewUser(name string) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyUserName
	}

	return &User{
		Name:     name,
		IsActive: true,
	}, nil
}

// Rename меняет имя пользователя с валидацией.
func (u *User) Rename(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return ErrEmptyUserName
	}
	u.Name = newName
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
