package parser

import (
	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/scmapanalyzer/replaymap"
	"github.com/marianogappa/screpdb/internal/models"
)

func buildMapContextLayoutFromReplay(replayPath string) (*models.MapContextLayout, error) {
	client, err := scmapanalyzer.NewClient()
	if err != nil {
		return nil, err
	}

	result, err := client.Analyze(replayPath)
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
	return &models.MapContextLayout{Bases: bases}, nil
}

func toContextBase(base replaymap.BasePolygon) models.MapContextBase {
	polygon := make([]models.MapPolygonPoint, 0, len(base.PolygonVertices))
	for _, vertex := range base.PolygonVertices {
		polygon = append(polygon, models.MapPolygonPoint{
			X: tileToPixelInt(vertex.X),
			Y: tileToPixelInt(vertex.Y),
		})
	}
	return models.MapContextBase{
		Name:  base.Name,
		Kind:  base.Kind,
		Clock: base.Clock,
		Center: models.MapResourcePosition{
			X: tileToPixelInt(base.CenterTile.X),
			Y: tileToPixelInt(base.CenterTile.Y),
		},
		Polygon:          polygon,
		MineralOnly:      base.MineralOnly,
		NaturalExpansion: base.NaturalExpansion,
	}
}

// scmapanalyzer replaymap.TilePoint values are in minitiles (8x8 px cells).
func tileToPixelInt(tileValue int) int {
	return tileValue*8 + 4
}
