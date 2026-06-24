package sqlgrammar

import "testing"

// TestZeroOrMoreZeroProgressGuard verifies that a ZeroOrMore body which succeeds
// without consuming a token terminates (instead of spinning forever and hanging
// the diagnostics goroutine). If the guard regresses, this test hangs.
func TestZeroOrMoreZeroProgressGuard(t *testing.T) {
	v := New("a b c")
	calls := 0
	ok := v.ZeroOrMore(func() bool {
		calls++
		// Optional always succeeds and, when "ZZZ" doesn't match, consumes nothing.
		return v.Optional(func() bool { return v.MatchWord("ZZZ") })
	})
	if !ok {
		t.Fatal("ZeroOrMore must always succeed")
	}
	if calls != 1 {
		t.Errorf("zero-progress body should run once then stop, ran %d times", calls)
	}
	if v.save() != 0 {
		t.Errorf("a non-consuming ZeroOrMore must not advance the cursor, pos=%d", v.save())
	}
}

// TestZeroOrMoreStillLoops guards against the progress check ending the loop too
// early: a body that DOES consume must keep iterating to exhaustion.
func TestZeroOrMoreStillLoops(t *testing.T) {
	v := New("x x x y")
	v.ZeroOrMore(func() bool { return v.MatchWord("x") })
	if got := v.Peek().Text(v.src); got != "y" {
		t.Errorf("expected the loop to consume every 'x' and stop at 'y', stopped at %q", got)
	}
}
