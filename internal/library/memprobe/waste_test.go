package memprobe

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"unsafe"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
)

func TestCapacityWasteByField(t *testing.T) {
	folder := os.Getenv("PROBE_FOLDER")
	if folder == "" {
		t.Skip("set PROBE_FOLDER")
	}
	if err := iofacade.AllowDir(folder); err != nil {
		t.Fatal(err)
	}
	lib := library.New(library.Options{})
	defer lib.Close()
	if err := load.New(lib, load.Options{Folder: folder, Generation: 1}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap := lib.Snapshot()
	n := snap.Len()
	waste := map[string]int{}
	for _, r := range snap.Replays {
		w := func(field string, l, c, elem int) { waste[field] += (c - l) * elem }
		w("Paths", len(r.Paths), cap(r.Paths), int(unsafe.Sizeof(library.FileRef{})))
		w("Players", len(r.Players), cap(r.Players), int(unsafe.Sizeof(library.Player{})))
		w("Markers", len(r.Markers), cap(r.Markers), int(unsafe.Sizeof(library.Marker{})))
		w("Events", len(r.Events), cap(r.Events), int(unsafe.Sizeof(library.GameEvent{})))
		w("Chat", len(r.Chat), cap(r.Chat), int(unsafe.Sizeof(library.ChatLine{})))
		w("Prod.Sec", len(r.Prod.Sec), cap(r.Prod.Sec), 2)
		w("Prod.Subject", len(r.Prod.Subject), cap(r.Prod.Subject), 2)
		w("Prod.Player", len(r.Prod.Player), cap(r.Prod.Player), 1)
		w("Prod.Kind", len(r.Prod.Kind), cap(r.Prod.Kind), 1)
		w("Prod.Count", len(r.Prod.Count), cap(r.Prod.Count), 1)
		for _, m := range r.Markers {
			w("Marker.Payload", len(m.Payload), cap(m.Payload), 1)
		}
		for i := range r.Players {
			p := &r.Players[i]
			w("HotkeyStream", len(p.HotkeyStream), cap(p.HotkeyStream), 1)
			if p.Fingerprint != nil {
				w("Fingerprint.Vector", len(p.Fingerprint.Vector), cap(p.Fingerprint.Vector), 8)
			}
		}
		if r.Alliance != nil {
			w("Alliance", len(r.Alliance.Snapshots), cap(r.Alliance.Snapshots), int(unsafe.Sizeof(library.AllianceSnapshot{})))
		}
	}
	type kv struct {
		k string
		v int
	}
	var rows []kv
	total := 0
	for k, v := range waste {
		if v != 0 {
			rows = append(rows, kv{k, v})
		}
		total += v
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].v > rows[j].v })
	fmt.Printf("\nPROBE-WASTE total %.2f KB/replay\n", float64(total)/float64(n)/1024)
	for _, row := range rows {
		fmt.Printf("  %-20s %7.2f KB/replay\n", row.k, float64(row.v)/float64(n)/1024)
	}
}
