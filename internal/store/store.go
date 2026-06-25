// Package store persists the set of SQL queries the operator maintains. Queries
// can be edited one at a time or replaced wholesale (import). They are stored as
// JSON in the application config directory.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"audit-extractor/internal/config"
)

// Query is a single named SQL statement to run against the database.
type Query struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	SQL     string `json:"sql"`
	Enabled bool   `json:"enabled"`
	Order   int    `json:"order"`
}

func path() (string, error) {
	dir, err := config.AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "queries.json"), nil
}

// List returns all queries, ordered by their Order field.
func List() ([]Query, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return []Query{}, nil
	}
	if err != nil {
		return nil, err
	}
	var qs []Query
	if err := json.Unmarshal(data, &qs); err != nil {
		return nil, err
	}
	normalize(qs)
	return qs, nil
}

func save(qs []Query) error {
	p, err := path()
	if err != nil {
		return err
	}
	normalize(qs)
	data, err := json.MarshalIndent(qs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// normalize sorts by Order then renumbers Order to be contiguous (0..n-1).
func normalize(qs []Query) {
	sort.SliceStable(qs, func(i, j int) bool { return qs[i].Order < qs[j].Order })
	for i := range qs {
		qs[i].Order = i
	}
}

// Upsert inserts a new query (when ID is empty) or updates an existing one,
// returning the stored query and the full list.
func Upsert(q Query) ([]Query, error) {
	qs, err := List()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(q.Name) == "" {
		q.Name = "Untitled query"
	}
	if q.ID == "" {
		q.ID = uuid.NewString()
		q.Order = len(qs)
		qs = append(qs, q)
	} else {
		found := false
		for i := range qs {
			if qs[i].ID == q.ID {
				q.Order = qs[i].Order
				qs[i] = q
				found = true
				break
			}
		}
		if !found {
			q.Order = len(qs)
			qs = append(qs, q)
		}
	}
	if err := save(qs); err != nil {
		return nil, err
	}
	return List()
}

// Delete removes the query with the given ID and returns the remaining list.
func Delete(id string) ([]Query, error) {
	qs, err := List()
	if err != nil {
		return nil, err
	}
	out := qs[:0]
	for _, q := range qs {
		if q.ID != id {
			out = append(out, q)
		}
	}
	if err := save(out); err != nil {
		return nil, err
	}
	return List()
}

// Move shifts a query up (delta=-1) or down (delta=+1) in the ordering.
func Move(id string, delta int) ([]Query, error) {
	qs, err := List()
	if err != nil {
		return nil, err
	}
	idx := -1
	for i := range qs {
		if qs[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return qs, nil
	}
	j := idx + delta
	if j < 0 || j >= len(qs) {
		return qs, nil
	}
	qs[idx], qs[j] = qs[j], qs[idx]
	for i := range qs {
		qs[i].Order = i
	}
	if err := save(qs); err != nil {
		return nil, err
	}
	return List()
}

// ReplaceAll overwrites the entire query set.
func ReplaceAll(qs []Query) ([]Query, error) {
	for i := range qs {
		if qs[i].ID == "" {
			qs[i].ID = uuid.NewString()
		}
		qs[i].Order = i
	}
	if err := save(qs); err != nil {
		return nil, err
	}
	return List()
}

// Export returns the query set as indented JSON suitable for saving to a file.
func Export() (string, error) {
	qs, err := List()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(qs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Import parses the supplied text and replaces the whole query set. The text may
// be either a JSON array of queries (as produced by Export) or a plain SQL
// script, in which case it is split on semicolons into individual queries.
func Import(text string) ([]Query, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("nothing to import")
	}
	if strings.HasPrefix(trimmed, "[") {
		var qs []Query
		if err := json.Unmarshal([]byte(trimmed), &qs); err != nil {
			return nil, fmt.Errorf("invalid JSON query set: %w", err)
		}
		return ReplaceAll(qs)
	}
	return ReplaceAll(parseSQLScript(trimmed))
}

// parseSQLScript splits a SQL script into queries on top-level semicolons,
// skipping blank statements and line/block comments when deriving names.
func parseSQLScript(script string) []Query {
	var qs []Query
	for _, stmt := range splitStatements(script) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || !hasSQLContent(stmt) {
			continue // skip blank chunks and comment-only trailers
		}
		qs = append(qs, Query{
			Name:    deriveName(stmt, len(qs)+1),
			SQL:     stmt,
			Enabled: true,
		})
	}
	return qs
}

// splitStatements splits on semicolons that are not inside single/double quotes,
// dollar-quoted strings, or comments (-- line and /* block */). Keeping comments
// intact lets deriveName use the text just before a query as its name.
func splitStatements(s string) []string {
	var out []string
	var b strings.Builder
	var inSingle, inDouble, inLine, inBlock bool
	var dollarTag string
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLine {
			b.WriteRune(c)
			if c == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			b.WriteRune(c)
			if c == '*' && next == '/' {
				b.WriteRune(next)
				i++
				inBlock = false
			}
			continue
		}
		if dollarTag != "" {
			b.WriteRune(c)
			if c == '$' {
				// look for closing tag
				rest := string(runes[i:])
				if strings.HasPrefix(rest, dollarTag) {
					b.WriteString(dollarTag[1:])
					i += len(dollarTag) - 1
					dollarTag = ""
				}
			}
			continue
		}
		switch {
		case inSingle:
			b.WriteRune(c)
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			b.WriteRune(c)
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
			b.WriteRune(c)
		case c == '"':
			inDouble = true
			b.WriteRune(c)
		case c == '-' && next == '-':
			inLine = true
			b.WriteRune(c)
		case c == '/' && next == '*':
			inBlock = true
			b.WriteRune(c)
		case c == '$':
			// detect a dollar-quote opening tag like $$ or $tag$
			rest := string(runes[i:])
			if tag := dollarOpen(rest); tag != "" {
				dollarTag = tag
				b.WriteString(tag)
				i += len(tag) - 1
			} else {
				b.WriteRune(c)
			}
		case c == ';':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteRune(c)
		}
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}

// dollarOpen returns the dollar-quote tag (e.g. "$$" or "$body$") if s starts
// with one, otherwise "".
func dollarOpen(s string) string {
	if len(s) < 2 || s[0] != '$' {
		return ""
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '$' {
			return s[:i+1]
		}
		if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return ""
		}
	}
	return ""
}

// deriveName names a statement after the comment immediately preceding the query
// (the text just before it). With several leading comment lines it uses the last
// one (closest to the query). With no comment it falls back to the first SQL line,
// then to a numbered default.
func deriveName(stmt string, n int) string {
	lastComment := ""
	for _, raw := range strings.Split(stmt, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if isComment(line) {
			if t := commentText(line); t != "" {
				lastComment = t
			}
			continue
		}
		// First real SQL line.
		if lastComment != "" {
			return truncateName(lastComment)
		}
		return truncateName(line)
	}
	if lastComment != "" {
		return truncateName(lastComment)
	}
	return fmt.Sprintf("Query %d", n)
}

func isComment(line string) bool {
	return strings.HasPrefix(line, "--") || strings.HasPrefix(line, "/*")
}

// commentText strips comment markers (-- or /* */) and surrounding noise.
func commentText(line string) string {
	if strings.HasPrefix(line, "--") {
		return strings.TrimSpace(strings.TrimLeft(line, "-"))
	}
	t := strings.TrimPrefix(line, "/*")
	t = strings.TrimSuffix(t, "*/")
	return strings.TrimSpace(strings.Trim(t, "*"))
}

// hasSQLContent reports whether a statement has any non-blank, non-comment line.
func hasSQLContent(stmt string) bool {
	for _, raw := range strings.Split(stmt, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || isComment(line) {
			continue
		}
		return true
	}
	return false
}

func truncateName(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 80 {
		s = strings.TrimSpace(string(r[:80]))
	}
	return s
}
