package httptransport

import (
	"encoding/json"
	"net/http"

	"PullRequestAssigner/internal/domain"
)

// DTO для TeamMember из openapi.yaml.
type teamMemberDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// DTO для Team из openapi.yaml.
type teamDTO struct {
	TeamName string          `json:"team_name"`
	Members  []teamMemberDTO `json:"members"`
}

// ответ для /team/add
type teamAddResponse struct {
	Team teamDTO `json:"team"`
}

func (h *Handler) handleTeamAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req teamDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.TeamName == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "team_name is required")
		return
	}

	// маппим участников
	members := make([]domain.TeamMember, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, domain.TeamMember{
			UserID:   domain.UserID(m.UserID),
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	team, err := h.teamSvc.CreateTeamWithMembers(
		r.Context(),
		domain.TeamName(req.TeamName),
		members,
	)
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := teamAddResponse{
		Team: teamToDTO(team),
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) handleTeamGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "team_name is required")
		return
	}

	team, err := h.teamSvc.GetTeam(r.Context(), domain.TeamName(teamName))
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := teamToDTO(team)
	writeJSON(w, http.StatusOK, resp)
}

func teamToDTO(t *domain.Team) teamDTO {
	dto := teamDTO{
		TeamName: string(t.Name),
		Members:  make([]teamMemberDTO, 0, len(t.Members)),
	}
	for _, m := range t.Members {
		dto.Members = append(dto.Members, teamMemberDTO{
			UserID:   string(m.UserID),
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}
	return dto
}

// DTO для запроса /team/deactivateMembers

type deactivateMembersRequest struct {
	TeamName string   `json:"team_name"`
	UserIDs  []string `json:"user_ids"`
}

type deactivateMembersResponse struct {
	TeamName              string   `json:"team_name"`
	DeactivatedUserIDs    []string `json:"deactivated_user_ids"`
	UpdatedPullRequestIDs []string `json:"updated_pull_request_ids"`
}

// POST /team/deactivateMembers
func (h *Handler) handleTeamDeactivateMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req deactivateMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.TeamName == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "team_name is required")
		return
	}

	var userIDs []domain.UserID
	for _, id := range req.UserIDs {
		if id == "" {
			continue
		}
		userIDs = append(userIDs, domain.UserID(id))
	}

	res, err := h.teamSvc.DeactivateMembersAndReassignOpenPRs(
		r.Context(),
		domain.TeamName(req.TeamName),
		userIDs,
	)
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := deactivateMembersResponse{
		TeamName:              string(res.TeamName),
		DeactivatedUserIDs:    make([]string, 0, len(res.DeactivatedUserIDs)),
		UpdatedPullRequestIDs: make([]string, 0, len(res.UpdatedPullRequestIDs)),
	}

	for _, id := range res.DeactivatedUserIDs {
		resp.DeactivatedUserIDs = append(resp.DeactivatedUserIDs, string(id))
	}
	for _, prID := range res.UpdatedPullRequestIDs {
		resp.UpdatedPullRequestIDs = append(resp.UpdatedPullRequestIDs, string(prID))
	}

	writeJSON(w, http.StatusOK, resp)
}
