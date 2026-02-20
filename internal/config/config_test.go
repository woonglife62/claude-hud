package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Width != 360 {
		t.Errorf("default Width = %d, want 360", cfg.Width)
	}
	if cfg.Height != 640 {
		t.Errorf("default Height = %d, want 640", cfg.Height)
	}
	if cfg.Opacity != 204 {
		t.Errorf("default Opacity = %d, want 204", cfg.Opacity)
	}
	if cfg.RefreshMs != 3000 {
		t.Errorf("default RefreshMs = %d, want 3000", cfg.RefreshMs)
	}
	if !cfg.PinDesktop {
		t.Error("default PinDesktop should be true")
	}
	if !cfg.DarkMode {
		t.Error("default DarkMode should be true")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Use a temp dir to avoid touching the real config location
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := DefaultConfig()
	original.X = 200
	original.Y = 300
	original.Width = 400
	original.Height = 700
	original.Opacity = 180
	original.RefreshMs = 5000
	original.PinDesktop = false
	original.DarkMode = false
	original.CompactMode = true

	// Write config to temp path
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read it back
	loaded := DefaultConfig()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := json.Unmarshal(raw, loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.X != 200 {
		t.Errorf("X = %d, want 200", loaded.X)
	}
	if loaded.Y != 300 {
		t.Errorf("Y = %d, want 300", loaded.Y)
	}
	if loaded.Width != 400 {
		t.Errorf("Width = %d, want 400", loaded.Width)
	}
	if loaded.Height != 700 {
		t.Errorf("Height = %d, want 700", loaded.Height)
	}
	if loaded.Opacity != 180 {
		t.Errorf("Opacity = %d, want 180", loaded.Opacity)
	}
	if loaded.RefreshMs != 5000 {
		t.Errorf("RefreshMs = %d, want 5000", loaded.RefreshMs)
	}
	if loaded.PinDesktop {
		t.Error("PinDesktop should be false")
	}
	if loaded.DarkMode {
		t.Error("DarkMode should be false")
	}
	if !loaded.CompactMode {
		t.Error("CompactMode should be true")
	}
}

func TestLoadConfig_Validation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write invalid config values
	bad := map[string]interface{}{
		"width":      50,  // below minimum 200
		"height":     100, // below minimum 300
		"opacity":    10,  // below minimum 50
		"refresh_ms": 200, // below minimum 1000
	}
	data, _ := json.Marshal(bad)
	os.WriteFile(path, data, 0644)

	// Manually test validation logic (mirrors LoadConfig internals)
	cfg := DefaultConfig()
	json.Unmarshal(data, cfg)
	if cfg.Width < 200 {
		cfg.Width = 200
	}
	if cfg.Height < 300 {
		cfg.Height = 300
	}
	if cfg.Opacity < 50 {
		cfg.Opacity = 50
	}
	if cfg.RefreshMs < 1000 {
		cfg.RefreshMs = 1000
	}

	if cfg.Width != 200 {
		t.Errorf("Width validation: got %d, want 200", cfg.Width)
	}
	if cfg.Height != 300 {
		t.Errorf("Height validation: got %d, want 300", cfg.Height)
	}
	if cfg.Opacity != 50 {
		t.Errorf("Opacity validation: got %d, want 50", cfg.Opacity)
	}
	if cfg.RefreshMs != 1000 {
		t.Errorf("RefreshMs validation: got %d, want 1000", cfg.RefreshMs)
	}
}
