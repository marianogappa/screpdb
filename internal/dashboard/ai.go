package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/outputparser"
)

type AI struct {
	ctx   context.Context
	llm   *openai.LLM
	store storage.Storage
	// conversations []Conversation
}

func NewAI(ctx context.Context, openaiAPIKey string, store storage.Storage) (*AI, error) {
	llm, err := openai.New(openai.WithToken(openaiAPIKey), openai.WithResponseFormat(responseFormat))
	if err != nil {
		log.Fatal(err)
	}
	return &AI{ctx: ctx, llm: llm, store: store}, nil
}

type Conversation struct {
	ai      *AI
	history []llms.MessageContent
}

func (a *AI) NewConversation(widgetID int) (*Conversation, error) {
	var sp bytes.Buffer
	if err := systemPromptTpl.Execute(&sp, struct{ WidgetID int }{widgetID}); err != nil {
		return nil, err
	}

	return &Conversation{
		ai:      a,
		history: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeSystem, sp.String())},
	}, nil
}

func (c *Conversation) addHumanPrompt(prompt string) {
	log.Printf("Adding human prompt: %s\n", prompt)
	c.history = append(c.history, llms.TextParts(llms.ChatMessageTypeHuman, prompt))
}

func (c *Conversation) addMessageContents(mcs []llms.MessageContent) {
	for _, mc := range mcs {
		log.Printf("Adding AI message content: %+v\n", mc)
	}
	c.history = append(c.history, mcs...)
}

func (c *Conversation) addContentChoice(contentChoice *llms.ContentChoice) {
	log.Printf("Adding AI content: %s\n", contentChoice.Content)
	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, contentChoice.Content)
	for _, tc := range contentChoice.ToolCalls {
		log.Printf("Adding AI response part: %+v\n", tc)
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	c.history = append(c.history, assistantResponse)
}

