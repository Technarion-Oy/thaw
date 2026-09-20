// SPDX-License-Identifier: GPL-3.0-or-later

package ai

import "testing"

func TestSanitize(t *testing.T) {
	cases := []struct {
		name, prefix, reply, want string
	}{
		{
			name:   "fenced reply repeating the prefix (#926)",
			prefix: "alter ",
			reply:  "```sql\nalter table \"DB\".\"S\".\"T\" add column \"C\" varchar(100);\n```",
			want:   "table \"DB\".\"S\".\"T\" add column \"C\" varchar(100);",
		},
		{name: "unclosed fence", prefix: "select ", reply: "```sql\nselect * from orders", want: "* from orders"},
		{name: "case-insensitive repetition", prefix: "SELECT ", reply: "select * from orders", want: "* from orders"},
		{name: "partial word repetition", prefix: "sel", reply: "select 1", want: "ect 1"}, //nolint:misspell // half a SQL keyword, not a typo
		{name: "whole multiline prefix restated", prefix: "select *\nfrom ", reply: "select *\nfrom orders", want: "orders"},
		{name: "clean completion untouched", prefix: "select * from ", reply: "orders o join customers c on c.id = o.cid", want: "orders o join customers c on c.id = o.cid"},
		{name: "echo only", prefix: "select ", reply: "select ", want: ""},
		{name: "empty prefix", prefix: "", reply: "select 1", want: "select 1"},
		{name: "one-line fence keeps its first word", prefix: "", reply: "```select 1```", want: "select 1"},

		// A suffix starting mid-word is not a repetition: these completed
		// correctly before the sanitiser existed and must keep doing so.
		{name: "double letter ac+count", prefix: "select * from ac", reply: "count", want: "count"},
		{name: "double letter ad+dress", prefix: "select * from ad", reply: "dress", want: "dress"},
		{name: "double letter of+fset", prefix: "select * from of", reply: "fset", want: "fset"},
		{name: "double letter clas+s", prefix: "select * from clas", reply: "s", want: "s"}, //nolint:misspell // half an identifier, not a typo

		// Providers hand their text over untrimmed, so Sanitize sees what the
		// model actually sent — including a pure echo and a leading separator.
		{name: "untrimmed echo", prefix: "select ", reply: "  select \n", want: ""},
		{name: "leading space kept mid-word", prefix: "select * from orders", reply: " where id = 1", want: " where id = 1"},
		{name: "leading newline collapses to one space", prefix: "select * from orders", reply: "\nwhere id = 1", want: " where id = 1"},
		{name: "no separator added after whitespace", prefix: "select * from ", reply: "\n  orders", want: "orders"},
	}
	for _, tc := range cases {
		if got := Sanitize(tc.prefix, tc.reply); got != tc.want {
			t.Errorf("%s: Sanitize(%q, %q) = %q, want %q", tc.name, tc.prefix, tc.reply, got, tc.want)
		}
	}
}
