package dashboard

import (
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

// availableTools simulates the tools/functions we're making available for
// the model.
var availableTools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "query_database",
			Description: "Execute SQL queries against the StarCraft replay database. The database contains tables: replays (metadata), players (player info) & commands (game events). Use this tool to analyze replay statistics, player performance, unit usage, and game patterns.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sql": map[string]any{
						"type":        "string",
						"description": "PostgresSQL query to execute against the StarCraft replay database",
					},
				},
				"required": []string{"sql"},
			},
		},
	},
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "get_database_schema",
			Description: "Detailed information about the StarCraft replay database schema including table structures, relationships obtained by querying the database itself.",
			Parameters:  map[string]any{},
		},
	},
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "get_starcraft_knowledge",
			Description: "Summary of StarCraft knowledge useful for knowing how to answer questions and make reports.",
			Parameters:  map[string]any{},
		},
	},
}

func (a *AI) respondToToolCalls(tcs []llms.ToolCall) []llms.MessageContent {
	messageHistory := []llms.MessageContent{}
	for _, tc := range tcs {
		a.logf("AI called the %v tool\n", tc.FunctionCall.Name)
		switch tc.FunctionCall.Name {
		case "query_database":
			var args struct {
				SQL string `json:"sql"`
			}
			if err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args); err != nil {
				messageHistory = append(messageHistory, buildResponse(tc, fmt.Sprintf("failed to unmarshal arguments: %v", err)))
				continue
			}
			queryResult, err := a.store.Query(a.ctx, args.SQL)
			if err != nil {
				messageHistory = append(messageHistory, buildResponse(tc, fmt.Sprintf("error running query: %v", err)))
			} else {
				messageHistory = append(messageHistory, buildResponse(tc, formatQueryResults(queryResult)))
			}
		case "get_database_schema":
			schema, err := a.store.GetDatabaseSchema(a.ctx)
			if err != nil {
				messageHistory = append(messageHistory, buildResponse(tc, fmt.Sprintf("failed to get database schema: %v", err)))
				continue
			}
			messageHistory = append(messageHistory, buildResponse(tc, fmt.Sprintf("%v\n%v", schema, schemaObservations)))
		case "get_starcraft_knowledge":
			messageHistory = append(messageHistory, buildResponse(tc, starcraftKnowledge))
		default:
			messageHistory = append(messageHistory, buildResponse(tc, "error: unknown function call"))
		}
	}
	return messageHistory
}

func buildResponse(tc llms.ToolCall, content string) llms.MessageContent {
	return llms.MessageContent{
		Role: llms.ChatMessageTypeTool,
		Parts: []llms.ContentPart{
			llms.ToolCallResponse{
				ToolCallID: tc.ID,
				Name:       tc.FunctionCall.Name,
				Content:    content,
			},
		},
	}
}
