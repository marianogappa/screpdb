package worldstate

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// EnrichFromCommands walks a parsed command stream once and returns the
// enriched view, sorted by frame.
//
// Single source of all command iteration in the new pipeline. Anywhere
// else in the engine that wants to "look at the next command" must
// receive a []cmdenrich.EnrichedCommand, not the raw []*models.Command.
//
// Counterpart of the donor's Enrich(*rep.Replay) but operates on
// screpdb's parser-side models.Command (which already carries Frame and
// OrderName). Commands that don't classify (Sync, Chat, Vision, etc.)
// are dropped.
func EnrichFromCommands(commands []*models.Command) []cmdenrich.EnrichedCommand {
	if len(commands) == 0 {
		return nil
	}
	out := make([]cmdenrich.EnrichedCommand, 0, len(commands))
	for _, c := range commands {
		ec, ok := cmdenrich.Classify(c)
		if !ok {
			continue
		}
		out = append(out, ec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Frame < out[j].Frame })
	return out
}
