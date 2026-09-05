package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileWatcher(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "secret.txt")

	// Create initial file
	err := os.WriteFile(filePath, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fw, err := NewFileWatcher(ctx)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer fw.Close()

	var mu sync.Mutex
	var updates [][]byte

	// Watch the file
	err = fw.WatchFile(filePath, func(content []byte) {
		mu.Lock()
		updates = append(updates, append([]byte{}, content...)) // copy content
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("failed to watch file: %v", err)
	}

	// Verify initial load
	mu.Lock()
	if len(updates) != 1 || string(updates[0]) != "initial" {
		t.Fatalf("expected initial load 'initial', got %v", updates)
	}
	mu.Unlock()

	// Update the file
	err = os.WriteFile(filePath, []byte("updated"), 0644)
	if err != nil {
		t.Fatalf("failed to write updated file: %v", err)
	}

	// Wait for the watcher to pick up the change (debouncer is 50ms, wait a bit longer)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(updates) < 2 {
		t.Fatalf("expected at least 2 updates, got %d", len(updates))
	}
	lastUpdate := string(updates[len(updates)-1])
	if lastUpdate != "updated" {
		t.Errorf("expected last update to be 'updated', got '%s'", lastUpdate)
	}
	mu.Unlock()
}
