package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/marianogappa/screpdb/internal/dashboard/history"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/outputparser"
)

type AI struct {
	ctx                context.Context
	llm                *openai.LLM
	store              storage.Storage
	promptHistoryStore *history.PromptHistoryStorage
	debug              bool
}

func NewAI(ctx context.Context, openaiAPIKey string, store storage.Storage, db *sql.DB, debug bool) (*AI, error) {
	llm, err := openai.New(openai.WithToken(openaiAPIKey), openai.WithResponseFormat(responseFormat))
	if err != nil {
		return nil, err
	}
	promptHistoryStore := history.NewPromptHistoryStorage(db, true)
	return &AI{ctx: ctx, llm: llm, store: store, promptHistoryStore: promptHistoryStore, debug: debug}, nil
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
			llms.WithModel("gpt-4o-2024-08-06"),
		)
		c.ai.logf("sent request to OpenAI with history with %v entries...\n", len(history))
		if err != nil {
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
