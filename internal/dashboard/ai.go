package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/history"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/outputparser"
)

const (
	OpenAIDefaultModel    = "gpt-4o-2024-08-06"
	AnthropicDefaultModel = "claude-opus-4"
	GeminiDefaultModel    = "gemini-2.5-flash-lite"
)

type AI struct {
	ctx                context.Context
	llm                llms.Model
	resolvedVendor     string
	resolvedModel      string
	store              storage.Storage
	promptHistoryStore *history.PromptHistoryStorage
	debug              bool
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

func NewAI(ctx context.Context, store storage.Storage, db *sql.DB, opts ...Option) (*AI, error) {
	promptHistoryStore := history.NewPromptHistoryStorage(db, true)
	a := &AI{ctx: ctx, store: store, promptHistoryStore: promptHistoryStore}
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return a, fmt.Errorf("error constructing AI: %w", err)
		}
	}
	a.logf("resolved AI vendor: %s with model %s", a.resolvedVendor, a.resolvedModel)
	return a, nil
}

type Conversation struct {
	ai             *AI
	storeForWidget *history.PromptHistoryStorageForWidget
}

func (a *AI) NewConversation(widgetID int64) (*Conversation, error) {
	storeForWidget, err := a.promptHistoryStore.ForWidgetID(a.ctx, widgetID)
	if err != nil {
		return nil, err
	}
	return &Conversation{
		ai:             a,
		storeForWidget: storeForWidget,
	}, nil
}

type StructuredResponse struct {
	Config      WidgetConfig `json:"config"`
	SQLQuery    string       `json:"sql_query"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
}

func (c *Conversation) Prompt(prompt string) (StructuredResponse, error) {
	if c.ai.llm == nil {
		return StructuredResponse{}, fmt.Errorf("OpenAI API key not configured")
	}
	definedParser, err := outputparser.NewDefined(StructuredResponse{})
	if err != nil {
		return StructuredResponse{}, err
	}
	if err := c.storeForWidget.AddHumanPrompt(fmt.Sprintf("%s\n%s", prompt, definedParser.GetFormatInstructions())); err != nil {
		return StructuredResponse{}, err
	}
	for {
		history, err := c.storeForWidget.Get()
		if err != nil {
			return StructuredResponse{}, err
		}
		callOpts := []llms.CallOption{llms.WithTools(availableTools)}
		if c.ai.resolvedVendor == AIVendorOpenAI {
			callOpts = append(callOpts, llms.WithJSONMode())
		}
		resp, err := c.ai.llm.GenerateContent(c.ai.ctx, history, callOpts...)
		c.ai.logf("sent request to %v with history with %v entries...\n", c.ai.resolvedVendor, len(history))
		if err != nil {
			c.ai.logf("request failed with error: %v\n", err)
			return StructuredResponse{}, err
		}
		respchoice := resp.Choices[0]
		if err := c.storeForWidget.AddContentChoice(respchoice); err != nil {
			return StructuredResponse{}, err
		}
		if err := c.storeForWidget.AddMessageContents(c.ai.respondToToolCalls(respchoice.ToolCalls)); err != nil {
			return StructuredResponse{}, err
		}
		if len(respchoice.ToolCalls) > 0 {
			c.ai.logf("response has %v tool calls so looping...\n", len(respchoice.ToolCalls))
			continue
		}

		content := stripMarkdownCodeFences(respchoice.Content)
		c.ai.logf("final LLM response (first 500 chars): %.500s\n", content)
		var sr StructuredResponse
		if err := json.Unmarshal([]byte(content), &sr); err != nil {
			sr, retryErr := c.retryForJSON(&definedParser)
			if retryErr != nil {
				return StructuredResponse{}, fmt.Errorf("failed to unmarshal final LLM response: %w (retry also failed: %v)", err, retryErr)
			}
			return sr, nil
		}
		return sr, nil
	}
}

// retryForJSON sends a follow-up request without tools but with JSONMode to
// coerce the model into returning valid JSON. This handles vendors like Gemini
// that don't support tools+JSONMode together: the tool-calling phase runs
// without JSONMode, and if the final response isn't valid JSON, this retry
// drops tools so JSONMode can be used.
func (c *Conversation) retryForJSON(parser *outputparser.Defined[StructuredResponse]) (StructuredResponse, error) {
	c.ai.logf("response was not valid JSON, retrying with JSONMode and no tools...\n")
	correction := fmt.Sprintf(
		"Your previous response was not valid JSON. Respond with ONLY the JSON object, no explanation or markdown.\n%s",
		parser.GetFormatInstructions(),
	)
	if err := c.storeForWidget.AddHumanPrompt(correction); err != nil {
		return StructuredResponse{}, err
	}
	history, err := c.storeForWidget.Get()
	if err != nil {
		return StructuredResponse{}, err
	}
	resp, err := c.ai.llm.GenerateContent(c.ai.ctx, history, llms.WithJSONMode())
	if err != nil {
		return StructuredResponse{}, fmt.Errorf("JSON retry request failed: %w", err)
	}
	respchoice := resp.Choices[0]
	if err := c.storeForWidget.AddContentChoice(respchoice); err != nil {
		return StructuredResponse{}, err
	}
	content := stripMarkdownCodeFences(respchoice.Content)
	c.ai.logf("JSON retry response (first 500 chars): %.500s\n", content)
	var sr StructuredResponse
	if err := json.Unmarshal([]byte(content), &sr); err != nil {
		return StructuredResponse{}, fmt.Errorf("JSON retry response still not valid JSON: %w", err)
	}
	return sr, nil
}

// stripMarkdownCodeFences removes markdown code fences (```json ... ```) that
// non-OpenAI models sometimes wrap around JSON responses.
func stripMarkdownCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return s
	}
	// Remove opening fence (```json, ```JSON, or just ```)
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline == -1 {
		return s
	}
	trimmed = trimmed[firstNewline+1:]
	// Remove closing fence
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
