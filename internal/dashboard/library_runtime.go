package dashboard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/legacyimport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/library/watch"
	"github.com/marianogappa/screpdb/internal/sampledata"
)

// libraryRuntime owns everything the dashboard needs to answer reads from the
// replay folder: the in-memory corpus, the loader and watcher that keep it in
// step with disk, and the JSON files that hold the little state which is not
// derivable from the replays themselves.
type libraryRuntime struct {
	root     string
	lib      *library.Library
	settings *dashboarddb.FileSettings
	bnet     *persist.BnetCache
	games    *persist.BnetGameArchive
	store    *dashboarddb.LibStore
	manager  *load.Manager
	hub      *libraryHub

	// sampleSetAutoLoaded records that no replay folder could be found and the
	// bundled example replays were used instead, so the browser can say so.
	sampleSetAutoLoaded bool
}

type libraryRuntimeOptions struct {
	// Root is the app-data directory holding settings.json, the Battle.net
	// caches and the extracted example replays.
	Root string
	// ReplayDir overrides the persisted folder, for headless callers.
	ReplayDir string
	// LegacyDBPath is the pre-library database to import settings and the
	// Battle.net caches out of on the first run after upgrading. Optional.
	LegacyDBPath string
	Hub          *libraryHub
}

func newLibraryRuntime(ctx context.Context, opts libraryRuntimeOptions) (*libraryRuntime, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return nil, errors.New("library runtime needs an app-data root")
	}
	if err := iofacade.AllowDir(root); err != nil {
		return nil, err
	}

	bnet := persist.NewBnetCache(root)
	games := persist.NewBnetGameArchive(root)
	sweepLegacyDatabase(root, opts.LegacyDBPath)
	if err := importLegacyState(ctx, root, opts.LegacyDBPath, bnet, games); err != nil {
		log.Printf("Could not carry over the previous settings: %v", err)
	}
	if err := bnet.Load(); err != nil {
		log.Printf("Could not read the cached Battle.net profiles: %v", err)
	}
	if err := games.Load(); err != nil {
		log.Printf("Could not read the Battle.net game archive: %v", err)
	}
	if profiles, migrated, err := persist.MigrateLegacyBnetCaches(root, bnet, games); err != nil {
		log.Printf("Could not migrate the pre-v2 Battle.net caches: %v", err)
	} else if profiles > 0 || migrated > 0 {
		log.Printf("Migrated %d Battle.net profiles and %d archived games out of the pre-v2 caches", profiles, migrated)
	}

	lib := library.New(library.Options{})
	settings, err := dashboarddb.NewFileSettings(root, lib)
	if err != nil {
		lib.Close()
		return nil, err
	}

	store := dashboarddb.NewLibStore(lib, bnet, settings)
	store.SetGameArchive(games)

	runtime := &libraryRuntime{
		root:     root,
		lib:      lib,
		settings: settings,
		bnet:     bnet,
		games:    games,
		store:    store,
		hub:      opts.Hub,
	}

	folder, err := runtime.resolveFolder(ctx, opts.ReplayDir)
	if err != nil {
		lib.Close()
		return nil, err
	}

	runtime.manager = load.NewManager(lib, load.ManagerOptions{
		Folder: folder,
		Loader: load.Options{Log: runtime.log, MaxReplays: settings.Settings().MaxReplays},
		Watch:  watch.Options{},
		Log:    runtime.log,
	})
	if opts.Hub != nil {
		opts.Hub.Watch(lib)
	}
	return runtime, nil
}

