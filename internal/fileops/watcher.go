package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches for new replay files
type FileWatcher struct {
	watcher *fsnotify.Watcher
	dir     string
	events  chan FileInfo
	errors  chan error
	done    chan bool
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(dir string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &FileWatcher{
		watcher: watcher,
		dir:     dir,
		events:  make(chan FileInfo, 100),
		errors:  make(chan error, 100),
		done:    make(chan bool),
	}, nil
}

// Start begins watching for file changes
func (fw *FileWatcher) Start() error {
	// Add the main directory
	if err := fw.watcher.Add(fw.dir); err != nil {
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	// Add all subdirectories
	err := filepath.Walk(fw.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fw.watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add subdirectories to watcher: %w", err)
	}

	go fw.watch()
	return nil
}

// Events returns the channel for new file events
func (fw *FileWatcher) Events() <-chan FileInfo {
	return fw.events
}

// Errors returns the channel for errors
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// Stop stops the watcher
func (fw *FileWatcher) Stop() {
	close(fw.done)
	fw.watcher.Close()
}

// watch is the main watching loop
func (fw *FileWatcher) watch() {
	defer close(fw.events)
	defer close(fw.errors)

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				if strings.HasSuffix(strings.ToLower(event.Name), ".rep") {
					// Wait a bit for the file to be fully written
					time.Sleep(100 * time.Millisecond)

					fileInfo, err := fw.getFileInfo(event.Name)
					if err != nil {
						select {
						case fw.errors <- err:
						case <-fw.done:
							return
						}
						continue
					}

					select {
					case fw.events <- *fileInfo:
					case <-fw.done:
						return
					}
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			select {
			case fw.errors <- err:
			case <-fw.done:
				return
			}

		case <-fw.done:
			return
		}
	}
}

// getFileInfo gets file information for a newly created file
func (fw *FileWatcher) getFileInfo(filePath string) (*FileInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	checksum, err := calculateChecksum(filePath)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Path:     filePath,
		Name:     info.Name(),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Checksum: checksum,
	}, nil
}
