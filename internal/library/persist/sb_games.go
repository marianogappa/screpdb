package persist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	SBGamesDirName = "shieldbattery"
	sbGamesFile    = "games.json"
)

// SBGame is ShieldBattery's settled record of one game. Only settled records
// are stored, because a settled game never changes and so never needs asking
// about again.
type SBGame struct {
	// Results maps a ShieldBattery user id to "win", "loss", "draw" or
	// "unknown".
	Results   map[uint32]string `json:"results,omitempty"`
	Canceled  bool              `json:"canceled,omitempty"`
	FetchedAt time.Time         `json:"fetched_at"`
}

type sbGamesFileShape struct {
	Games map[string]SBGame `json:"games"`
}

// SBGames stores settled ShieldBattery game records at
// <root>/shieldbattery/games.json, keyed by game id.
type SBGames struct {
	root   string
	mu     sync.Mutex
	loaded *sbGamesFileShape
}

func NewSBGames(root string) *SBGames {
	return &SBGames{root: root}
}

func (s *SBGames) path() string {
	return filepath.Join(s.root, SBGamesDirName, sbGamesFile)
}

func (s *SBGames) Get(gameID string) (SBGame, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.fileLocked()
	if err != nil {
		return SBGame{}, false, err
	}
	game, ok := file.Games[gameID]
	return game, ok, nil
}

func (s *SBGames) Put(gameID string, game SBGame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.fileLocked()
	if err != nil {
		return err
	}
	file.Games[gameID] = game
	return s.writeLocked(file)
}

func (s *SBGames) fileLocked() (*sbGamesFileShape, error) {
	if s.loaded != nil {
		return s.loaded, nil
	}
	file := &sbGamesFileShape{Games: map[string]SBGame{}}
	raw, err := iofacade.ReadFile(s.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		// A damaged file only costs refetching: every record can be asked for
		// again.
		var parsed sbGamesFileShape
		if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Games != nil {
			file = &parsed
		}
	}
	s.loaded = file
	return file, nil
}

func (s *SBGames) writeLocked(file *sbGamesFileShape) error {
	path := s.path()
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := iofacade.WriteFile(temp, raw, 0o644); err != nil {
		return err
	}
	return iofacade.Rename(temp, path)
}
