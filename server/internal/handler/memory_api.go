package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/devioslang/memorix/server/internal/domain"
)

// memoryAPIHandler handles memory management API endpoints.
// These endpoints are user-centric (not tenant-centric) for easier integration.

// listUserFacts handles GET /api/memory/{user_id}/facts
// Returns all facts for a specific user.
func (s *Server) listUserFacts(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	filter := domain.UserProfileFactFilter{
		UserID:   userID,
		Category: domain.FactCategory(q.Get("category")),
		Limit:    limit,
		Offset:   offset,
	}

	facts, total, err := svc.ListFacts(r.Context(), filter)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if facts == nil {
		facts = []domain.UserProfileFact{}
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"facts":   facts,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// listUserSummaries handles GET /api/memory/{user_id}/summaries
// Returns all conversation summaries for a specific user.
func (s *Server) listUserSummaries(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	auth := authInfo(r)
	svc := s.resolveSummarizerServices(auth)

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	filter := domain.ConversationSummaryFilter{
		UserID:   userID,
		KeyTopic: q.Get("topic"),
		Limit:    limit,
		Offset:   offset,
	}

	summaries, total, err := svc.ListSummaries(r.Context(), filter)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if summaries == nil {
		summaries = []domain.ConversationSummary{}
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"user_id":   userID,
		"summaries": summaries,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// searchUserExperiencesRequest is the request body for experience search.
type searchUserExperiencesRequest struct {
	Query    string   `json:"query"`
	Topic    string   `json:"topic,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	MinScore float64  `json:"min_score,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// searchUserExperiences handles GET /api/memory/{user_id}/experiences
// Performs semantic search on user experiences.
func (s *Server) searchUserExperiences(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	auth := authInfo(r)
	svc := s.resolveExperienceServices(auth)
	if svc == nil {
		respondError(w, http.StatusServiceUnavailable, "experience service not configured")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	minScore := 0.3
	if ms := q.Get("min_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 64); err == nil {
			minScore = parsed
		}
	}

	filter := domain.ExperienceFilter{
		UserID:   userID,
		Query:    query,
		Topic:    q.Get("topic"),
		Tags:     q["tag"],
		MinScore: minScore,
		Limit:    limit,
	}

	result, err := svc.Search(r.Context(), filter)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"user_id":     userID,
		"query":       query,
		"experiences": result.Experiences,
		"total":       result.Total,
		"latency_ms":  result.LatencyMs,
	})
}

// deleteUserFact handles DELETE /api/memory/{user_id}/facts/{fact_id}
// Deletes a specific fact for a user.
func (s *Server) deleteUserFact(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	factID := chi.URLParam(r, "fact_id")
	if userID == "" || factID == "" {
		respondError(w, http.StatusBadRequest, "user_id and fact_id are required")
		return
	}

	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)

	// Verify the fact belongs to the user
	fact, err := svc.GetFact(r.Context(), factID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if fact.UserID != userID {
		respondError(w, http.StatusForbidden, "fact does not belong to this user")
		return
	}

	if err := svc.DeleteFact(r.Context(), factID); err != nil {
		s.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// triggerUserGC handles POST /api/memory/{user_id}/gc
// Triggers garbage collection for a user's memories.
func (s *Server) triggerUserGC(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	q := r.URL.Query()
	dryRun := q.Get("dry_run") == "true"

	auth := authInfo(r)
	svc := s.resolveGCServices(auth)

	// Run GC
	result, err := svc.Run(r.Context(), auth.TenantID, dryRun)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"user_id":  userID,
		"gc_result": result,
	})
}

// getUserMemoryStats handles GET /api/memory/{user_id}/stats
// Returns memory usage statistics for a user.
func (s *Server) getUserMemoryStats(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	auth := authInfo(r)
	statsSvc := s.resolveStatsServices(auth)

	stats, err := statsSvc.GetStats(r.Context(), userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, stats)
}

// getUserMemoryOverview handles GET /api/memory/{user_id}/overview
// Returns a complete memory overview for a user.
func (s *Server) getUserMemoryOverview(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	q := r.URL.Query()
	includeContent := q.Get("include_content") == "true"

	auth := authInfo(r)
	statsSvc := s.resolveStatsServices(auth)

	overview, err := statsSvc.GetOverview(r.Context(), userID, includeContent)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, overview)
}

// serveDashboard serves the memory dashboard web UI.
func (s *Server) serveDashboard(w http.ResponseWriter, r *http.Request) {
	// Serve embedded or static web UI
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Memorix Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1a1a2e; color: #eee; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: #00d9ff; margin-bottom: 20px; }
        .card { background: #16213e; border-radius: 8px; padding: 20px; margin-bottom: 20px; }
        .card h2 { color: #00d9ff; margin-bottom: 15px; font-size: 18px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; }
        .stat { background: #0f3460; padding: 15px; border-radius: 6px; }
        .stat-value { font-size: 28px; font-weight: bold; color: #00d9ff; }
        .stat-label { color: #888; font-size: 12px; }
        input, button { padding: 10px; border-radius: 4px; border: none; }
        input { background: #0f3460; color: #fff; width: 300px; }
        button { background: #00d9ff; color: #1a1a2e; cursor: pointer; font-weight: bold; }
        button:hover { background: #00b8d9; }
        .api-list { font-family: monospace; font-size: 13px; }
        .api-list li { padding: 8px 0; border-bottom: 1px solid #0f3460; }
        .api-list code { color: #00d9ff; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧠 Memorix Memory Dashboard</h1>
        
        <div class="card">
            <h2>📊 Memory Statistics</h2>
            <p style="color:#888;margin-bottom:15px;">Enter a user ID to view memory stats</p>
            <input type="text" id="userId" placeholder="User ID">
            <button onclick="loadStats()">Load Stats</button>
            <div id="stats" class="stats" style="margin-top:20px;"></div>
        </div>
        
        <div class="card">
            <h2>🔌 Available APIs</h2>
            <ul class="api-list">
                <li><code>GET /api/memory/{user_id}/facts</code> - List user facts</li>
                <li><code>GET /api/memory/{user_id}/summaries</code> - List conversation summaries</li>
                <li><code>GET /api/memory/{user_id}/experiences?q=xxx</code> - Semantic search</li>
                <li><code>DELETE /api/memory/{user_id}/facts/{fact_id}</code> - Delete fact</li>
                <li><code>POST /api/memory/{user_id}/gc?dry_run=true</code> - Trigger GC</li>
                <li><code>GET /api/memory/{user_id}/stats</code> - Memory statistics</li>
                <li><code>GET /api/debug/context/{session_id}</code> - Context diagnostics</li>
            </ul>
        </div>
    </div>
    <script>
    async function loadStats() {
        const userId = document.getElementById('userId').value;
        if (!userId) return alert('Please enter a user ID');
        try {
            const resp = await fetch('/api/memory/' + userId + '/stats');
            const data = await resp.json();
            document.getElementById('stats').innerHTML = 
                '<div class="stat"><div class="stat-value">' + (data.facts_count || 0) + '</div><div class="stat-label">Facts</div></div>' +
                '<div class="stat"><div class="stat-value">' + (data.summaries_count || 0) + '</div><div class="stat-label">Summaries</div></div>' +
                '<div class="stat"><div class="stat-value">' + (data.experiences_count || 0) + '</div><div class="stat-label">Experiences</div></div>' +
                '<div class="stat"><div class="stat-value">' + (data.total_tokens || 0) + '</div><div class="stat-label">Total Tokens</div></div>';
        } catch(e) { alert('Error: ' + e.message); }
    }
    </script>
</body>
</html>
`))
}
