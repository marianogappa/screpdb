package library

import "sync"

// Table is an append-only, process-wide string interner. Id 0 is always the
// empty string so zero-valued record fields read back as "".
type Table struct {
	mu    sync.RWMutex
	names []string
	ids   map[string]uint16
}

const maxTableEntries = 1 << 16

func NewTable() *Table {
	return &Table{names: []string{""}, ids: map[string]uint16{"": 0}}
}

// Intern returns the id for s, assigning a new one when unseen. A table that
// has exhausted the uint16 space returns 0 (the empty string) rather than
// failing; no table in practice holds more than a few thousand entries.
func (t *Table) Intern(s string) uint16 {
	t.mu.RLock()
	id, ok := t.ids[s]
	t.mu.RUnlock()
	if ok {
		return id
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.ids[s]; ok {
		return id
	}
	if len(t.names) >= maxTableEntries {
		return 0
	}
	id = uint16(len(t.names))
	t.names = append(t.names, s)
	t.ids[s] = id
	return id
}

// Lookup returns the id for s without interning it.
func (t *Table) Lookup(s string) (uint16, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	id, ok := t.ids[s]
	return id, ok
}

// Name returns the string for id; unknown ids read as "".
func (t *Table) Name(id uint16) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if int(id) >= len(t.names) {
		return ""
	}
	return t.names[id]
}

func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.names)
}

var (
	// Units holds unit type names (Train / Morph / Build subjects, attack units).
	Units = NewTable()
	// Techs holds research names.
	Techs = NewTable()
	// Upgrades holds upgrade names.
	Upgrades = NewTable()
	// Orders holds spell-cast order names (only those in gamerules.CompositionSpells).
	Orders = NewTable()
	// EventTypes holds worldstate game-event type names.
	EventTypes = NewTable()
	// Features holds marker feature keys.
	Features = NewTable()
	// Strings holds low-cardinality free text: map names, game types,
	// matchups, leave reasons, fuzzy opener labels, colors, hosts.
	Strings = NewTable()
)
