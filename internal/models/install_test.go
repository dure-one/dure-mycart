package models

import "testing"

func TestInstall_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      Install
		wantErr bool
	}{
		{"ok minimal", Install{Email: "admin@example.com", Password: "Str0ngPass!", DBType: "sqlite", SQLitePath: ":memory:"}, false},
		{"ok with domain", Install{Email: "admin@example.com", Password: "Str0ngPass!", Domain: "example.com", DBType: "sqlite", SQLitePath: ":memory:"}, false},
		{"missing email", Install{Password: "Str0ngPass!", DBType: "sqlite", SQLitePath: ":memory:"}, true},
		{"bad email", Install{Email: "not-an-email", Password: "Str0ngPass!", DBType: "sqlite", SQLitePath: ":memory:"}, true},
		{"short password", Install{Email: "admin@example.com", Password: "123", DBType: "sqlite", SQLitePath: ":memory:"}, true},
		{"password >72", Install{Email: "admin@example.com", Password: make73(), DBType: "sqlite", SQLitePath: ":memory:"}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.in.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// make73 produces a 73-character string used to exercise the max-length
// branch. Kept here (rather than inline) to make the test table terse.
func make73() string {
	b := make([]byte, 73)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