type StructuredResponse struct {
	HTMLContent string `json:"html_content"`
	SQLQuery    string `json:"sql_query"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TODO: don't save history in memory
func (c *Conversation) Prompt(prompt string) (StructuredResponse, error) {
	definedParser, err := outputparser.NewDefined(StructuredResponse{})
	if err != nil {
		return StructuredResponse{}, err
	}
	c.addHumanPrompt(fmt.Sprintf("%s\n%s", prompt, definedParser.GetFormatInstructions()))
	for {
		resp, err := c.ai.llm.GenerateContent(
			c.ai.ctx,
			c.history,
			llms.WithTools(availableTools),
			llms.WithModel("gpt-4o-2024-08-06"),
		)
		if err != nil {
			return StructuredResponse{}, err
		}
		respchoice := resp.Choices[0]
		c.addContentChoice(respchoice)
		c.addMessageContents(c.ai.respondToToolCalls(respchoice.ToolCalls))
		if len(respchoice.ToolCalls) > 0 {
			continue
		}

		var sr StructuredResponse
		if err := json.Unmarshal([]byte(respchoice.Content), &sr); err != nil {
			return StructuredResponse{}, fmt.Errorf("failed to unmarshal final LLM response: %w", err)
		}
		return sr, nil
	}
}

func (a *AI) respondToToolCalls(tcs []llms.ToolCall) []llms.MessageContent {
	messageHistory := []llms.MessageContent{}
	for _, tc := range tcs {
		switch tc.FunctionCall.Name {
		case "query_database":
			var args struct {
				SQL string `json:"sql"`
			}
			if err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args); err != nil {
				toolResponse := llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: tc.ID,
							Name:       tc.FunctionCall.Name,
							Content:    fmt.Sprintf("failed to unmarshal arguments: %v", err),
						},
					},
				}
				messageHistory = append(messageHistory, toolResponse)
				continue
			}
			queryResult, err := a.store.Query(a.ctx, args.SQL)
			if err != nil {
				toolResponse := llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: tc.ID,
							Name:       tc.FunctionCall.Name,
							Content:    fmt.Sprintf("error running query: %v", err),
						},
					},
				}
				messageHistory = append(messageHistory, toolResponse)
			} else {
				toolResponse := llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: tc.ID,
							Name:       tc.FunctionCall.Name,
							Content:    formatQueryResults(queryResult),
						},
					},
				}
				messageHistory = append(messageHistory, toolResponse)
			}
		case "get_database_schema":
			schema, err := a.store.GetDatabaseSchema(a.ctx)
			if err != nil {
				toolResponse := llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: tc.ID,
							Name:       tc.FunctionCall.Name,
							Content:    fmt.Sprintf("failed to get database schema: %v", err),
						},
					},
				}
				messageHistory = append(messageHistory, toolResponse)
				continue
			}
			toolResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: tc.ID,
						Name:       tc.FunctionCall.Name,
						Content:    fmt.Sprintf("%v\n%v", schema, schemaObservations),
					},
				},
			}
			messageHistory = append(messageHistory, toolResponse)
		case "get_starcraft_knowledge":
			toolResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: tc.ID,
						Name:       tc.FunctionCall.Name,
						Content:    starcraftKnowledge,
					},
				},
			}
			messageHistory = append(messageHistory, toolResponse)
		default:
			toolResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: tc.ID,
						Name:       tc.FunctionCall.Name,
						Content:    "error: unknown function call",
					},
				},
			}
			messageHistory = append(messageHistory, toolResponse)
		}
	}
	return messageHistory
}

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

// formatQueryResults formats query results for display
func formatQueryResults(results []map[string]any) string {
	if len(results) == 0 {
		return "No results found."
	}

	// Get column names from first row
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	// Create table format
	var output strings.Builder
	output.WriteString("Query Results:\n\n")

	// Header
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		output.WriteString(col)
	}
	output.WriteString("\n")

	// Separator
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		for j := 0; j < len(col); j++ {
			output.WriteString("-")
		}
	}
	output.WriteString("\n")

	// Data rows
	for _, row := range results {
		for i, col := range columns {
			if i > 0 {
				output.WriteString(" | ")
			}
			value := fmt.Sprintf("%v", row[col])
			output.WriteString(value)
		}
		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("\nTotal rows: %d", len(results)))

	return output.String()
}

const (
	schemaObservations = `
	- Replays have up to 8 players (and up to 4 observers) and a sequential list of commands/actions (like Chess). Command timing is tracked in "frames" since game start and also with a timestamp.
	- The commands table has action-type-specific fields, so for a given row many fields are null.

	- JOIN patterns:
		- players.replay_id = replays.id
		- commands.replay_id = replays.id
		- commands.player_id = players.id

	- Common WHERE clauses:
		- players.type = 'Human' (i.e. skip 'Computer' players)
		- players.is_observer = false (i.e. Observer players are not part of the game)

	action_types:
		- Build
		- Land
		- RightClick
		- TargetedOrder
		- Train
		- BuildInterceptorOrScarab
		- MinimapPing
		- CancelTrain
		- UnitMorph
		- Tech
		- Upgrade
		- GameSpeed
		- Hotkey
		- Chat
		- Vision
		- Alliance
		- LeaveGame
		- Stop
		- CarrierStop
		- ReaverStop
		- ReturnCargo
		- UnloadAll
		- HoldPosition
		- Burrow
		- Unburrow
		- Siege
		- Unsiege
		- Cloack
		- Decloack
		- Cheat

	unit_types:

		- Supply Depot
		- Forge
		- Hydralisk Den
		- Siege Tank (Tank Mode)
		- Barracks
		- Reaver
		- Engineering Bay
		- ComSat
		- Valkyrie
		- Corsair
		- Creep Colony
		- Extractor
		- Covert Ops
		- Gateway
		- Ultralisk
		- Academy
		- Nuclear Missile
		- Defiler Mound
		- Guardian
		- Spore Colony
		- Templar Archives
		- Arbiter
		- Hive
		- Firebat
		- Zealot
		- Arbiter Tribunal
		- Cybernetics Core
		- Wraith
		- Overlord
		- Evolution Chamber
		- Stargate
		- Physics Lab
		- Spawning Pool
		- Science Facility
		- Fleet Beacon
		- Goliath
		- Probe
		- Missile Turret
		- Sunken Colony
		- Robotics Support Bay
		- Vulture
		- Nuclear Silo
		- Medic
		- Observatory
		- Queen
		- High Templar
		- Starport
		- Ghost
		- Spire
		- Armory
		- Factory
		- Nexus
		- Marine
		- Bunker
		- Battlecruiser
		- Shield Battery
		- Robotics Facility
		- Mutalisk
		- Carrier
		- Hydralisk
		- Shuttle
		- Scourge
		- Observer
		- Greater Spire
		- Devourer
		- Scout
		- Drone
		- Machine Shop
		- Lair
		- Refinery
		- Dark Templar
		- SCV
		- Nydus Canal
		- Queens Nest
		- Dropship
		- Hatchery
		- Ultralisk Cavern
		- Assimilator
		- Science Vessel
		- Dragoon
		- Photon Cannon
		- Lurker
		- Defiler
		- Pylon
		- Control Tower
		- Zergling
		- Citadel of Adun
		- Command Center
	`

	starcraftKnowledge = `
"StarCraft: Remastered" is a real-time strategy game where players choose a race (Terran, Protoss or Zerg), build economies, form armies, sometimes ally other players, and battle for map control until one side destroys all opponent buildings to win.

Game Mechanics

- Economy: workers mine → resources → spend on units/buildings
- Tech tree: unlocks progressively with buildings
- Army control: composition, micro, positioning
- Fog of war: vision-limited
- Win condition: destroy all opponent buildings
- Maximum supply count: 200. Workers cost 1. Heavier units cost 2, 4, etc.

Units: combat, workers, spellcasters, transports, detectors
Resources: minerals, vespene gas, supply (food cap)
Buildings: tech tree enablers, production, economy, defense
Worker units: Drone (Zerg), Probe (Protoss), SCV (Terran)
Main building: Nexus (Protoss), Command Center (Terran), Hatchery/Lair/Hive (Zerg). Resources are gathered to these buildings.

Replay Essentials

- Timeline of actions (build orders, expansions, engagements)
- APM (actions per minute), supply, resources, spending efficiency
- Army composition over time
- Map size is in tiles (128x128, 96x256) but (x, y) is in pixels. 1 tile = 32 pixels.
- Map control, scouting, expansions

Macro vs Micro

- These commands are macro: Build, Train, BuildInterceptorOrScarab, UnitMorph, Tech, Upgrade
- These commands are micro: RightClick, TargetedOrder, Hotkey, UnloadAll, HoldPosition, Burrow, Unburrow, Siege, Unsiege, Cloack, Decloack

Report Metrics

- If you're asked to report on players, stick to "players.type = 'Human'" (skip Computer)
- 'players.is_winner' is not too accurate. Players may leave game after winning, replays may be incomplete.
- Resource collection & spending efficiency
- Player performance stats (APM, using hotkeys, macro vs micro balance, time to first building, time to first combat unit, time to expansion)
- Better players: have higher APMs, use hotkeys, > micro actions, if they have more workers they make more units/buildings.
- Build order timings (e.g. “2 Hatch Muta,” “1 Gate Expand”)

Meta terms

- "rush": when a player attacks another (e.g. RightClick, TargetedOrder w/ order_name Attack*) within a few minutes of game start
- "timing push": deliberate attack launched at a specific moment when a build order hits a temporary power spike (e.g. first tanks with siege, zealots become fast, mutalisks get +1 attack).
- "tech switch": Rapidly shifting production to a different unit tech path to exploit an opponent’s weak counters (e.g. mutalisks → lurkers, marine+medic → tank+goliath).
- "natural": The first "expansion" (main building) to gather more resources, which is in close proximity to the main starting location.
- "expa/expansion": Another expansion which is not necessarily the "natural".
- Main building starting locations are usually conveyed in o'clock positions (like 3, 6, 9, 12).

	`
	systemPromptTemplate = `You help to create Starcraft: Remastered dashboards. The prompts ask to create dashboard widgets. Each widget is a UI component fed from one SQL query.
The responses must be structured JSON which return:
- widget title
- widget description
- widget PostgreSQL query
- widget HTML content

Assume you have D3.js in scope to draw a chart with the data, and that the data is available as an array of objects (as your query returns) in a variable called sqlRowsForWidget{{.WidgetID}}.
You have a limited surface (e.g. 500x500) for the widget.

IMPORTANT: The dashboard background is black. When generating HTML content, ensure that text colors, chart colors, and other visual elements are chosen to be visible and readable against a black background. Use light colors for text and ensure sufficient contrast for all visual elements.

You must first use the available tools to figure out how to construct the query, and then to run it and make sure that the results make sense (and to know how to display it).
`
)

var (
	systemPromptTpl, _ = template.New("").Parse(systemPromptTemplate)
	responseFormat     = &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   "widget_schema",
			Strict: true,
			Schema: &openai.ResponseFormatJSONSchemaProperty{
				Type: "object",
				Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
					"title": {
						Type:        "string",
						Description: "Widget's title",
					},
					"description": {
						Type:        "string",
						Description: "Succinct description of the widget's content",
					},
					"html_content": {
						Type:        "string",
						Description: "HTML content with potentially CSS style tags and JS script tags (It must use the SQL rows returned by running sql_query)",
					},
					"sql_query": {
						Type:        "string",
						Description: "A valid PostgreSQL query that returns the rows that feed into the widgets D3 chart/content.",
					},
				},
				AdditionalProperties: false,
				Required:             []string{"title", "description", "html_content", "sql_query"},
			},
		},
	}
)
