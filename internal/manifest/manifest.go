// Package manifest records a tamper-evident description of a run: every query,
// its output files and checksum, timings and outcome, plus the (password-free)
// connection and tool versions. A run-level checksum over the manifest makes the
// whole evidence set verifiable.
package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"

	"pgevidence/internal/checksum"
)

// ConnInfo is the non-secret description of the target connection.
type ConnInfo struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	DBName  string `json:"dbName"`
	User    string `json:"user"`
	SSLMode string `json:"sslMode"`
}

// QueryRecord captures the result of running one query.
type QueryRecord struct {
	Index          int    `json:"index"`
	Name           string `json:"name"`
	SQL            string `json:"sql"`
	SQLFile        string `json:"sqlFile,omitempty"`
	ResultFile     string `json:"resultFile"`
	ChecksumFile   string `json:"checksumFile"`
	ScreenshotFile string `json:"screenshotFile,omitempty"`
	SHA256         string `json:"sha256"`
	RowCount       int    `json:"rowCount"`
	StartedAt      string `json:"startedAt"`
	EndedAt        string `json:"endedAt"`
	DurationMS     int64  `json:"durationMs"`
	Status         string `json:"status"` // "ok" | "error"
	Error          string `json:"error,omitempty"`
}

// Manifest is the top-level run record written to manifest.json.
type Manifest struct {
	AppVersion      string        `json:"appVersion"`
	PSQLVersion     string        `json:"psqlVersion"`
	Connection      ConnInfo      `json:"connection"`
	GeneratedAt     string        `json:"generatedAt"`
	RunDir          string        `json:"runDir"`
	DwellSeconds    int           `json:"dwellSeconds"`
	ReadOnly        bool          `json:"readOnly"`
	Screenshots     bool          `json:"screenshots"`
	Video           bool          `json:"video"`
	VideoFile       string        `json:"videoFile,omitempty"`
	Queries         []QueryRecord `json:"queries"`
}

// Write serialises the manifest to <dir>/manifest.json and writes a sibling
// run.sha256 checksum over the manifest bytes. It returns the manifest path.
func Write(dir string, m Manifest) (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return "", err
	}
	// Sidecar follows the same convention as result files: name.ext -> name.ext.sha256.
	sum := checksum.BytesSHA256(data)
	sumPath := manifestPath + ".sha256"
	line := sum + "  manifest.json\n"
	if err := os.WriteFile(sumPath, []byte(line), 0o644); err != nil {
		return manifestPath, err
	}
	return manifestPath, nil
}
