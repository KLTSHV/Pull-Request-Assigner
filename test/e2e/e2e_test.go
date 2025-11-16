package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

type teamMemberDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type teamDTO struct {
	TeamName string          `json:"team_name"`
	Members  []teamMemberDTO `json:"members"`
}

type teamAddResponse struct {
	Team teamDTO `json:"team"`
}

type pullRequestDTO struct {
	ID                string   `json:"pull_request_id"`
	Name              string   `json:"pull_request_name"`
	AuthorID          string   `json:"author_id"`
	Status            string   `json:"status"`
	AssignedReviewers []string `json:"assigned_reviewers"`
}

type createPRResponse struct {
	PR pullRequestDTO `json:"pr"`
}

type mergePRResponse struct {
	PR pullRequestDTO `json:"pr"`
}

type userGetReviewResponse struct {
	UserID       string           `json:"user_id"`
	PullRequests []pullRequestDTO `json:"pull_requests"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func doJSON(t *testing.T, method, path string, body any, expectedStatus int, out any) {
	t.Helper()

	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		buf = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, buf)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		// попробуем распарсить ошибку
		var er errorResponse
		_ = json.Unmarshal(respBody, &er)
		t.Fatalf("unexpected status %d (want %d). body=%s parsed_error=%+v",
			resp.StatusCode, expectedStatus, string(respBody), er)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			t.Fatalf("failed to unmarshal response: %v, body=%s", err, string(respBody))
		}
	}
}

func TestEndToEnd_CreateTeam_PR_Assign_Merge_ReassignOnMerged(t *testing.T) {
	// ждём, пока сервис поднимется
	waitForHealth(t)

	// генерируем уникальные ID, чтобы тест можно было гонять многократно
	suffix := time.Now().UnixNano()
	teamName := fmt.Sprintf("e2e-backend-%d", suffix)
	prID := fmt.Sprintf("e2e-pr-%d", suffix)

	// 1. Создаём команду с 4 участниками
	var teamResp teamAddResponse
	doJSON(t, http.MethodPost, "/team/add", map[string]any{
		"team_name": teamName,
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Carol", "is_active": true},
			{"user_id": "u4", "username": "Dave", "is_active": true},
		},
	}, http.StatusCreated, &teamResp)

	if teamResp.Team.TeamName != teamName {
		t.Fatalf("unexpected team_name: %s", teamResp.Team.TeamName)
	}
	if len(teamResp.Team.Members) != 4 {
		t.Fatalf("expected 4 members, got %d", len(teamResp.Team.Members))
	}

	// 2. Создаём PR от u1 и проверяем ревьюверов
	var prCreateResp createPRResponse
	doJSON(t, http.MethodPost, "/pullRequest/create", map[string]any{
		"pull_request_id":   prID,
		"pull_request_name": "E2E Feature",
		"author_id":         "u1",
	}, http.StatusCreated, &prCreateResp)

	if prCreateResp.PR.ID != prID {
		t.Fatalf("unexpected pr id: %s", prCreateResp.PR.ID)
	}
	if prCreateResp.PR.AuthorID != "u1" {
		t.Fatalf("unexpected author_id: %s", prCreateResp.PR.AuthorID)
	}
	if prCreateResp.PR.Status != "OPEN" {
		t.Fatalf("expected status OPEN, got %s", prCreateResp.PR.Status)
	}
	if len(prCreateResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected at least 1 reviewer")
	}
	if len(prCreateResp.PR.AssignedReviewers) > 2 {
		t.Fatalf("expected at most 2 reviewers, got %d", len(prCreateResp.PR.AssignedReviewers))
	}
	for _, r := range prCreateResp.PR.AssignedReviewers {
		if r == "u1" {
			t.Fatalf("author should not be assigned as reviewer")
		}
	}

	// 3. Проверяем, что первый ревьювер видит PR в /users/getReview
	firstReviewer := prCreateResp.PR.AssignedReviewers[0]
	var reviewResp userGetReviewResponse
	doJSON(t, http.MethodGet, "/users/getReview?user_id="+firstReviewer, nil, http.StatusOK, &reviewResp)

	found := false
	for _, p := range reviewResp.PullRequests {
		if p.ID == prID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("user %s should have PR %s in review list", firstReviewer, prID)
	}

	// 4. Merge PR - ожидаем MERGED
	var mergeResp mergePRResponse
	doJSON(t, http.MethodPost, "/pullRequest/merge", map[string]any{
		"pull_request_id": prID,
	}, http.StatusOK, &mergeResp)

	if mergeResp.PR.Status != "MERGED" {
		t.Fatalf("expected status MERGED after merge, got %s", mergeResp.PR.Status)
	}

	// 5. Пытаемся переназначить ревьювера у MERGED PR - 409 PR_MERGED
	var errResp errorResponse
	doJSON(t, http.MethodPost, "/pullRequest/reassign", map[string]any{
		"pull_request_id": prID,
		"old_user_id":     firstReviewer,
	}, http.StatusConflict, &errResp)

	if errResp.Error.Code != "PR_MERGED" {
		t.Fatalf("expected error code PR_MERGED, got %+v", errResp)
	}
}

// Ожидаем, пока /health начнёт отвечать 200.
// Если сервис не поднят, тест быстро и понятно упадёт.
func waitForHealth(t *testing.T) {
	t.Helper()

	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("service is not healthy on %s within timeout", baseURL)
}
