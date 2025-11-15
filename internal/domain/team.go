package domain

import (
	"errors"
	"strings"
)

// Тип идентификатора команды.
type TeamID int64

type Team struct {
	ID   TeamID
	Name string
}

var (
	ErrEmptyTeamName = errors.New("team name cannot be empty")
)

// NewTeam создаёт команду без ID, репозиторий потом подставит ID.
func NewTeam(name string) (*Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyTeamName
	}

	return &Team{
		Name: name,
	}, nil
}

// Переименование команды с валидацией.
func (t *Team) Rename(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return ErrEmptyTeamName
	}
	t.Name = newName
	return nil
}

// Тип для членства в команде
type TeamMember struct {
	TeamID TeamID
	UserID UserID
}
