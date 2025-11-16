package httptransport

import (
	"encoding/json"
	"net/http"

	"PullRequestAssigner/internal/domain"
)

// DTO пользователя для ответов.
type userDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type setIsActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type userResponse struct {
	User userDTO `json:"user"`
}

// /users/setIsActive
func (h *Handler) handleUserSetIsActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req setIsActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}

	u, err := h.userSvc.SetIsActive(r.Context(), domain.UserID(req.UserID), req.IsActive)
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := userResponse{
		User: userToDTO(u),
	}
	writeJSON(w, http.StatusOK, resp)
}

// /users/getReview
// Возвращает PR, где пользователь назначен ревьювером.
type userGetReviewResponse struct {
	UserID       string                `json:"user_id"`
	PullRequests []pullRequestShortDTO `json:"pull_requests"`
}

func (h *Handler) handleUserGetReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}

	prs, err := h.prSvc.ListPRsByReviewer(r.Context(), domain.UserID(userID))
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := userGetReviewResponse{
		UserID:       userID,
		PullRequests: make([]pullRequestShortDTO, 0, len(prs)),
	}

	for _, p := range prs {
		resp.PullRequests = append(resp.PullRequests, prShortToDTO(p))
	}

	writeJSON(w, http.StatusOK, resp)
}

// helper: domain.User - userDTO
func userToDTO(u *domain.User) userDTO {
	return userDTO{
		UserID:   string(u.ID),
		Username: u.Username,
		TeamName: string(u.TeamName),
		IsActive: u.IsActive,
	}
}
