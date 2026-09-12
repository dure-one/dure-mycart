package database

import "testing"

func TestRebindDollar(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"single", "SELECT * FROM t WHERE a = ?", "SELECT * FROM t WHERE a = $1"},
		{"ordered", "WHERE a = ? AND b = ? AND c = ?", "WHERE a = $1 AND b = $2 AND c = $3"},
		{"none", "SELECT 1", "SELECT 1"},
		{"empty", "", ""},
		{
			"single quoted string is untouched",
			`SELECT 'who?' FROM t WHERE a = ?`,
			`SELECT 'who?' FROM t WHERE a = $1`,
		},
		{
			"doubled quote inside a string does not end it",
			`SELECT 'it''s ? here' FROM t WHERE a = ?`,
			`SELECT 'it''s ? here' FROM t WHERE a = $1`,
		},
		{
			"double quoted identifier is untouched",
			`SELECT "odd?name" FROM t WHERE a = ?`,
			`SELECT "odd?name" FROM t WHERE a = $1`,
		},
		{
			"backtick identifier is untouched",
			"SELECT `odd?name` FROM t WHERE a = ?",
			"SELECT `odd?name` FROM t WHERE a = $1",
		},
		{
			"line comment is untouched",
			"SELECT a\n-- why? because\nFROM t WHERE a = ?",
			"SELECT a\n-- why? because\nFROM t WHERE a = $1",
		},
		{
			"block comment is untouched",
			"SELECT a /* why? */ FROM t WHERE a = ?",
			"SELECT a /* why? */ FROM t WHERE a = $1",
		},
		{
			"unterminated string still suppresses rewriting",
			"SELECT 'oops ?",
			"SELECT 'oops ?",
		},
		{
			"percent signs survive",
			"SELECT * FROM t WHERE a LIKE ? AND b = ?",
			"SELECT * FROM t WHERE a LIKE $1 AND b = $2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rebindDollar(tt.query); got != tt.want {
				t.Errorf("rebindDollar() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSQLiteRebindIsIdentity(t *testing.T) {
	const query = "SELECT * FROM t WHERE a = ? AND b = 'x?y'"

	if got := SQLite().Rebind(query); got != query {
		t.Errorf("sqlite Rebind() = %q, want the query unchanged", got)
	}
}
