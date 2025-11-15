package domain

import (
	"errors"
	"strings"
)

// Тип идентификатора команды.
type TeamName string

type TeamMember struct {
	UserID   UserID
	Username string
	IsActive bool
}

type Team struct {
	Name    TeamName
	Members []TeamMember
}

var (
	ErrEmptyTeamName = errors.New("team name cannot be empty")
	ErrTeamExists    = errors.New("team with this name already exists")
	ErrNotFound      = errors.New("not found")
)

// NewTeam создаёт команду без ID, репозиторий потом подставит ID.
func NewTeam(name TeamName) (*Team, error) {
	name = TeamName(strings.TrimSpace(string(name)))
	if name == "" {
		return nil, ErrEmptyTeamName
	}

	return &Team{
		Name: name,
	}, nil
}

// Переименование команды с валидацией.
func (t *Team) Rename(newName TeamName) error {
	newName = TeamName(strings.TrimSpace(string(newName)))
	if newName == "" {
		return ErrEmptyTeamName
	}
	t.Name = newName
	return nil
}

// SetMembers полностью заменяет состав команды.
func (t *Team) SetMembers(members []TeamMember) {
	t.Members = append([]TeamMember(nil), members...)
}
