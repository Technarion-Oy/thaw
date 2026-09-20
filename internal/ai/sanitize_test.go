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
		{name: "partial word repetition", prefix: "sel", reply: "select 1", want: "ect 1"},
		{name: "whole multiline prefix restated", prefix: "select *\nfrom ", reply: "select *\nfrom orders", want: "orders"},
		{name: "clean completion untouched", prefix: "select * from ", reply: "orders o join customers c on c.id = o.cid", want: "orders o join customers c on c.id = o.cid"},
		{name: "echo only", prefix: "select ", reply: "select ", want: ""},
		{name: "empty prefix", prefix: "", reply: "select 1", want: "select 1"},
	}
	for _, tc := range cases {
		if got := Sanitize(tc.prefix, tc.reply); got != tc.want {
			t.Errorf("%s: Sanitize(%q, %q) = %q, want %q", tc.name, tc.prefix, tc.reply, got, tc.want)
		}
	}
}
