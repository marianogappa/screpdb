package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
)

type columnInfo struct {
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type tableInfo struct {
	Columns     map[string]columnInfo `json:"columns"`
	ForeignKeys []foreignKeyInfo      `json:"foreign_keys"`
}

type foreignKeyInfo struct {
	Column     string `json:"column"`
	References string `json:"references"`
}

func (d *Dashboard) handlerSchema(w http.ResponseWriter, _ *http.Request) {
	tables := map[string]tableInfo{}
	targetTables := []string{"replays", "players", "commands", "detected_patterns_replay", "detected_patterns_replay_team", "detected_patterns_replay_player"}

	for _, tableName := range targetTables {
		rows, err := d.db.Query("PRAGMA table_info(" + tableName + ")")
		if err != nil {
			log.Printf("schema: failed to get table info for %s: %v", tableName, err)
			continue
		}

		columns := map[string]columnInfo{}
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var dfltValue *string
			var pk int
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				log.Printf("schema: failed to scan column for %s: %v", tableName, err)
				continue
			}
			columns[name] = columnInfo{
				Type:     colType,
				Nullable: notNull == 0,
			}
		}
		rows.Close()

		fkRows, err := d.db.Query("PRAGMA foreign_key_list(" + tableName + ")")
		var fks []foreignKeyInfo
		if err == nil {
			for fkRows.Next() {
				var id, seq int
				var refTable, from, to, onUpdate, onDelete, match string
				if err := fkRows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
					continue
				}
				fks = append(fks, foreignKeyInfo{
					Column:     from,
					References: refTable + "." + to,
				})
			}
			fkRows.Close()
		}
		if fks == nil {
			fks = []foreignKeyInfo{}
		}

		tables[tableName] = tableInfo{
			Columns:     columns,
			ForeignKeys: fks,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tables": tables})
}
