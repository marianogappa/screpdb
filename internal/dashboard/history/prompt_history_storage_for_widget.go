package history

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

type PromptHistoryStorageForWidget struct {
	phs      *PromptHistoryStorage
	ctx      context.Context
	widgetID int64
}

func (s *PromptHistoryStorageForWidget) Get() ([]llms.MessageContent, error) {
	return s.phs.get(s.ctx, s.widgetID)
}

func (s *PromptHistoryStorageForWidget) AddHumanPrompt(prompt string) error {
	return s.phs.add(s.ctx, s.widgetID, []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, prompt)})
}

func (s *PromptHistoryStorageForWidget) AddMessageContents(mcs []llms.MessageContent) error {
	return s.phs.add(s.ctx, s.widgetID, mcs)
}

func (s *PromptHistoryStorageForWidget) AddContentChoice(contentChoice *llms.ContentChoice) error {
	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, contentChoice.Content)
	for _, tc := range contentChoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	return s.phs.add(s.ctx, s.widgetID, []llms.MessageContent{assistantResponse})
}
