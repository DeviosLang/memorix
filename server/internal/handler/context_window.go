package handler

import (
	"net/http"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
)

// contextWindowRequest is the request body for the context window endpoint.
type contextWindowRequest struct {
	// Messages is the list of messages to truncate.
	Messages []service.ContextMessage `json:"messages"`

	// Optional overrides for the default configuration.
	MaxTokens *int `json:"max_tokens,omitempty"`

	// System prompt content (will be injected as a system message).
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Session metadata for injection (collected at session start, not stored long-term).
	// This is injected before memory content.
	SessionMetadata *domain.SessionMetadata `json:"session_metadata,omitempty"`

	// Memory content (will be injected as a memory message).
	MemoryContent string `json:"memory_content,omitempty"`

	// SessionID for logging/tracking purposes.
	SessionID string `json:"session_id,omitempty"`
}

// contextWindowResponse is the response from the context window endpoint.
type contextWindowResponse struct {
	// Truncated messages ready for LLM context.
	Messages []service.ContextMessage `json:"messages"`

	// Token counts.
	OriginalTokens int `json:"original_tokens"`
	FinalTokens    int `json:"final_tokens"`

	// Truncation metadata.
	MessagesDropped int            `json:"messages_dropped"`
	DroppedRoles    map[string]int `json:"dropped_roles,omitempty"`
	Truncated       bool           `json:"truncated"`

	// Tokenizer info.
	TokenizerName string `json:"tokenizer_name"`
	MaxTokens     int    `json:"max_tokens"`
}

// contextWindow handles POST requests to truncate a message list to fit within token limits.
// It preserves system prompts, session metadata, and memory injections while dropping oldest user/assistant pairs first.
func (s *Server) contextWindow(w http.ResponseWriter, r *http.Request) {
	var req contextWindowRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	if len(req.Messages) == 0 {
		s.handleError(w, &domain.ValidationError{Field: "messages", Message: "required"})
		return
	}

	// Build the full message list with injected content
	// Order: system -> metadata -> memory -> conversation
	messages := make([]service.ContextMessage, 0, len(req.Messages)+3)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, service.ContextMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Add session metadata if provided (before memory, after system)
	if req.SessionMetadata != nil {
		metadataMsg, err := service.BuildMetadataMessage(req.SessionMetadata, service.NewSessionMetadataFormatter(service.DefaultSessionMetadataConfig()))
		if err != nil {
			s.handleError(w, err)
			return
		}
		if metadataMsg.Content != "" {
			messages = append(messages, metadataMsg)
		}
	}

	// Add memory content if provided
	if req.MemoryContent != "" {
		messages = append(messages, service.ContextMessage{
			Role:    "memory",
			Content: req.MemoryContent,
		})
	}

	// Add the provided messages
	messages = append(messages, req.Messages...)

	// Get configuration from server settings or request overrides
	cfg := s.contextWindowConfig
	if cfg.Tokenizer == nil {
		cfg.Tokenizer = s.tokenizer
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		cfg.MaxTokens = *req.MaxTokens
	}

	// Create context window manager and truncate
	cw := service.NewContextWindow(cfg)
	result := cw.Truncate(messages)

	// Build response
	resp := contextWindowResponse{
		Messages:        result.Messages,
		OriginalTokens:  result.OriginalTokens,
		FinalTokens:     result.FinalTokens,
		MessagesDropped: result.MessagesDropped,
		DroppedRoles:    result.DroppedRoles,
		Truncated:       result.Truncated,
		TokenizerName:   cfg.Tokenizer.Name(),
		MaxTokens:       cfg.MaxTokens,
	}

	respond(w, http.StatusOK, resp)
}

// quickTruncateRequest is for simple truncation without detailed response.
type quickTruncateRequest struct {
	Messages        []service.ContextMessage `json:"messages"`
	MaxTokens       *int                     `json:"max_tokens,omitempty"`
	SystemPrompt    string                   `json:"system_prompt,omitempty"`
	SessionMetadata *domain.SessionMetadata  `json:"session_metadata,omitempty"`
	MemoryContent   string                   `json:"memory_content,omitempty"`
}

// quickTruncateResponse returns just the truncated messages.
type quickTruncateResponse struct {
	Messages []service.ContextMessage `json:"messages"`
}

// quickTruncate handles a simpler truncation request that returns only the truncated messages.
func (s *Server) quickTruncate(w http.ResponseWriter, r *http.Request) {
	var req quickTruncateRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	if len(req.Messages) == 0 {
		respond(w, http.StatusOK, quickTruncateResponse{Messages: []service.ContextMessage{}})
		return
	}

	// Build the full message list
	// Order: system -> metadata -> memory -> conversation
	messages := make([]service.ContextMessage, 0, len(req.Messages)+3)
	if req.SystemPrompt != "" {
		messages = append(messages, service.ContextMessage{Role: "system", Content: req.SystemPrompt})
	}

	// Add session metadata if provided (before memory, after system)
	if req.SessionMetadata != nil {
		metadataMsg, err := service.BuildMetadataMessage(req.SessionMetadata, service.NewSessionMetadataFormatter(service.DefaultSessionMetadataConfig()))
		if err == nil && metadataMsg.Content != "" {
			messages = append(messages, metadataMsg)
		}
	}

	if req.MemoryContent != "" {
		messages = append(messages, service.ContextMessage{Role: "memory", Content: req.MemoryContent})
	}
	messages = append(messages, req.Messages...)

	// Configure and truncate
	cfg := s.contextWindowConfig
	if cfg.Tokenizer == nil {
		cfg.Tokenizer = s.tokenizer
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		cfg.MaxTokens = *req.MaxTokens
	}

	cw := service.NewContextWindow(cfg)
	truncated := cw.QuickTruncate(messages)

	respond(w, http.StatusOK, quickTruncateResponse{Messages: truncated})
}

