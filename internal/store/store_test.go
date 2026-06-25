package store

import "testing"

func TestParseSQLScript_Names(t *testing.T) {
	script := `
-- Active users in last 30 days
SELECT * FROM users WHERE last_login > now() - interval '30 days';

-- section header
-- Orders summary
SELECT count(*) FROM orders;

SELECT 1 AS ok;

/* Block comment name */
SELECT 2;

-- a comment mentioning ; a semicolon
SELECT 3;
`
	qs := parseSQLScript(script)
	if len(qs) != 5 {
		t.Fatalf("expected 5 queries, got %d: %+v", len(qs), names(qs))
	}
	want := []string{
		"Active users in last 30 days", // leading comment
		"Orders summary",               // last of several comment lines
		"SELECT 1 AS ok",               // no comment -> first SQL line
		"Block comment name",           // /* */ comment
		"a comment mentioning ; a semicolon", // ';' inside comment must not split
	}
	for i, w := range want {
		if qs[i].Name != w {
			t.Errorf("query %d name = %q, want %q", i, qs[i].Name, w)
		}
	}
}

func TestParseSQLScript_SkipsCommentOnlyTrailer(t *testing.T) {
	qs := parseSQLScript("SELECT 1;\n-- just a trailing note\n")
	if len(qs) != 1 {
		t.Fatalf("expected 1 query, got %d: %+v", len(qs), names(qs))
	}
}

func names(qs []Query) []string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q.Name
	}
	return out
}
