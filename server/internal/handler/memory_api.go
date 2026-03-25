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
<html lang="en">
<head>
    <title>Memorix Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root {
            --bg-primary: #1a1a2e;
            --bg-secondary: #16213e;
            --bg-tertiary: #0f3460;
            --accent: #00d9ff;
            --accent-hover: #00b8d9;
            --text-primary: #eee;
            --text-secondary: #888;
            --success: #4ade80;
            --warning: #fbbf24;
            --danger: #f87171;
            --border: #2a2a4a;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg-primary); color: var(--text-primary); min-height: 100vh; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        h1 { color: var(--accent); margin-bottom: 8px; font-size: 28px; }
        .subtitle { color: var(--text-secondary); margin-bottom: 24px; }

        /* Header bar */
        .header-bar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 24px; }
        .header-bar .left { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
        .header-bar .right { display: flex; align-items: center; gap: 8px; }

        /* Input styles */
        input[type="text"], input[type="search"], textarea, select {
            padding: 10px 14px;
            border-radius: 6px;
            border: 1px solid var(--border);
            background: var(--bg-tertiary);
            color: var(--text-primary);
            font-size: 14px;
        }
        input:focus, textarea:focus, select:focus { outline: none; border-color: var(--accent); }
        input::placeholder { color: var(--text-secondary); }

        /* Button styles */
        button {
            padding: 10px 16px;
            border-radius: 6px;
            border: none;
            cursor: pointer;
            font-weight: 500;
            font-size: 14px;
            display: inline-flex;
            align-items: center;
            gap: 6px;
            transition: all 0.2s;
        }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
        .btn-primary { background: var(--accent); color: var(--bg-primary); }
        .btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
        .btn-secondary { background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border); }
        .btn-secondary:hover:not(:disabled) { background: var(--bg-secondary); }
        .btn-danger { background: var(--danger); color: #fff; }
        .btn-danger:hover:not(:disabled) { background: #ef4444; }
        .btn-small { padding: 6px 12px; font-size: 13px; }

        /* Cards */
        .card { background: var(--bg-secondary); border-radius: 10px; padding: 20px; margin-bottom: 20px; border: 1px solid var(--border); }
        .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
        .card h2 { color: var(--accent); font-size: 18px; }

        /* Stats grid */
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 24px; }
        .stat { background: var(--bg-tertiary); padding: 16px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 32px; font-weight: bold; color: var(--accent); }
        .stat-label { color: var(--text-secondary); font-size: 12px; text-transform: uppercase; margin-top: 4px; }

        /* Memory type tabs */
        .tabs { display: flex; gap: 4px; margin-bottom: 16px; border-bottom: 1px solid var(--border); padding-bottom: 12px; }
        .tab { padding: 8px 16px; border-radius: 6px 6px 0 0; background: transparent; color: var(--text-secondary); border: none; cursor: pointer; font-size: 14px; }
        .tab:hover { color: var(--text-primary); background: var(--bg-tertiary); }
        .tab.active { background: var(--accent); color: var(--bg-primary); font-weight: 500; }

        /* Memory list */
        .memory-list { display: flex; flex-direction: column; gap: 12px; }
        .memory-item { background: var(--bg-tertiary); border-radius: 8px; padding: 16px; border-left: 3px solid var(--accent); transition: transform 0.2s; }
        .memory-item:hover { transform: translateX(4px); }
        .memory-item.pinned { border-left-color: var(--success); }
        .memory-item.insight { border-left-color: var(--accent); }
        .memory-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 8px; }
        .memory-type { font-size: 11px; padding: 3px 8px; border-radius: 4px; font-weight: 500; text-transform: uppercase; }
        .memory-type.pinned { background: rgba(74, 222, 128, 0.2); color: var(--success); }
        .memory-type.insight { background: rgba(0, 217, 255, 0.2); color: var(--accent); }
        .memory-content { color: var(--text-primary); line-height: 1.6; margin-bottom: 12px; word-break: break-word; }
        .memory-meta { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
        .memory-tags { display: flex; gap: 6px; flex-wrap: wrap; }
        .memory-tag { font-size: 11px; padding: 2px 8px; background: var(--bg-secondary); border-radius: 4px; color: var(--text-secondary); }
        .memory-actions { display: flex; gap: 8px; }
        .memory-date { font-size: 12px; color: var(--text-secondary); }
        .memory-empty { text-align: center; padding: 60px 20px; color: var(--text-secondary); }
        .memory-empty svg { width: 64px; height: 64px; margin-bottom: 16px; opacity: 0.5; }

        /* Modal */
        .modal-overlay { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); display: flex; align-items: center; justify-content: center; z-index: 1000; opacity: 0; visibility: hidden; transition: all 0.3s; }
        .modal-overlay.active { opacity: 1; visibility: visible; }
        .modal { background: var(--bg-secondary); border-radius: 12px; padding: 24px; width: 90%; max-width: 500px; max-height: 90vh; overflow-y: auto; transform: scale(0.9); transition: transform 0.3s; border: 1px solid var(--border); }
        .modal-overlay.active .modal { transform: scale(1); }
        .modal h3 { color: var(--accent); margin-bottom: 20px; font-size: 20px; }
        .modal-field { margin-bottom: 16px; }
        .modal-field label { display: block; margin-bottom: 6px; color: var(--text-secondary); font-size: 13px; }
        .modal-field textarea { width: 100%; min-height: 120px; resize: vertical; }
        .modal-field input { width: 100%; }
        .modal-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 24px; }

        /* Progress bar */
        .progress-bar { height: 8px; background: var(--bg-tertiary); border-radius: 4px; overflow: hidden; margin-top: 12px; }
        .progress-fill { height: 100%; background: var(--accent); transition: width 0.3s; }
        .progress-text { font-size: 12px; color: var(--text-secondary); margin-top: 8px; }

        /* Toast */
        .toast { position: fixed; bottom: 24px; right: 24px; padding: 12px 20px; border-radius: 8px; font-size: 14px; z-index: 2000; animation: slideIn 0.3s ease; }
        .toast.success { background: var(--success); color: #000; }
        .toast.error { background: var(--danger); color: #fff; }
        @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }

        /* Loading spinner */
        .spinner { width: 20px; height: 20px; border: 2px solid transparent; border-top-color: currentColor; border-radius: 50%; animation: spin 0.8s linear infinite; }
        @keyframes spin { to { transform: rotate(360deg); } }

        /* Search bar */
        .search-bar { position: relative; width: 300px; }
        .search-bar input { width: 100%; padding-left: 36px; }
        .search-bar svg { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); width: 16px; height: 16px; color: var(--text-secondary); }

        /* Load more */
        .load-more { text-align: center; padding: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Memorix Dashboard</h1>
        <p class="subtitle">Memory Management - Import, Export & Manual CRUD</p>

        <div class="header-bar">
            <div class="left">
                <input type="text" id="tenantId" placeholder="Enter Tenant ID" style="width: 300px;">
                <button class="btn-primary" onclick="loadMemories()">Load Memories</button>
            </div>
            <div class="right">
                <button class="btn-secondary" onclick="openAddModal()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                    Add Memory
                </button>
                <button class="btn-secondary" onclick="openImportModal()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
                    Import
                </button>
                <button class="btn-secondary" onclick="exportMemories()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                    Export
                </button>
            </div>
        </div>

        <div class="stats" id="statsContainer">
            <div class="stat">
                <div class="stat-value" id="totalCount">-</div>
                <div class="stat-label">Total Memories</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="pinnedCount">-</div>
                <div class="stat-label">Saved by You</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="insightCount">-</div>
                <div class="stat-label">Learned from Chats</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="activeCount">-</div>
                <div class="stat-label">Active</div>
            </div>
        </div>

        <div class="card">
            <div class="card-header">
                <h2>Memories</h2>
                <div class="search-bar">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
                    <input type="search" id="searchInput" placeholder="Search memories..." oninput="debounceSearch()">
                </div>
            </div>

            <div class="tabs">
                <button class="tab active" data-type="all" onclick="filterByType('all')">All</button>
                <button class="tab" data-type="pinned" onclick="filterByType('pinned')">Saved by You</button>
                <button class="tab" data-type="insight" onclick="filterByType('insight')">Learned from Chats</button>
            </div>

            <div class="memory-list" id="memoryList">
                <div class="memory-empty">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/></svg>
                    <p>Enter a Tenant ID to view memories</p>
                </div>
            </div>
            <div class="load-more" id="loadMore" style="display: none;">
                <button class="btn-secondary" onclick="loadMoreMemories()">Load More</button>
            </div>
        </div>
    </div>

    <!-- Add Memory Modal -->
    <div class="modal-overlay" id="addModal">
        <div class="modal">
            <h3>Add Memory</h3>
            <div class="modal-field">
                <label>Content *</label>
                <textarea id="addContent" placeholder="Enter memory content..."></textarea>
            </div>
            <div class="modal-field">
                <label>Tags (comma-separated)</label>
                <input type="text" id="addTags" placeholder="tag1, tag2, tag3">
            </div>
            <div class="modal-field">
                <label>Memory Type</label>
                <select id="addType">
                    <option value="pinned">Saved by You (Pinned)</option>
                    <option value="insight">Learned from Chats (Insight)</option>
                </select>
            </div>
            <div class="modal-actions">
                <button class="btn-secondary" onclick="closeAddModal()">Cancel</button>
                <button class="btn-primary" id="addBtn" onclick="createMemory()">Add Memory</button>
            </div>
        </div>
    </div>

    <!-- Edit Memory Modal -->
    <div class="modal-overlay" id="editModal">
        <div class="modal">
            <h3>Edit Memory</h3>
            <p style="color: var(--text-secondary); font-size: 13px; margin-bottom: 16px;">Only pinned memories can be edited.</p>
            <div class="modal-field">
                <label>Content *</label>
                <textarea id="editContent" placeholder="Enter memory content..."></textarea>
            </div>
            <div class="modal-field">
                <label>Tags (comma-separated)</label>
                <input type="text" id="editTags" placeholder="tag1, tag2, tag3">
            </div>
            <input type="hidden" id="editMemoryId">
            <input type="hidden" id="editVersion">
            <div class="modal-actions">
                <button class="btn-secondary" onclick="closeEditModal()">Cancel</button>
                <button class="btn-primary" id="editBtn" onclick="updateMemory()">Save Changes</button>
            </div>
        </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div class="modal-overlay" id="deleteModal">
        <div class="modal">
            <h3>Delete Memory</h3>
            <p style="margin-bottom: 20px;">Are you sure you want to delete this memory? This action cannot be undone.</p>
            <div style="background: var(--bg-tertiary); padding: 12px; border-radius: 6px; margin-bottom: 20px;">
                <p id="deletePreview" style="font-size: 14px; line-height: 1.5;"></p>
            </div>
            <input type="hidden" id="deleteMemoryId">
            <div class="modal-actions">
                <button class="btn-secondary" onclick="closeDeleteModal()">Cancel</button>
                <button class="btn-danger" id="deleteBtn" onclick="deleteMemory()">Delete</button>
            </div>
        </div>
    </div>

    <!-- Import Modal -->
    <div class="modal-overlay" id="importModal">
        <div class="modal">
            <h3>Import Memories</h3>
            <p style="color: var(--text-secondary); font-size: 13px; margin-bottom: 16px;">Upload a JSON file with memories to import.</p>
            <div class="modal-field">
                <label>Agent ID</label>
                <input type="text" id="importAgentId" placeholder="dashboard">
            </div>
            <div class="modal-field">
                <label>JSON File</label>
                <input type="file" id="importFile" accept=".json" style="padding: 8px;">
            </div>
            <div id="importProgress" style="display: none;">
                <div class="progress-bar">
                    <div class="progress-fill" id="importProgressFill" style="width: 0%;"></div>
                </div>
                <p class="progress-text" id="importProgressText">Uploading...</p>
            </div>
            <div class="modal-actions">
                <button class="btn-secondary" onclick="closeImportModal()">Cancel</button>
                <button class="btn-primary" id="importBtn" onclick="importMemories()">Import</button>
            </div>
        </div>
    </div>

    <script>
        // State
        let tenantId = '';
        let memories = [];
        let currentFilter = 'all';
        let searchQuery = '';
        let offset = 0;
        let total = 0;
        let isLoading = false;
        const limit = 50;

        // Toast notification
        function showToast(message, type = 'success') {
            const toast = document.createElement('div');
            toast.className = 'toast ' + type;
            toast.textContent = message;
            document.body.appendChild(toast);
            setTimeout(() => toast.remove(), 3000);
        }

        // Get API base URL
        function getApiBase() {
            tenantId = document.getElementById('tenantId').value.trim();
            if (!tenantId) {
                showToast('Please enter a Tenant ID', 'error');
                return null;
            }
            return '/v1alpha1/memorix/' + encodeURIComponent(tenantId);
        }

        // Load memories
        async function loadMemories(reset = true) {
            const apiBase = getApiBase();
            if (!apiBase) return;

            if (reset) {
                offset = 0;
                memories = [];
            }

            isLoading = true;
            try {
                let url = apiBase + '/memories?limit=' + limit + '&offset=' + offset;
                if (searchQuery) url += '&q=' + encodeURIComponent(searchQuery);
                if (currentFilter !== 'all') url += '&memory_type=' + currentFilter;

                const resp = await fetch(url);
                if (!resp.ok) throw new Error('Failed to load memories: ' + resp.status);

                const data = await resp.json();
                if (reset) {
                    memories = data.memories || [];
                } else {
                    memories = memories.concat(data.memories || []);
                }
                total = data.total || 0;

                renderMemories();
                loadStats();
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            } finally {
                isLoading = false;
            }
        }

        // Load stats
        async function loadStats() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            try {
                // Get counts by making separate requests
                const [totalResp, pinnedResp, insightResp] = await Promise.all([
                    fetch(apiBase + '/memories?limit=1'),
                    fetch(apiBase + '/memories?limit=1&memory_type=pinned'),
                    fetch(apiBase + '/memories?limit=1&memory_type=insight')
                ]);

                const totalData = await totalResp.json();
                const pinnedData = await pinnedResp.json();
                const insightData = await insightResp.json();

                document.getElementById('totalCount').textContent = totalData.total || 0;
                document.getElementById('pinnedCount').textContent = pinnedData.total || 0;
                document.getElementById('insightCount').textContent = insightData.total || 0;
                document.getElementById('activeCount').textContent = totalData.total || 0;
            } catch (e) {
                console.error('Failed to load stats:', e);
            }
        }

        // Render memories
        function renderMemories() {
            const container = document.getElementById('memoryList');
            const loadMoreBtn = document.getElementById('loadMore');

            if (memories.length === 0) {
                container.innerHTML = '<div class="memory-empty"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/></svg><p>No memories found</p></div>';
                loadMoreBtn.style.display = 'none';
                return;
            }

            container.innerHTML = memories.map(m => {
                const date = new Date(m.created_at).toLocaleDateString();
                const typeLabel = m.memory_type === 'pinned' ? 'Saved by You' : 'Learned';
                return '<div class="memory-item ' + m.memory_type + '">' +
                    '<div class="memory-header">' +
                    '<span class="memory-type ' + m.memory_type + '">' + typeLabel + '</span>' +
                    '<span class="memory-date">' + date + '</span>' +
                    '</div>' +
                    '<div class="memory-content">' + escapeHtml(m.content) + '</div>' +
                    '<div class="memory-meta">' +
                    '<div class="memory-tags">' + (m.tags || []).map(t => '<span class="memory-tag">' + escapeHtml(t) + '</span>').join('') + '</div>' +
                    '<div class="memory-actions">' +
                    (m.memory_type === 'pinned' ? '<button class="btn-secondary btn-small" onclick="openEditModal(\'' + m.id + '\', \'' + escapeHtml(m.content.replace(/'/g, "\\'")) + '\', ' + JSON.stringify(m.tags || []) + ', ' + m.version + ')">Edit</button>' : '') +
                    '<button class="btn-danger btn-small" onclick="openDeleteModal(\'' + m.id + '\', \'' + escapeHtml(m.content.substring(0, 100).replace(/'/g, "\\'")) + '\')">Delete</button>' +
                    '</div>' +
                    '</div>' +
                    '</div>';
            }).join('');

            loadMoreBtn.style.display = memories.length < total ? 'block' : 'none';
        }

        // Escape HTML
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Filter by type
        function filterByType(type) {
            currentFilter = type;
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelector('.tab[data-type="' + type + '"]').classList.add('active');
            loadMemories(true);
        }

        // Debounce search
        let searchTimeout;
        function debounceSearch() {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                searchQuery = document.getElementById('searchInput').value.trim();
                loadMemories(true);
            }, 300);
        }

        // Load more
        function loadMoreMemories() {
            offset += limit;
            loadMemories(false);
        }

        // Modal functions
        function openAddModal() {
            if (!getApiBase()) return;
            document.getElementById('addModal').classList.add('active');
            document.getElementById('addContent').value = '';
            document.getElementById('addTags').value = '';
            document.getElementById('addType').value = 'pinned';
        }
        function closeAddModal() { document.getElementById('addModal').classList.remove('active'); }

        function openEditModal(id, content, tags, version) {
            document.getElementById('editModal').classList.add('active');
            document.getElementById('editMemoryId').value = id;
            document.getElementById('editContent').value = content;
            document.getElementById('editTags').value = (tags || []).join(', ');
            document.getElementById('editVersion').value = version;
        }
        function closeEditModal() { document.getElementById('editModal').classList.remove('active'); }

        function openDeleteModal(id, preview) {
            document.getElementById('deleteModal').classList.add('active');
            document.getElementById('deleteMemoryId').value = id;
            document.getElementById('deletePreview').textContent = preview + '...';
        }
        function closeDeleteModal() { document.getElementById('deleteModal').classList.remove('active'); }

        function openImportModal() {
            if (!getApiBase()) return;
            document.getElementById('importModal').classList.add('active');
            document.getElementById('importAgentId').value = 'dashboard';
            document.getElementById('importFile').value = '';
            document.getElementById('importProgress').style.display = 'none';
        }
        function closeImportModal() { document.getElementById('importModal').classList.remove('active'); }

        // Create memory
        async function createMemory() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            const content = document.getElementById('addContent').value.trim();
            const tags = document.getElementById('addTags').value.split(',').map(t => t.trim()).filter(Boolean);
            const memoryType = document.getElementById('addType').value;

            if (!content) {
                showToast('Content is required', 'error');
                return;
            }

            const btn = document.getElementById('addBtn');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner"></span> Adding...';

            try {
                const resp = await fetch(apiBase + '/memories', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content, tags, memory_type: memoryType, agent_id: 'dashboard' })
                });

                if (!resp.ok) {
                    const err = await resp.json();
                    throw new Error(err.error || 'Failed to create memory');
                }

                showToast('Memory added successfully');
                closeAddModal();
                loadMemories(true);
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            } finally {
                btn.disabled = false;
                btn.textContent = 'Add Memory';
            }
        }

        // Update memory
        async function updateMemory() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            const id = document.getElementById('editMemoryId').value;
            const content = document.getElementById('editContent').value.trim();
            const tags = document.getElementById('editTags').value.split(',').map(t => t.trim()).filter(Boolean);
            const version = document.getElementById('editVersion').value;

            if (!content) {
                showToast('Content is required', 'error');
                return;
            }

            const btn = document.getElementById('editBtn');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner"></span> Saving...';

            try {
                const resp = await fetch(apiBase + '/memories/' + id, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'If-Match': version
                    },
                    body: JSON.stringify({ content, tags })
                });

                if (!resp.ok) {
                    const err = await resp.json();
                    if (resp.status === 409) {
                        throw new Error('Version conflict - memory was modified by another request. Please refresh and try again.');
                    }
                    throw new Error(err.error || 'Failed to update memory');
                }

                showToast('Memory updated successfully');
                closeEditModal();
                loadMemories(true);
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            } finally {
                btn.disabled = false;
                btn.textContent = 'Save Changes';
            }
        }

        // Delete memory
        async function deleteMemory() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            const id = document.getElementById('deleteMemoryId').value;

            const btn = document.getElementById('deleteBtn');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner"></span> Deleting...';

            try {
                const resp = await fetch(apiBase + '/memories/' + id, { method: 'DELETE' });

                if (!resp.ok) {
                    const err = await resp.json();
                    throw new Error(err.error || 'Failed to delete memory');
                }

                showToast('Memory deleted successfully');
                closeDeleteModal();
                loadMemories(true);
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            } finally {
                btn.disabled = false;
                btn.textContent = 'Delete';
            }
        }

        // Export memories
        async function exportMemories() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            try {
                const resp = await fetch(apiBase + '/export');
                if (!resp.ok) throw new Error('Failed to export memories');

                const data = await resp.json();

                // Create downloadable file
                const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'memorix-export-' + new Date().toISOString().slice(0, 10) + '.json';
                a.click();
                URL.revokeObjectURL(url);

                showToast('Exported ' + data.memories.length + ' memories');
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            }
        }

        // Import memories
        async function importMemories() {
            const apiBase = getApiBase();
            if (!apiBase) return;

            const file = document.getElementById('importFile').files[0];
            const agentId = document.getElementById('importAgentId').value.trim() || 'dashboard';

            if (!file) {
                showToast('Please select a file', 'error');
                return;
            }

            const btn = document.getElementById('importBtn');
            btn.disabled = true;
            document.getElementById('importProgress').style.display = 'block';

            try {
                const formData = new FormData();
                formData.append('file', file);
                formData.append('agent_id', agentId);
                formData.append('file_type', 'memory');

                const resp = await fetch(apiBase + '/imports', {
                    method: 'POST',
                    body: formData
                });

                if (!resp.ok) {
                    const err = await resp.json();
                    throw new Error(err.error || 'Failed to start import');
                }

                const data = await resp.json();
                document.getElementById('importProgressText').textContent = 'Import started... Task ID: ' + data.id;

                // Poll for task status
                pollImportStatus(apiBase, data.id);
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
                btn.disabled = false;
            }
        }

        // Poll import status
        async function pollImportStatus(apiBase, taskId) {
            const maxPolls = 60; // 5 minutes max
            for (let i = 0; i < maxPolls; i++) {
                await new Promise(r => setTimeout(r, 5000));

                try {
                    const resp = await fetch(apiBase + '/imports/' + taskId);
                    const task = await resp.json();

                    const progress = task.total > 0 ? Math.round((task.done / task.total) * 100) : 0;
                    document.getElementById('importProgressFill').style.width = progress + '%';
                    document.getElementById('importProgressText').textContent = task.status + '... ' + task.done + '/' + task.total;

                    if (task.status === 'done') {
                        showToast('Import completed successfully');
                        closeImportModal();
                        document.getElementById('importBtn').disabled = false;
                        loadMemories(true);
                        return;
                    }

                    if (task.status === 'failed') {
                        showToast('Import failed: ' + (task.error || 'Unknown error'), 'error');
                        document.getElementById('importBtn').disabled = false;
                        return;
                    }
                } catch (e) {
                    console.error('Poll error:', e);
                }
            }

            showToast('Import is taking too long. Check task status later.', 'error');
            document.getElementById('importBtn').disabled = false;
        }

        // Close modals on outside click
        document.querySelectorAll('.modal-overlay').forEach(overlay => {
            overlay.addEventListener('click', e => {
                if (e.target === overlay) {
                    overlay.classList.remove('active');
                }
            });
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', e => {
            if (e.key === 'Escape') {
                document.querySelectorAll('.modal-overlay').forEach(m => m.classList.remove('active'));
            }
        });
    </script>
</body>
</html>
`))
}