// countTokensRequest is for counting tokens in messages.
type countTokensRequest struct {
	Messages []service.ContextMessage `json:"messages"`
}

// countTokensResponse returns the token count.
type countTokensResponse struct {
	TokenCount int `json:"token_count"`
}

// countTokens handles a request to count tokens in a message list.
func (s *Server) countTokens(w http.ResponseWriter, r *http.Request) {
	var req countTokensRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	if len(req.Messages) == 0 {
		respond(w, http.StatusOK, countTokensResponse{TokenCount: 0})
		return
	}

	cfg := s.contextWindowConfig
	if cfg.Tokenizer == nil {
		cfg.Tokenizer = s.tokenizer
	}

	cw := service.NewContextWindow(cfg)
	count := cw.CountTokens(req.Messages)

	respond(w, http.StatusOK, countTokensResponse{TokenCount: count})
}

// buildContextRequest is the request body for the context builder endpoint.
type buildContextRequest struct {
	// SessionID is the current session identifier.
	SessionID string `json:"session_id,omitempty"`

	// UserID is the user identifier for fetching user memories/facts.
	UserID string `json:"user_id,omitempty"`

	// SystemInstructions contains the base system prompt/instructions.
	SystemInstructions string `json:"system_instructions,omitempty"`

	// SessionMetadata contains session-level metadata.
	SessionMetadata *domain.SessionMetadata `json:"session_metadata,omitempty"`

	// ConversationSummary is a summary of past conversations.
	ConversationSummary string `json:"conversation_summary,omitempty"`

	// CurrentMessages contains the current session messages.
	CurrentMessages []domain.ContextMessage `json:"current_messages,omitempty"`

	// MaxTokens is the maximum total tokens allowed.
	// If not set, uses the default from configuration.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// buildContextResponse is the response from the context builder endpoint.
type buildContextResponse struct {
	// Prompt is the assembled system prompt.
	Prompt string `json:"prompt"`

	// TotalTokens is the total token count of the assembled prompt.
	TotalTokens int `json:"total_tokens"`

	// MaxTokens is the maximum tokens that were allowed.
	MaxTokens int `json:"max_tokens"`

	// LayerStats contains token statistics for each layer.
	LayerStats []domain.LayerStats `json:"layer_stats"`

	// Truncated indicates whether any truncation occurred.
	Truncated bool `json:"truncated"`

	// TruncationDetails contains details about what was truncated.
	TruncationDetails []domain.TruncationDetail `json:"truncation_details,omitempty"`
}

// buildContext handles POST requests to build a context prompt from multiple layers.
// It assembles content from system instructions, session metadata, user memories,
// conversation summaries, and current session messages within the token budget.
func (s *Server) buildContext(w http.ResponseWriter, r *http.Request) {
	var req buildContextRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	auth := authInfo(r)
	svc := s.resolveServices(auth)

	// Build the context request
	buildReq := &domain.BuildContextRequest{
		SessionID:           req.SessionID,
		UserID:              req.UserID,
		SystemInstructions:  req.SystemInstructions,
		SessionMetadata:     req.SessionMetadata,
		ConversationSummary: req.ConversationSummary,
		CurrentMessages:     req.CurrentMessages,
		MaxTokens:           req.MaxTokens,
	}

	// Fetch user memories if UserID is provided
	if req.UserID != "" {
		memories, _, err := svc.memory.Search(r.Context(), domain.MemoryFilter{
			AgentID: req.UserID,
			Limit:   20,
		})
		if err == nil && len(memories) > 0 {
			buildReq.UserMemories = memories
		}

		// Fetch user profile facts
		facts, err := svc.userProfile.GetFactsByUser(r.Context(), req.UserID)
		if err == nil && len(facts) > 0 {
			buildReq.UserProfileFacts = facts
		}
	}

	// Create context builder and build the prompt
	builder := service.NewContextBuilder(s.contextBuilderConfig)
	result, err := builder.Build(buildReq)
	if err != nil {
		s.handleError(w, err)
		return
	}

	// Build response
	resp := buildContextResponse{
		Prompt:            result.Prompt,
		TotalTokens:       result.TotalTokens,
		MaxTokens:         result.MaxTokens,
		LayerStats:        result.LayerStats,
		Truncated:         result.Truncated,
		TruncationDetails: result.TruncationDetails,
	}

	respond(w, http.StatusOK, resp)
}
