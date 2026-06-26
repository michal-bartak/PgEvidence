// Package config holds the application configuration and database connection
// definitions, persisted as JSON in the user config directory. No secrets are
// ever stored here — connection definitions carry no password.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// mu serializes all config read-modify-write so concurrent writers (Settings
// auto-save, the window-resize handler, theme changes) can't clobber each other.
var mu sync.Mutex

// Connection describes how to reach a PostgreSQL database. It deliberately
// contains no password: the password is supplied at run time via ~/.pgpass or
// an in-memory session prompt.
type Connection struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	DBName  string `json:"dbName"`
	User    string `json:"user"`
	SSLMode string `json:"sslMode"`
}

// Config is the full persisted application configuration.
type Config struct {
	Connections          []Connection `json:"connections"`
	SelectedConnectionID string       `json:"selectedConnectionId"`
	DwellSeconds         int          `json:"dwellSeconds"`
	OutputDir            string       `json:"outputDir"`
	PreviewRows          int          `json:"previewRows"`
	EnforceReadOnly      bool         `json:"enforceReadOnly"`
	Screenshots          bool         `json:"screenshots"`
	Video                bool         `json:"video"`
	MonitorIndex         int          `json:"monitorIndex"`
	StopOnError          bool         `json:"stopOnError"`
	Theme                string       `json:"theme"`        // "system" | "light" | "dark"
	PsqlPath             string       `json:"psqlPath"`     // override; empty = auto-detect
	FfmpegPath           string       `json:"ffmpegPath"`   // override; empty = auto-detect
	SaveQuerySQL         bool         `json:"saveQuerySQL"` // write NNNN_slug.sql per query
	Zip                  bool         `json:"zip"`
	ZipPasswordMode      string       `json:"zipPasswordMode"` // "none" | "explicit" | "auto"
	ZipPassword          string       `json:"zipPassword"`     // explicit mode only; stored plaintext
	DeleteSourcesAfterZip bool        `json:"deleteSourcesAfterZip"`
	ExcludeVideoFromZip  bool         `json:"excludeVideoFromZip"` // keep run.mp4 out of the archive (and out of prune)
	WindowWidth          int          `json:"windowWidth,omitempty"`  // last OS window size; 0 = use default
	WindowHeight         int          `json:"windowHeight,omitempty"`
	ScreenAccessPrompted bool         `json:"screenAccessPrompted,omitempty"` // macOS: have we shown the TCC prompt once?
}

// appDirName is the human-readable config/state dir (paths show "PgEvidence", not
// lowercase). legacyAppDirNames are migrated to it once (incl. a case fix on
// case-insensitive filesystems).
const appDirName = "PgEvidence"

var legacyAppDirNames = []string{"pgevidence", "audit-extractor"}

// AppDir returns the application's config/state directory, creating it if
// necessary (e.g. ~/Library/Application Support/PgEvidence on macOS), migrating an
// older-named dir if present.
func AppDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	migrateAppDir(base, dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// migrateAppDir renames a legacy app dir to the canonical name. On case-insensitive
// filesystems the legacy "pgevidence" resolves to the same dir as "PgEvidence"; the
// os.SameFile check lets us rename it purely to fix the displayed case without
// clobbering a genuinely distinct directory.
func migrateAppDir(base, dir string) {
	for _, ln := range legacyAppDirNames {
		legacy := filepath.Join(base, ln)
		if legacy == dir {
			continue
		}
		lfi, e := os.Stat(legacy)
		if e != nil || !lfi.IsDir() {
			continue
		}
		if dfi, de := os.Stat(dir); de == nil {
			if os.SameFile(lfi, dfi) {
				_ = os.Rename(legacy, dir) // case-only fix
			}
			continue // a distinct canonical dir already exists — don't clobber
		}
		_ = os.Rename(legacy, dir)
	}
}

func configPath() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Default returns a sensible default configuration. The output directory points
// at ~/PgEvidence.
func Default() Config {
	out := "PgEvidence"
	if home, err := os.UserHomeDir(); err == nil {
		out = filepath.Join(home, "PgEvidence")
	}
	return Config{
		Connections: []Connection{{
			ID:      "default",
			Name:    "Local PostgreSQL",
			Host:    "localhost",
			Port:    5432,
			DBName:  "postgres",
			User:    "postgres",
			SSLMode: "prefer",
		}},
		SelectedConnectionID: "default",
		DwellSeconds:         5,
		OutputDir:            out,
		PreviewRows:          20,
		EnforceReadOnly:      true,
		Screenshots:          true,
		Video:                false,
		MonitorIndex:         0,
		StopOnError:          false,
		Theme:                 "system",
		SaveQuerySQL:          true,
		Zip:                   true,
		ZipPasswordMode:       "none",
		ZipPassword:           "",
		DeleteSourcesAfterZip: false,
	}
}

// Load reads the configuration from disk, falling back to defaults (and writing
// them) when no config file exists yet.
func Load() (Config, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// Save writes the configuration to disk.
func Save(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(cfg)
}

// Update performs an atomic read-modify-write under the config lock: it loads the
// current config, applies mutate, and saves the result. Use this for partial
// updates (window size, theme) so they don't race a full Save.
func Update(mutate func(*Config)) error {
	mu.Lock()
	defer mu.Unlock()
	cfg, err := loadLocked()
	if err != nil {
		return err
	}
	mutate(&cfg)
	return saveLocked(cfg)
}

func loadLocked() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		_ = saveLocked(cfg)
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// saveLocked writes the config atomically: marshal to a temp file in the same
// dir, then rename over the target so a crash mid-write can't leave a torn file.
func saveLocked(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// FindConnection returns the connection with the given ID, or false.
func (c Config) FindConnection(id string) (Connection, bool) {
	for _, conn := range c.Connections {
		if conn.ID == id {
			return conn, true
		}
	}
	return Connection{}, false
}
