package service

import "testing"

func TestValidateSelectSQLAllowsSelect(t *testing.T) {
	got, err := ValidateSelectSQL(" /* ok */ SELECT id, name FROM users WHERE status = 'active'; ")
	if err != nil {
		t.Fatalf("expected select to pass: %v", err)
	}
	want := "SELECT id, name FROM users WHERE status = 'active'"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidateSelectSQLRejectsDangerousStatements(t *testing.T) {
	cases := []string{
		"UPDATE users SET role='admin'",
		"SELECT * FROM users; DROP TABLE users",
		"SELECT * FROM users FOR UPDATE",
		"SELECT * INTO OUTFILE '/tmp/a' FROM users",
	}
	for _, input := range cases {
		if _, err := ValidateSelectSQL(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}
