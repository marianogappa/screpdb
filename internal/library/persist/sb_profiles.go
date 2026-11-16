package persist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

const sbProfilesFile = "profiles.json"

// SBProfile is a ShieldBattery profile as last fetched. Unlike game records,
// profiles keep changing, so callers decide from FetchedAt when to refetch.
type SBProfile struct {
	Profile   sbfacade.Profile `json:"profile"`
	FetchedAt time.Time        `json:"fetched_at"`
}

type sbProfilesFileShape struct {
	Profiles map[uint32]SBProfile `json:"profiles"`
}

// SBProfiles stores ShieldBattery profiles at
// <root>/shieldbattery/profiles.json, keyed by user id.
type SBProfiles struct {
	root   string
	mu     sync.Mutex
	loaded *sbProfilesFileShape
}

func NewSBProfiles(root string) *SBProfiles {
	return &SBProfiles{root: root}
}

func (s *SBProfiles) path() string {
	return filepath.Join(s.root, SBGamesDirName, sbProfilesFile)
}

func (s *SBProfiles) Get(userID uint32) (SBProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.fileLocked()
	if err != nil {
		return SBProfile{}, false, err
	}
	profile, ok := file.Profiles[userID]
	return profile, ok, nil
}

func (s *SBProfiles) Put(userID uint32, profile SBProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.fileLocked()
	if err != nil {
		return err
	}
	file.Profiles[userID] = profile
	return writeJSONAtomically(s.path(), file)
}

func (s *SBProfiles) fileLocked() (*sbProfilesFileShape, error) {
	if s.loaded != nil {
		return s.loaded, nil
	}
	file := &sbProfilesFileShape{Profiles: map[uint32]SBProfile{}}
	raw, err := iofacade.ReadFile(s.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		// A damaged file only costs refetching: every profile can be asked
		// for again.
		var parsed sbProfilesFileShape
		if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Profiles != nil {
			file = &parsed
		}
	}
	s.loaded = file
	return file, nil
}
