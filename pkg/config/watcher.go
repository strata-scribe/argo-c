package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher provides a dynamic mechanism to watch files and reload their
// content when changed, using fsnotify. It watches the directory of the file
// to handle symlink swaps correctly (e.g. Kubernetes ConfigMaps/Secrets).
type FileWatcher struct {
	watcher *fsnotify.Watcher
	watches map[string]func([]byte)
	mu      sync.RWMutex
}

// NewFileWatcher creates and starts a new FileWatcher.
func NewFileWatcher(ctx context.Context) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	fw := &FileWatcher{
		watcher: watcher,
		watches: make(map[string]func([]byte)),
	}

	go fw.run(ctx)
	return fw, nil
}

// WatchFile adds a file to be watched. When the file is created or modified,
// the onUpdate callback is invoked with the new content.
func (fw *FileWatcher) WatchFile(path string, onUpdate func([]byte)) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// Always watch the directory to handle atomic swaps (symlinks changing)
	dir := filepath.Dir(absPath)
	if err := fw.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	fw.mu.Lock()
	fw.watches[absPath] = onUpdate
	fw.mu.Unlock()

	// Initial load
	content, err := os.ReadFile(absPath)
	if err == nil {
		onUpdate(content)
	} else if !os.IsNotExist(err) {
		log.Printf("failed to read file initially %s: %v", absPath, err)
	}

	return nil
}

// Close stops the watcher and cleans up resources.
func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}

func (fw *FileWatcher) run(ctx context.Context) {
	// Simple debouncing using a map of paths to pending updates
	debounceDuration := 50 * time.Millisecond
	debounceTimer := time.NewTimer(debounceDuration)
	debounceTimer.Stop()
	pending := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// We care about Write and Create, and perhaps Remove/Rename (but for secrets, usually Create/Write on symlinks or targets happen).
			// ConfigMap/Secret updates often involve Create/Remove/Rename/Chmod on a symlink or an underlying ..data directory.
			// Just debouncing events on the directory and reloading the target file if it's in our watches is a robust approach.
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Chmod) {
				absEventPath, err := filepath.Abs(event.Name)
				if err == nil {
					fw.mu.RLock()
					// Check if this exact file is watched, or if it's an event in a watched directory
					// Because we watch directories, event.Name might be a temp file (e.g., ..data_tmp)
					// So we just queue an update for all watched files in that directory.
					dir := filepath.Dir(absEventPath)
					for watchedPath := range fw.watches {
						if filepath.Dir(watchedPath) == dir {
							pending[watchedPath] = struct{}{}
						}
					}
					fw.mu.RUnlock()

					if len(pending) > 0 {
						debounceTimer.Reset(debounceDuration)
					}
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("fsnotify watcher error: %v", err)

		case <-debounceTimer.C:
			fw.mu.RLock()
			for path := range pending {
				if onUpdate, ok := fw.watches[path]; ok {
					content, err := os.ReadFile(path)
					if err == nil {
						onUpdate(content)
					}
				}
			}
			fw.mu.RUnlock()
			pending = make(map[string]struct{})
		}
	}
}
