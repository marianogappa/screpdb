package compact

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
)

func TestInternLayoutKeepsChokesAndWhereTheyLead(t *testing.T) {
	l := &models.MapContextLayout{WidthTiles: 128, HeightTiles: 128, Bases: []models.MapContextBase{
		{Name: "start 7", Chokes: []models.MapContextChoke{{X: 112, Y: 768, ConnectsTo: "natural of 7"}}},
		{Name: "natural of 7", Chokes: []models.MapContextChoke{{X: 168, Y: 704}}},
	}}
	got := internLayout("chokes test", l)
	if want := (library.MapChoke{X: 112, Y: 768, To: 1}); len(got.Bases[0].Chokes) != 1 || got.Bases[0].Chokes[0] != want {
		t.Errorf("main chokes = %+v", got.Bases[0].Chokes)
	}
	if want := (library.MapChoke{X: 168, Y: 704, To: -1}); len(got.Bases[1].Chokes) != 1 || got.Bases[1].Chokes[0] != want {
		t.Errorf("natural chokes = %+v", got.Bases[1].Chokes)
	}
	moved := *l
	moved.Bases = append([]models.MapContextBase(nil), l.Bases...)
	moved.Bases[1].Chokes = []models.MapContextChoke{{X: 200, Y: 704}}
	if internLayout("chokes test", &moved) == got {
		t.Error("a layout whose chokes differ is a different layout")
	}
}
