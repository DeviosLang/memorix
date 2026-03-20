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
