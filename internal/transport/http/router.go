package httptransport

import (
	"net/http"

	"PullRequestAssigner/internal/service"
	"PullRequestAssigner/internal/stats"
)

type Handler struct {
	userSvc  *service.UserService
	teamSvc  *service.TeamService
	prSvc    *service.PRService
	statsSvc *stats.Service
}

// NewRouter регистрирует все эндпоинты и навешивает middleware.
func NewRouter(
	userSvc *service.UserService,
	teamSvc *service.TeamService,
	prSvc *service.PRService,
	statsSvc *stats.Service,
) http.Handler {
	h := &Handler{
		userSvc:  userSvc,
		teamSvc:  teamSvc,
		prSvc:    prSvc,
		statsSvc: statsSvc,
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/health", h.handleHealth)

	// Teams
	mux.HandleFunc("/team/add", h.handleTeamAdd)
	mux.HandleFunc("/team/get", h.handleTeamGet)
	mux.HandleFunc("/team/deactivateMembers", h.handleTeamDeactivateMembers)

	// Users
	mux.HandleFunc("/users/setIsActive", h.handleUserSetIsActive)
	mux.HandleFunc("/users/getReview", h.handleUserGetReview)

	// Pull Requests
	mux.HandleFunc("/pullRequest/create", h.handlePullRequestCreate)
	mux.HandleFunc("/pullRequest/merge", h.handlePullRequestMerge)
	mux.HandleFunc("/pullRequest/reassign", h.handlePullRequestReassign)

	// Stats
	mux.HandleFunc("/stats", h.handleStats)
	return withMiddlewares(mux)
}

// простой health-check
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	resp := map[string]string{"status": "ok"}
	writeJSON(w, http.StatusOK, resp)
}
