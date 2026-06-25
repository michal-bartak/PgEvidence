// Package archive packages a run's output folder into a single ZIP file that
// lives inside the run folder. Encryption is optional and uses legacy ZipCrypto
// (chosen for maximum compatibility: macOS `unzip`, Windows Explorer, etc.).
//
// Security note: ZipCrypto is weak, and passwords are stored in plaintext (the
// explicit password in config.json, the auto password in the .pwd sidecar). This
// is packaging convenience, not strong secrecy.
package archive

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeka/zip"
)

// Result describes a created archive.
type Result struct {
	ZipPath   string `json:"zipPath"`
	PwdPath   string `json:"pwdPath"`
	Password  string `json:"password"`
	Encrypted bool   `json:"encrypted"`
	Mode      string `json:"mode"`
}

// Create zips every file in runDir into <runDir>/<base>.zip, where base is the
// run-folder name. When password is non-empty the entries are ZipCrypto-encrypted.
// The output zip and any *.zip / *.zip.pwd files are excluded from the archive.
func Create(runDir, password string) (string, error) {
	base := filepath.Base(runDir)
	zipPath := filepath.Join(runDir, base+".zip")

	out, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	zw := zip.NewWriter(out)

	entries, err := os.ReadDir(runDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".zip.pwd") {
			continue
		}
		if err := addFile(zw, filepath.Join(runDir, name), name, password); err != nil {
			_ = zw.Close()
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return zipPath, nil
}

func addFile(zw *zip.Writer, path, name, password string) error {
	var w io.Writer
	var err error
	if password != "" {
		w, err = zw.Encrypt(name, password, zip.StandardEncryption)
	} else {
		w, err = zw.Create(name)
	}
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

const pwAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// GeneratePassword returns a 20-character random password (crypto/rand,
// ambiguous-character-free alphabet).
func GeneratePassword() (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(pwAlphabet)))
	for i := 0; i < 20; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(pwAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// PruneSources deletes every file in runDir except the archive and its .pwd
// sidecar — used to leave only the ZIP after a successful archive. It refuses to
// run if the archive is missing or empty, so sources are never lost without a zip.
func PruneSources(runDir string) error {
	base := filepath.Base(runDir)
	zipPath := filepath.Join(runDir, base+".zip")
	info, err := os.Stat(zipPath)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("refusing to prune: archive missing or empty (%s)", zipPath)
	}
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".zip.pwd") {
			continue
		}
		if err := os.Remove(filepath.Join(runDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// WritePwdFile writes the password to "<zipPath>.pwd" and returns that path.
func WritePwdFile(zipPath, password string) (string, error) {
	pwdPath := zipPath + ".pwd"
	if err := os.WriteFile(pwdPath, []byte(password+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write pwd file: %w", err)
	}
	return pwdPath, nil
}