// resolveFolder decides which folder the corpus mirrors: an explicit override,
// then the persisted choice, then the user's StarCraft folder, and finally the
// bundled example replays so a first run still has something to explore.
func (r *libraryRuntime) resolveFolder(ctx context.Context, override string) (string, error) {
	if dir := strings.TrimSpace(override); dir != "" {
		if err := r.settings.SetIngestInputDir(ctx, globalReplayFilterConfigKey, dir); err != nil {
			return "", err
		}
		_ = iofacade.AllowDir(dir)
		return dir, nil
	}

	stored := strings.TrimSpace(r.settings.Settings().ReplayFolder)
	if stored != "" {
		_ = iofacade.AllowDir(stored)
		if err := fileops.ValidateReplayDir(stored); err == nil {
			return stored, nil
		}
		log.Printf("The saved replay folder %s is no longer readable; looking for another one", stored)
	}

	if detected, err := fileops.ResolveDefaultReplayDir(); err == nil {
		if err := r.settings.SetIngestInputDir(ctx, globalReplayFilterConfigKey, detected); err != nil {
			return "", err
		}
		log.Printf("Using the StarCraft replay folder at %s", detected)
		return detected, nil
	}

	sample := r.sampleSetDir()
	if err := sampledata.Extract(sample); err != nil {
		return "", fmt.Errorf("extract the example replays: %w", err)
	}
	if err := r.settings.SetIngestInputDir(ctx, globalReplayFilterConfigKey, sample); err != nil {
		return "", err
	}
	r.sampleSetAutoLoaded = true
	log.Printf("No StarCraft replay folder found; using the example replays in %s", sample)
	return sample, nil
}

