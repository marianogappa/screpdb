package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	OpenAIDefaultModel    = "gpt-4o-2024-08-06"
	AnthropicDefaultModel = "claude-opus-4"
	GeminiDefaultModel    = "gemini-2.5-flash-lite"
)

type AI struct {
	ctx            context.Context
	llm            llms.Model
	resolvedVendor string
	resolvedModel  string
	store          storage.Storage
	debug          bool
}

func WithDebug(d bool) func(*AI) error {
	return func(a *AI) error {
		a.debug = d
		return nil
	}
}

func WithOpenAI(openaiAPIKey string, model string) func(*AI) error {
	_model := os.Getenv("OPENAI_MODEL")
	if model != "" {
		_model = model
	}
	if _model == "" {
		_model = OpenAIDefaultModel
	}

	_openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey != "" {
		_openaiAPIKey = openaiAPIKey
	}

	return func(a *AI) error {
		llm, err := openai.New(
			openai.WithToken(_openaiAPIKey),
			openai.WithResponseFormat(responseFormat),
			openai.WithModel(_model),
		)
		if err != nil {
			return err
		}
		a.llm = llm
		a.resolvedModel = _model
		a.resolvedVendor = AIVendorOpenAI
		return nil
	}
}

func WithAnthropic(anthropicAPIKey string, model string) func(*AI) error {
	return func(a *AI) error {
		_model := AnthropicDefaultModel
		if os.Getenv("ANTHROPIC_MODEL") != "" {
			_model = os.Getenv("ANTHROPIC_MODEL")
		}
		if model != "" {
			_model = model
		}

		_anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
		if anthropicAPIKey != "" {
			_anthropicAPIKey = anthropicAPIKey
		}

		llm, err := anthropic.New(
			anthropic.WithToken(_anthropicAPIKey),
			anthropic.WithModel(_model),
		)
		if err != nil {
			return err
		}
		a.llm = llm
		a.resolvedModel = _model
		a.resolvedVendor = AIVendorAnthropic
		return nil
	}
}

func WithGemini(geminiAPIKey string, model string) func(*AI) error {
	return func(a *AI) error {
		_model := os.Getenv("GEMINI_MODEL")
		if model != "" {
			_model = model
		}
		if _model == "" {
			_model = GeminiDefaultModel
		}

		_geminiAPIKey := os.Getenv("GEMINI_API_KEY")
		if geminiAPIKey != "" {
			_geminiAPIKey = geminiAPIKey
		}

		llm, err := googleai.New(
			context.Background(),
			googleai.WithAPIKey(_geminiAPIKey),
			googleai.WithDefaultModel(_model),
		)
		if err != nil {
			return err
		}
		a.llm = llm
		a.resolvedModel = _model
		a.resolvedVendor = AIVendorGemini
		return nil
	}
}

type Option func(*AI) error

func NewAI(ctx context.Context, store storage.Storage, _ *sql.DB, opts ...Option) (*AI, error) {
	a := &AI{ctx: ctx, store: store}
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return a, fmt.Errorf("error constructing AI: %w", err)
		}
	}
	a.logf("resolved AI vendor: %s with model %s", a.resolvedVendor, a.resolvedModel)
	return a, nil
}

// stripMarkdownCodeFences removes markdown code fences (```json ... ```) that
// non-OpenAI models sometimes wrap around JSON responses.
func stripMarkdownCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return s
	}
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline == -1 {
		return s
	}
	trimmed = trimmed[firstNewline+1:]
	if idx := strings.LastIndex(trimmed, "```"); idx != -1 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
}

func (s *AI) logf(message string, args ...any) {
	if !s.debug {
		return
	}
	log.Printf(message, args...)
}

func (a *AI) IsAvailable() bool {
	return a.llm != nil
}

type WorkflowStructuredResponse struct {
	Config      WidgetConfig `json:"config"`
	SQLQuery    string       `json:"sql_query"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	TextAnswer  string       `json:"text_answer"`
}

func (a *AI) AnswerWorkflowQuestion(question string, contextData any, scopeInstructions string) (WorkflowStructuredResponse, error) {
	if a.llm == nil {
		return WorkflowStructuredResponse{}, fmt.Errorf("OpenAI API key not configured")
	}
	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		return WorkflowStructuredResponse{}, fmt.Errorf("failed to marshal context: %w", err)
	}

	systemPrompt := fmt.Sprintf(`You are answering StarCraft replay analytics questions.
Return ONLY JSON matching the configured schema with fields: title, description, sql_query, config, text_answer.

You have access to tools:
- query_database
- get_database_schema
- get_starcraft_knowledge

Use tools before finalizing whenever SQL is needed.
If a chart/table answer is appropriate, choose one of: gauge, table, pie_chart, bar_chart, line_chart, scatter_plot, histogram, heatmap.
If the user asked for a plain explanation (or SQL would be uncertain), use config.type="text" and put the explanation in text_answer.

Scope restrictions:
%s

Use only the provided context and tool outputs. If unsure, use text type and be explicit.
`, scopeInstructions)

	userPrompt := fmt.Sprintf("Context JSON:\n%s\n\nQuestion:\n%s", string(contextJSON), question)
	history := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	for i := 0; i < 6; i++ {
		callOpts := []llms.CallOption{llms.WithTools(availableTools)}
		if a.resolvedVendor == AIVendorOpenAI {
			callOpts = append(callOpts, llms.WithJSONMode())
		}
		resp, err := a.llm.GenerateContent(a.ctx, history, callOpts...)
		if err != nil {
			return WorkflowStructuredResponse{}, err
		}
		if len(resp.Choices) == 0 {
			return WorkflowStructuredResponse{}, fmt.Errorf("LLM returned no choices")
		}
		choice := resp.Choices[0]
		assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, choice.Content)
		for _, tc := range choice.ToolCalls {
			assistantResponse.Parts = append(assistantResponse.Parts, tc)
		}
		history = append(history, assistantResponse)
		toolResponses := a.respondToToolCalls(choice.ToolCalls)
		if len(toolResponses) > 0 {
			history = append(history, toolResponses...)
			continue
		}
		content := stripMarkdownCodeFences(choice.Content)
		var sr WorkflowStructuredResponse
		if err := json.Unmarshal([]byte(content), &sr); err != nil {
			return WorkflowStructuredResponse{}, fmt.Errorf("failed to unmarshal workflow response: %w", err)
		}
		if sr.Config.Type == "" {
			sr.Config.Type = WidgetTypeText
		}
		return sr, nil
	}
	return WorkflowStructuredResponse{}, fmt.Errorf("LLM reached tool-call iteration limit")
}
