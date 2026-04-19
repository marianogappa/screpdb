package dashboard

import (
	"strings"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/scmapanalyzer/replaymap"
	"github.com/marianogappa/screpdb/internal/models"
)

func buildDashboardMapContextLayoutFromReplay(replayPath string, mapName string) (*models.MapContextLayout, error) {
	client, err := scmapanalyzer.NewClient()
	if err != nil {
		return nil, err
	}

	opts := []scmapanalyzer.Option{}
	if strings.TrimSpace(mapName) != "" {
		opts = append(opts, scmapanalyzer.WithMapName(mapName))
	}

	result, err := client.Analyze(replayPath, opts...)
	if err != nil {
		return nil, err
	}

	bases := make([]models.MapContextBase, 0, len(result.Starts)+len(result.Expas))
	for _, base := range result.Starts {
		bases = append(bases, dashboardContextBaseFromAnalyzer(base))
	}
	for _, base := range result.Expas {
		bases = append(bases, dashboardContextBaseFromAnalyzer(base))
	}
	if len(bases) == 0 {
		return nil, nil
	}
	return &models.MapContextLayout{Bases: bases}, nil
}

func dashboardContextBaseFromAnalyzer(base replaymap.BasePolygon) models.MapContextBase {
	polygon := make([]models.MapPolygonPoint, 0, len(base.PolygonVertices))
	for _, vertex := range base.PolygonVertices {
		polygon = append(polygon, models.MapPolygonPoint{
			X: tileToPixelDashboard(vertex.X),
			Y: tileToPixelDashboard(vertex.Y),
		})
	}
	return models.MapContextBase{
		Name:  base.Name,
		Kind:  base.Kind,
		Clock: base.Clock,
		Center: models.MapResourcePosition{
			X: tileToPixelDashboard(base.CenterTile.X),
			Y: tileToPixelDashboard(base.CenterTile.Y),
		},
		Polygon:          polygon,
		MineralOnly:      base.MineralOnly,
		NaturalExpansion: base.NaturalExpansion,
	}
}

func tileToPixelDashboard(tileValue int) int {
	return tileValue*32 + 16
}
