package tasks

import (
	"testing"

	"thaw/internal/snowflake"
)

func TestBareIdent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`"MY_TASK"`, "MY_TASK"},
		{`"my""task"`, `my"task`},    // embedded double-quote, surrounded
		{`my""task`, `my"task`},      // embedded double-quote, no surrounding quotes
		{`MY_TASK`, "MY_TASK"},        // no quotes at all
		{`""`, ""},                    // just quotes, empty name
		{`"a""b""c"`, `a"b"c`},       // multiple embedded quotes
	}
	for _, c := range cases {
		got := bareIdent(c.in)
		if got != c.want {
			t.Errorf("bareIdent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestQRoundTrip checks that snowflake.QuoteIdent() and bareIdent() are inverse operations.
func TestQRoundTrip(t *testing.T) {
	names := []string{
		"MY_TASK",
		`my"task`,
		`has""two`,
		`"leading`,
		`trailing"`,
	}
	for _, name := range names {
		quoted := snowflake.QuoteIdent(name)
		got := bareIdent(quoted)
		if got != name {
			t.Errorf("bareIdent(snowflake.QuoteIdent(%q)) = %q, want %q", name, got, name)
		}
	}
}
