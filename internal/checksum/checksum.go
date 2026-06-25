// Package checksum computes SHA-256 digests and writes sha256sum-compatible
// checksum files, so auditors can verify evidence with `sha256sum -c`.
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileSHA256 returns the lowercase hex SHA-256 of the file at path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BytesSHA256 returns the lowercase hex SHA-256 of b.
func BytesSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// WriteSidecar computes the SHA-256 of resultPath and writes a sibling
// "<resultPath>.sha256" file in coreutils format ("<hex>  <basename>\n").
// It returns the hex digest and the checksum file path.
func WriteSidecar(resultPath string) (sum string, sumPath string, err error) {
	sum, err = FileSHA256(resultPath)
	if err != nil {
		return "", "", err
	}
	sumPath = resultPath + ".sha256"
	line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(resultPath))
	if err := os.WriteFile(sumPath, []byte(line), 0o644); err != nil {
		return "", "", err
	}
	return sum, sumPath, nil
}
