package httptransport

import (
	"encoding/json"
	"net/http"
	"time"

	"PullRequestAssigner/internal/domain"
)

// DTO полного PR
type pullRequestDTO struct {
	ID                string     `json:"pull_request_id"`
	Name              string     `json:"pull_request_name"`
	AuthorID          string     `json:"author_id"`
	Status            string     `json:"status"`
	AssignedReviewers []string   `json:"assigned_reviewers"`
	CreatedAt         *time.Time `json:"createdAt,omitempty"`
	MergedAt          *time.Time `json:"mergedAt,omitempty"`
}

// DTO короткого PR
type pullRequestShortDTO struct {
	ID       string `json:"pull_request_id"`
	Name     string `json:"pull_request_name"`
	AuthorID string `json:"author_id"`
	Status   string `json:"status"`
}

type prResponse struct {
	PR pullRequestDTO `json:"pr"`
}

type prReassignResponse struct {
	PR         pullRequestDTO `json:"pr"`
	ReplacedBy string         `json:"replaced_by"`
}

// /pullRequest/create

type createPRRequest struct {
	ID       string `json:"pull_request_id"`
	Name     string `json:"pull_request_name"`
	AuthorID string `json:"author_id"`
}

func (h *Handler) handlePullRequestCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req createPRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.ID == "" || req.Name == "" || req.AuthorID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "pull_request_id, pull_request_name and author_id are required")
		return
	}

	pr, err := h.prSvc.CreatePR(
		r.Context(),
		domain.PullRequestID(req.ID),
		req.Name,
		domain.UserID(req.AuthorID),
	)
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := prResponse{
		PR: prToDTO(pr),
	}
	writeJSON(w, http.StatusCreated, resp)
}

// /pullRequest/merge

type mergePRRequest struct {
	ID string `json:"pull_request_id"`
}

func (h *Handler) handlePullRequestMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req mergePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "pull_request_id is required")
		return
	}

	pr, err := h.prSvc.MergePR(r.Context(), domain.PullRequestID(req.ID))
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := prResponse{
		PR: prToDTO(pr),
	}
	writeJSON(w, http.StatusOK, resp)
}

// /pullRequest/reassign

type reassignPRRequest struct {
	ID        string `json:"pull_request_id"`
	OldUserID string `json:"old_user_id"`
}

func (h *Handler) handlePullRequestReassign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req reassignPRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.ID == "" || req.OldUserID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "pull_request_id and old_user_id are required")
		return
	}

	pr, newReviewer, err := h.prSvc.ReassignReviewer(
		r.Context(),
		domain.PullRequestID(req.ID),
		domain.UserID(req.OldUserID),
	)
	if err != nil {
		status, code, msg := mapError(err)
		writeError(w, status, code, msg)
		return
	}

	resp := prReassignResponse{
		PR:         prToDTO(pr),
		ReplacedBy: string(newReviewer),
	}
	writeJSON(w, http.StatusOK, resp)
}

// helpers

func prToDTO(pr *domain.PullRequest) pullRequestDTO {
	dto := pullRequestDTO{
		ID:                string(pr.ID),
		Name:              pr.Name,
		AuthorID:          string(pr.AuthorID),
		Status:            string(pr.Status),
		AssignedReviewers: make([]string, 0, len(pr.Reviewers)),
		CreatedAt:         pr.CreatedAt,
		MergedAt:          pr.MergedAt,
	}

	for _, r := range pr.Reviewers {
		dto.AssignedReviewers = append(dto.AssignedReviewers, string(r))
	}

	return dto
}

func prShortToDTO(p domain.PullRequestShort) pullRequestShortDTO {
	return pullRequestShortDTO{
		ID:       string(p.ID),
		Name:     p.Name,
		AuthorID: string(p.AuthorID),
		Status:   string(p.Status),
	}
}
