package httptransport

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"PullRequestAssigner/internal/domain"
	"PullRequestAssigner/internal/service"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// withMiddlewares навешивает middleware-цепочку.
func withMiddlewares(next http.Handler) http.Handler {
	return loggingMiddleware(recoveryMiddleware(next))
}

// recoveryMiddleware ловит паники и отдаёт 500.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware логирует запросы.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		start := time.Now()
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %s %d %s",
			r.RemoteAddr, r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

// writeJSON утилита для JSON-ответов.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// writeError — обёртка над ErrorResponse.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	resp := errorResponse{
		Error: errorBody{
			Code:    code,
			Message: msg,
		},
	}
	writeJSON(w, status, resp)
}

// mapError переводит доменные/сервисные ошибки в HTTP-код и ErrorResponse.code
func mapError(err error) (status int, code, msg string) {
	if err == nil {
		return http.StatusOK, "", ""
	}

	switch {
	case errors.Is(err, domain.ErrTeamExists):
		return http.StatusBadRequest, "TEAM_EXISTS", err.Error()
	case errors.Is(err, domain.ErrPullRequestExists):
		return http.StatusConflict, "PR_EXISTS", err.Error()
	case errors.Is(err, domain.ErrReviewersLocked):
		return http.StatusConflict, "PR_MERGED", err.Error()
	case errors.Is(err, domain.ErrReviewerNotFound):
		return http.StatusConflict, "NOT_ASSIGNED", err.Error()
	case errors.Is(err, service.ErrNoCandidate):
		return http.StatusConflict, "NO_CANDIDATE", err.Error()
	case errors.Is(err, domain.ErrNotFound):
		// единый код NOT_FOUND для всех не найдены
		return http.StatusNotFound, "NOT_FOUND", "resource not found"
	default:
		log.Printf("unhandled error: %v", err)
		return http.StatusInternalServerError, "INTERNAL", "internal server error"
	}
}
