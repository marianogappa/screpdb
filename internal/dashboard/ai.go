package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

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
		resp, err := c.ai.llm.GenerateContent(
			c.ai.ctx,
			history,
			llms.WithTools(availableTools),
			llms.WithJSONMode(),
		)
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
			c.ai.logf("response from OpenAI has %v tool calls so looping...\n", len(respchoice.ToolCalls))
			continue
		}

		var sr StructuredResponse
		if err := json.Unmarshal([]byte(respchoice.Content), &sr); err != nil {
			return StructuredResponse{}, fmt.Errorf("failed to unmarshal final LLM response: %w", err)
		}
		return sr, nil
	}
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
