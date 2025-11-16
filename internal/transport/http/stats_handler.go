package httptransport

import (
	"net/http"
)

// DTO для статистики по пользователям.
type statsByUserDTO struct {
	UserID             string `json:"user_id"`
	AssignedAsReviewer int    `json:"assigned_as_reviewer"`
	Authored           int    `json:"authored"`
}

// DTO для статистики по PR.
type statsByPRDTO struct {
	PullRequestID string `json:"pull_request_id"`
	ReviewerCount int    `json:"reviewer_count"`
}

// Полный ответ /stats.
type statsResponse struct {
	ByUser []statsByUserDTO `json:"by_user"`
	ByPR   []statsByPRDTO   `json:"by_pr"`
}

// GET /stats
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if h.statsSvc == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "stats service not configured")
		return
	}

	snapshot, err := h.statsSvc.GetSnapshot(r.Context())
	if err != nil {
		status, code, msg := mapError(err) // упадёт в INTERNAL 500 по дефолту
		writeError(w, status, code, msg)
		return
	}

	resp := statsResponse{
		ByUser: make([]statsByUserDTO, 0, len(snapshot.ByUser)),
		ByPR:   make([]statsByPRDTO, 0, len(snapshot.ByPR)),
	}

	for _, u := range snapshot.ByUser {
		resp.ByUser = append(resp.ByUser, statsByUserDTO{
			UserID:             string(u.UserID),
			AssignedAsReviewer: u.AssignedAsReviewer,
			Authored:           u.Authored,
		})
	}

	for _, p := range snapshot.ByPR {
		resp.ByPR = append(resp.ByPR, statsByPRDTO{
			PullRequestID: string(p.PRID),
			ReviewerCount: p.ReviewerCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
