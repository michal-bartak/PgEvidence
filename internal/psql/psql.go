// Package psql wraps the system `psql` binary to execute a query and stream its
// CSV output to a file. It enforces reproducible, fail-fast behaviour and an
// optional read-only session. No password is ever written to disk: a session
// password, if provided, is passed only through the child process environment.
package psql

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"pgevidence/internal/proc"
)

// Conn carries the non-secret parts of a database connection.
type Conn struct {
	Host    string
	Port    int
	DBName  string
	User    string
	SSLMode string
}

// connString builds a libpq keyword/value connection string. Values are escaped
// per libpq rules (backslash and single quote), with the whole value quoted.
func (c Conn) connString() string {
	var parts []string
	add := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `'`, `\'`)
		parts = append(parts, fmt.Sprintf("%s='%s'", k, v))
	}
	add("host", c.Host)
	if c.Port > 0 {
		parts = append(parts, fmt.Sprintf("port=%d", c.Port))
	}
	add("dbname", c.DBName)
	add("user", c.User)
	add("sslmode", c.SSLMode)
	return strings.Join(parts, " ")
}

// Result is the outcome of running one query.
type Result struct {
	ExitCode int
	Stderr   string
	Duration time.Duration
}

// configuredPath is an optional user-set psql path (from config); empty = auto.
var configuredPath string

// SetPath sets a user override for the psql binary. Empty restores auto-detection.
func SetPath(p string) { configuredPath = p }

// binary resolves the psql executable: an explicit user override first, then PATH,
// then well-known install locations — necessary on macOS, where apps launched from
// Finder inherit only a minimal PATH that excludes Homebrew/Postgres.app/EDB dirs.
func binary() (string, error) {
	if configuredPath != "" && isExecutable(configuredPath) {
		return configuredPath, nil
	}
	if p, err := exec.LookPath("psql"); err == nil {
		return p, nil
	}
	for _, c := range candidatePaths() {
		if isExecutable(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("psql not found on PATH or in common install locations")
}

func candidatePaths() []string {
	name := "psql"
	if runtime.GOOS == "windows" {
		name = "psql.exe"
	}
	var fixed []string
	switch runtime.GOOS {
	case "darwin":
		fixed = []string{
			"/opt/homebrew/bin/" + name, // Apple-silicon Homebrew
			"/usr/local/bin/" + name,    // Intel Homebrew
			"/opt/local/bin/" + name,    // MacPorts
			"/usr/bin/" + name,
		}
	case "windows":
		// handled entirely by globs below
	default: // linux & others
		fixed = []string{
			"/usr/bin/" + name,
			"/usr/local/bin/" + name,
			"/opt/homebrew/bin/" + name,
		}
	}

	// Versioned installer locations (Postgres.app, EDB one-click, etc.).
	globs := []string{
		"/Applications/Postgres.app/Contents/Versions/*/bin/" + name,
		"/Library/PostgreSQL/*/bin/" + name,
		`C:\Program Files\PostgreSQL\*\bin\` + name,
		`C:\Program Files (x86)\PostgreSQL\*\bin\` + name,
	}
	for _, g := range globs {
		if matches, err := filepath.Glob(g); err == nil {
			fixed = append(fixed, matches...)
		}
	}
	return fixed
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

var versionRe = regexp.MustCompile(`\d+(\.\d+)*`)

// Detect returns the psql path and version string, or an error if psql is not
// installed.
func Detect() (path string, version string, err error) {
	path, err = binary()
	if err != nil {
		return "", "", fmt.Errorf("psql not found on PATH: %w", err)
	}
	cmd := exec.Command(path, "--version")
	proc.Hide(cmd)
	out, err := cmd.Output()
	if err != nil {
		return path, "", nil
	}
	return path, strings.TrimSpace(string(out)), nil
}

// childEnv builds the environment for the psql child process, layering in
// PGPASSWORD (when a session password is supplied) and the read-only GUC.
func childEnv(password string, readOnly bool) []string {
	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	if readOnly {
		// Set the GUC at connection startup so the entire session is read-only,
		// without contaminating the CSV output.
		opt := "-c default_transaction_read_only=on"
		merged := false
		for i, e := range env {
			if strings.HasPrefix(e, "PGOPTIONS=") {
				env[i] = e + " " + opt
				merged = true
				break
			}
		}
		if !merged {
			env = append(env, "PGOPTIONS="+opt)
		}
	}
	return env
}

func args(sql string) []string {
	return []string{
		"--no-psqlrc",
		"--csv",
		"-v", "ON_ERROR_STOP=1",
		"-c", sql,
	}
}

// RunToFile executes sql and writes the CSV result to the file at outPath.
// stdout is streamed straight to the file; stderr is captured for reporting.
func RunToFile(ctx context.Context, conn Conn, sql string, readOnly bool, password, outPath string) (Result, error) {
	bin, err := binary()
	if err != nil {
		return Result{}, fmt.Errorf("psql not found on PATH: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()

	cmd := exec.CommandContext(ctx, bin, append(args(sql), "-d", conn.connString())...)
	proc.Hide(cmd)
	cmd.Env = childEnv(password, readOnly)
	cmd.Stdout = f
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	res := Result{
		Stderr:   strings.TrimSpace(stderr.String()),
		Duration: time.Since(start),
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil {
		if res.Stderr != "" {
			return res, fmt.Errorf("%s", res.Stderr)
		}
		return res, runErr
	}
	return res, nil
}

// Test runs `SELECT 1` to validate connectivity and credentials.
func Test(ctx context.Context, conn Conn, password string) error {
	bin, err := binary()
	if err != nil {
		return fmt.Errorf("psql not found on PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, append(args("SELECT 1"), "-d", conn.connString())...)
	proc.Hide(cmd)
	cmd.Env = childEnv(password, false)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return fmt.Errorf("%s", s)
		}
		return err
	}
	return nil
}
