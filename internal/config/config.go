// Package config holds the application configuration and database connection
// definitions, persisted as JSON in the user config directory. No secrets are
// ever stored here — connection definitions carry no password.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
}

const appDirName = "audit-extractor"

// AppDir returns the application's config/state directory, creating it if
// necessary (e.g. ~/Library/Application Support/audit-extractor on macOS).
func AppDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Default returns a sensible default configuration. The output directory points
// at ~/audit-extracts.
func Default() Config {
	out := "audit-extracts"
	if home, err := os.UserHomeDir(); err == nil {
		out = filepath.Join(home, "audit-extracts")
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
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		_ = Save(cfg)
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

// Save writes the configuration to disk.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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
