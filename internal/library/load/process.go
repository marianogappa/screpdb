package load

import (
	"context"
	"errors"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/compact"
	"github.com/marianogappa/screpdb/internal/parser"
)

// errUnsupportedUMS marks a replay that parsed but is excluded by policy: use
// map settings games have no comparable build orders or matchups.
var errUnsupportedUMS = errors.New("replay skipped: use map settings is not supported")

type outcome uint8

const (
	outcomeLoaded outcome = iota
	outcomeAliased
	outcomeKnown
	outcomeExcluded
)

func (l *Loader) process(ctx context.Context, generation uint64, file fileops.FileInfo) (outcome, error) {
	if l.alreadyLoaded(file) {
		return outcomeKnown, nil
	}
	hashed, err := fileops.HashFiles(ctx, []fileops.FileInfo{file})
	if err != nil {
		return outcomeExcluded, err
	}
	file = hashed[0]
	checksum, err := library.ChecksumFromHex(file.Checksum)
	if err != nil {
		return outcomeExcluded, err
	}
	ref := library.FileRef{Path: file.Path, Size: file.Size, ModTime: file.ModTime}

	if l.claimChecksum(checksum) {
		l.parkAlias(checksum, ref)
		return outcomeAliased, nil
	}

	record, err := l.parse(file, checksum)
	if err != nil {
		l.releaseChecksum(checksum)
		return outcomeExcluded, err
	}
	l.lib.Add(generation, record)
	return outcomeLoaded, nil
}

func (l *Loader) parse(file fileops.FileInfo, checksum [32]byte) (*library.Replay, error) {
	replay := parser.CreateReplayFromFileInfo(file.Path, file.Name, file.Size, file.Checksum)
	data, err := parser.ParseReplayWithOptions(file.Path, replay, parser.Options{})
	if err != nil {
		return nil, err
	}
	if data.Replay != nil && data.Replay.MapKind == "UseMapSettings" {
		return nil, errUnsupportedUMS
	}
	return compact.FromReplayData(data, compact.FileMeta{
		Path:     file.Path,
		Size:     file.Size,
		ModTime:  file.ModTime,
		Checksum: checksum,
	})
}

// alreadyLoaded reports whether the corpus already holds this exact file. It is
// the cheap pass that makes a rescan of an unchanged folder nearly free: no
// hash, no parse.
func (l *Loader) alreadyLoaded(file fileops.FileInfo) bool {
	return knownFile(l.lib.Snapshot(), file.Path, file.Size, file.ModTime)
}

// claimChecksum reports whether this content is already accounted for, either
// in the corpus or by another worker in this run, and claims it otherwise so
// two copies of one replay are parsed once.
func (l *Loader) claimChecksum(checksum [32]byte) bool {
	if _, ok := l.lib.Snapshot().ByChecksum(checksum); ok {
		return true
	}
	_, claimed := l.seen.LoadOrStore(checksum, struct{}{})
	return claimed
}

func (l *Loader) releaseChecksum(checksum [32]byte) {
	l.seen.Delete(checksum)
}

// parkAlias holds a second file of an already-claimed replay until the parse
// that owns the content has been committed: the library drops an alias whose
// checksum it has not seen yet, and the two files are handled concurrently.
func (l *Loader) parkAlias(checksum [32]byte, ref library.FileRef) {
	l.aliasMu.Lock()
	defer l.aliasMu.Unlock()
	if l.aliases == nil {
		l.aliases = map[[32]byte][]library.FileRef{}
	}
	l.aliases[checksum] = append(l.aliases[checksum], ref)
}

func (l *Loader) flushAliases(generation uint64) {
	l.aliasMu.Lock()
	parked := l.aliases
	l.aliases = nil
	l.aliasMu.Unlock()
	if len(parked) == 0 {
		return
	}
	for checksum, refs := range parked {
		for _, ref := range refs {
			l.lib.Alias(generation, ref, checksum)
		}
	}
	l.lib.Flush()
}
