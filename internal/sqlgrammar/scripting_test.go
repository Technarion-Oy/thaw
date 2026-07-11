package sqlgrammar

import "testing"

func TestParseAwait(t *testing.T) {
	assertValid(t, (*Validator).ParseAwait,
		`AWAIT ALL`,
		`await all`, // case-insensitive
		`AWAIT my_result_set`,
		`AWAIT "My Result Set"`,
	)
	assertInvalid(t, (*Validator).ParseAwait,
		`AWAIT`,                // missing target
		`AWAIT ALL extra`,      // trailing token
		`AWAIT my_set another`, // two names
		`WAIT ALL`,             // wrong keyword
	)
}
