package store

import (
	"strings"
	"testing"
)

// The user's real format: a free-text description (possibly multi-line, no SQL
// comment markers) precedes each query; the query starts at a SQL keyword.
func TestParseSQLScript_PlainTextNames(t *testing.T) {
	script := `A list of login-enabled users who are members of selected privileged built-in
PostgreSQL role
SELECT u.rolname AS user_name, r.rolname AS privileged_role
FROM pg_auth_members am
JOIN pg_roles r ON am.roleid = r.oid
WHERE u.rolcanlogin = true;
A list of all PostgreSQL roles and their key security attributes
SELECT rolname, rolsuper FROM pg_roles ORDER BY rolname;`

	qs := parseSQLScript(script)
	if len(qs) != 2 {
		t.Fatalf("expected 2 queries, got %d: %+v", len(qs), names(qs))
	}

	if qs[0].Name != "A list of login-enabled users who are members of selected privileged built-in PostgreSQL role" {
		t.Errorf("q0 name = %q", qs[0].Name)
	}
	if !strings.HasPrefix(qs[0].SQL, "SELECT u.rolname") {
		t.Errorf("q0 SQL should start at SELECT, got: %q", qs[0].SQL)
	}
	if strings.Contains(qs[0].SQL, "login-enabled users") {
		t.Errorf("q0 SQL still contains the name text: %q", qs[0].SQL)
	}

	if qs[1].Name != "A list of all PostgreSQL roles and their key security attributes" {
		t.Errorf("q1 name = %q", qs[1].Name)
	}
	if qs[1].SQL != "SELECT rolname, rolsuper FROM pg_roles ORDER BY rolname" {
		t.Errorf("q1 SQL not clean: %q", qs[1].SQL)
	}
}

func TestParseSQLScript_CommentAndBareNames(t *testing.T) {
	script := `-- Comment name
SELECT 1;
SELECT 2 AS plain;`
	qs := parseSQLScript(script)
	if len(qs) != 2 {
		t.Fatalf("expected 2, got %d: %+v", len(qs), names(qs))
	}
	if qs[0].Name != "Comment name" || qs[0].SQL != "SELECT 1" {
		t.Errorf("q0 = (%q, %q)", qs[0].Name, qs[0].SQL)
	}
	// No leading text -> name falls back to the first SQL line, SQL intact.
	if qs[1].Name != "SELECT 2 AS plain" || qs[1].SQL != "SELECT 2 AS plain" {
		t.Errorf("q1 = (%q, %q)", qs[1].Name, qs[1].SQL)
	}
}

func names(qs []Query) []string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q.Name
	}
	return out
}
