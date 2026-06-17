package parser

import (
	"sync"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/scmapanalyzer/replaymap"
	"github.com/marianogappa/screpdb/internal/models"
)

// mapAnalyzerClient is a process-wide singleton scmapanalyzer.Client. NewClient
// JSON-unmarshals every embedded ladder map on each call (~17 ms per replay on
// a real corpus), so we pay that cost once and reuse the client. Client itself
// is documented safe for concurrent use.
var (
	mapAnalyzerClientOnce sync.Once
	mapAnalyzerClient     *scmapanalyzer.Client
	mapAnalyzerClientErr  error
)

func getMapAnalyzerClient() (*scmapanalyzer.Client, error) {
	mapAnalyzerClientOnce.Do(func() {
		mapAnalyzerClient, mapAnalyzerClientErr = scmapanalyzer.NewClient()
	})
	return mapAnalyzerClient, mapAnalyzerClientErr
}

// buildMapContextLayoutFromReplay returns the polygon layout for a replay's
// map. mapName lets scmapanalyzer.Analyze short-circuit the replay parse when
// the map is in the embedded ladder cache (the common case). widthTiles and
// heightTiles come from the already-parsed *rep.Replay so we do not re-parse
// the file just to read header dimensions.
func buildMapContextLayoutFromReplay(replayPath string, mapName string, widthTiles, heightTiles int) (*models.MapContextLayout, error) {
	client, err := getMapAnalyzerClient()
	if err != nil {
		return nil, err
	}

	result, err := client.Analyze(replayPath, scmapanalyzer.WithMapName(mapName))
	if err != nil {
		return nil, err
	}

	bases := make([]models.MapContextBase, 0, len(result.Starts)+len(result.Expas))
	for _, base := range result.Starts {
		bases = append(bases, toContextBase(base))
	}
	for _, base := range result.Expas {
		bases = append(bases, toContextBase(base))
	}
	if len(bases) == 0 {
		return nil, nil
	}
	return &models.MapContextLayout{
		Bases:       bases,
		WidthTiles:  widthTiles,
		HeightTiles: heightTiles,
	}, nil
}

func toContextBase(base replaymap.BasePolygon) models.MapContextBase {
	polygon := make([]models.MapPolygonPoint, 0, len(base.PolygonVertices))
	for _, vertex := range base.PolygonVertices {
		polygon = append(polygon, models.MapPolygonPoint{
			X: minitileToPixelInt(vertex.X),
			Y: minitileToPixelInt(vertex.Y),
		})
	}
	return models.MapContextBase{
		Name:  base.Name,
		Kind:  base.Kind,
		Clock: base.Clock,
		Center: models.MapResourcePosition{
			X: minitileToPixelInt(base.CenterTile.X),
			Y: minitileToPixelInt(base.CenterTile.Y),
		},
		Polygon:          polygon,
		MineralOnly:      base.MineralOnly,
		NaturalExpansion: base.NaturalExpansion,
	}
}

// scmapanalyzer replaymap.TilePoint values are in minitiles (8x8 px cells).
func minitileToPixelInt(minitileValue int) int {
	return minitileValue*8 + 4
}