func (r *libraryRuntime) sampleSetDir() string {
	dir := filepath.Join(r.root, sampleSetDirName)
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// Start begins loading the folder and watching it. It returns as soon as the
// load is under way.
func (r *libraryRuntime) Start(ctx context.Context) error { return r.manager.Start(ctx) }

// SetFolder points the corpus at a different folder, keeping the current one
// on screen until the newest replays of the new one have loaded.
func (r *libraryRuntime) SetFolder(ctx context.Context, folder string) error {
	folder = strings.TrimSpace(folder)
	_ = iofacade.AllowDir(folder)
	if err := fileops.ValidateReplayDir(folder); err != nil {
		return err
	}
	if err := r.settings.SetIngestInputDir(ctx, globalReplayFilterConfigKey, folder); err != nil {
		return err
	}
	r.sampleSetAutoLoaded = false
	return r.manager.SetFolder(ctx, folder)
}

// UseSampleSet extracts the bundled example replays and switches to them.
func (r *libraryRuntime) UseSampleSet(ctx context.Context) (string, error) {
	dir := r.sampleSetDir()
	if err := sampledata.Extract(dir); err != nil {
		return "", fmt.Errorf("extract the example replays: %w", err)
	}
	if err := r.SetFolder(ctx, dir); err != nil {
		return "", err
	}
	return dir, nil
}

// Rescan re-reads the folder now instead of waiting for the next check.
func (r *libraryRuntime) Rescan() { r.manager.Rescan() }

func (r *libraryRuntime) Folder() string { return r.manager.Folder() }

func (r *libraryRuntime) IsSampleSetActive() bool {
	return samePath(r.Folder(), r.sampleSetDir())
}

func (r *libraryRuntime) Close() {
	if r.manager != nil {
		r.manager.Close()
	}
	if r.lib != nil {
		r.lib.Close()
	}
}

func (r *libraryRuntime) log(event load.LogEvent) {
	if r.hub != nil {
		r.hub.Log(event)
	}
}

// importLegacyState carries the previous release's settings and Battle.net
// caches out of the database once, so the first run after upgrading keeps the
// user's folder and does not re-spend the rate-limited profile budget. The
// database is opened read-only and is never written to.
func importLegacyState(ctx context.Context, root, legacyDBPath string, bnet *persist.BnetCache, games *persist.BnetGameArchive) error {
	if strings.TrimSpace(legacyDBPath) == "" {
		return nil
	}
	if _, found, err := persist.LoadSettings(root); err != nil || found {
		return err
	}
	legacy, err := legacyimport.Read(ctx, legacyDBPath)
	if errors.Is(err, legacyimport.ErrNoDatabase) {
		return nil
	}
	if err != nil {
		return err
	}

	var profiles []persist.BnetProfile
	var archived []persist.BnetGame
	for _, profile := range legacy.Profiles {
		fetchedAt, err := parseLegacyTime(profile.FetchedAt)
		if err != nil {
			continue
		}
		distilled, profileGames := persist.DistillBnetProfile(profile.Toon, int64(profile.Gateway), fetchedAt, []byte(profile.Payload))
		if !distilled.Found && profile.Found {
			distilled.Found = true
			distilled.AuroraID = profile.AuroraID
			distilled.BattleTag = profile.BattleTag
			distilled.CountryCode = profile.CountryCode
		}
		profiles = append(profiles, distilled)
		archived = append(archived, profileGames...)
	}
	if err := bnet.UpsertBatch(profiles); err != nil {
		return err
	}

	for _, game := range legacy.GameResults {
		converted := persist.BnetGame{
			GameID:     game.GameID,
			CreateTime: unixTime(game.CreateTimeUnix),
			Gateway:    game.Gateway,
			Ladder:     game.MatchGUID != "",
			MapName:    game.MapName,
			Accounts:   []int64{game.AuroraID},
		}
		if toon := strings.TrimSpace(game.Toon); toon != "" {
			converted.Players = []persist.BnetGamePlayer{{
				Toon:    toon,
				Race:    strings.ToLower(strings.TrimSpace(game.Race)),
				Result:  persist.BnetResultFromString(game.Result),
				APM:     game.APM,
				Seconds: game.DurationSeconds,
			}}
		}
		archived = append(archived, converted)
	}
	if err := games.Upsert(archived); err != nil {
		return err
	}

	// SaveSettings is last: the settings file is the sentinel that gates this
	// import (and the sweep), so writing it only after every cache file has
	// landed guarantees a retry on partial failure.
	if legacy.Settings != nil {
		next := persist.DefaultSettings()
		next.ReplayFolder = legacy.Settings.ReplayDir
		next.FeatureFlags = legacy.Settings.FeatureFlags
		filter := library.FilterConfig{
			GameTypes:         legacy.Settings.GameTypes,
			ExcludeShortGames: legacy.Settings.ExcludeShortGames,
			ExcludeComputers:  legacy.Settings.ExcludeComputers,
			MapKinds:          legacy.Settings.MapKinds,
		}
		if normalized, err := filter.Normalize(); err == nil {
			next.GlobalFilter = normalized
		}
		if err := persist.SaveSettings(root, next); err != nil {
			return err
		}
	}
	log.Printf("Carried over %d Battle.net profiles and %d game results from the previous version", len(legacy.Profiles), len(legacy.GameResults))
	return nil
}

// sweepLegacyDatabase removes the pre-library SQLite files once the one-time
// import has succeeded. It is gated on settings.json existing: that file is
// written last by importLegacyState, so its presence proves the caches have
// already been carried over. Running the sweep before the import means the two
// never execute on the same launch.
func sweepLegacyDatabase(root, legacyDBPath string) {
	if strings.TrimSpace(legacyDBPath) == "" {
		return
	}
	if _, found, err := persist.LoadSettings(root); err != nil || !found {
		return
	}
	targets := []string{
		legacyDBPath,
		legacyDBPath + "-wal",
		legacyDBPath + "-shm",
	}
	var freed int64
	for _, path := range targets {
		freed += removeAndMeasure(path)
	}
	if freed > 0 {
		log.Printf("Removed the legacy database, freeing %.1f MB", float64(freed)/(1024*1024))
	}
}

func removeAndMeasure(path string) int64 {
	info, err := iofacade.Stat(path)
	if err != nil {
		return 0
	}
	size := info.Size()
	if err := iofacade.Remove(path); err != nil {
		log.Printf("Could not remove %s: %v", path, err)
		return 0
	}
	return size
}

func parseLegacyTime(value string) (time.Time, error) { return time.Parse(time.RFC3339, value) }

func unixTime(seconds int64) time.Time { return time.Unix(seconds, 0).UTC() }
