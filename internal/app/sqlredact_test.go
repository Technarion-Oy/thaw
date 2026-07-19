// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"strings"
	"testing"
)

func TestRedactSQLSecrets(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantMasked []string // substrings that must survive
		wantGone   []string // secret values that must NOT survive
	}{
		{
			name:       "CREATE SECRET password",
			in:         `CREATE SECRET s TYPE=PASSWORD USERNAME='u' PASSWORD='hunter2'`,
			wantMasked: []string{"PASSWORD='***'", "USERNAME='u'"},
			wantGone:   []string{"hunter2"},
		},
		{
			name:       "SECRET_STRING and OAUTH_REFRESH_TOKEN",
			in:         `ALTER SECRET s SET SECRET_STRING='abc' OAUTH_REFRESH_TOKEN='rt-99'`,
			wantMasked: []string{"SECRET_STRING='***'", "OAUTH_REFRESH_TOKEN='***'"},
			wantGone:   []string{"abc", "rt-99"},
		},
		{
			name:       "integration bearer/webhook/client secrets",
			in:         `CREATE INTEGRATION i OAUTH_CLIENT_SECRET='cs' BEARER_TOKEN='bt' WEBHOOK_SECRET='ws'`,
			wantMasked: []string{"OAUTH_CLIENT_SECRET='***'", "BEARER_TOKEN='***'", "WEBHOOK_SECRET='***'"},
			wantGone:   []string{"'cs'", "'bt'", "'ws'"},
		},
		{
			name:       "CREATE USER password",
			in:         `CREATE USER bob PASSWORD = 'p@ss'`,
			wantMasked: []string{"PASSWORD = '***'"},
			wantGone:   []string{"p@ss"},
		},
		{
			name:       "stage AWS secret key and master key",
			in:         `CREATE STAGE st CREDENTIALS=(AWS_KEY_ID='AKIA' AWS_SECRET_KEY='shh') ENCRYPTION=(MASTER_KEY='mk')`,
			wantMasked: []string{"AWS_KEY_ID='AKIA'", "AWS_SECRET_KEY='***'", "MASTER_KEY='***'"},
			wantGone:   []string{"'shh'", "'mk'"},
		},
		{
			name:       "escaped quote inside secret value",
			in:         `ALTER USER u SET PASSWORD='a''b'`,
			wantMasked: []string{"PASSWORD='***'"},
			wantGone:   []string{"a''b"},
		},
		{
			name:       "no secret is left untouched",
			in:         `SELECT * FROM t WHERE name = 'alice'`,
			wantMasked: []string{"name = 'alice'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSQLSecrets(tt.in)
			for _, want := range tt.wantMasked {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in redacted output, got: %s", want, got)
				}
			}
			for _, gone := range tt.wantGone {
				if strings.Contains(got, gone) {
					t.Errorf("secret %q leaked into redacted output: %s", gone, got)
				}
			}
		})
	}
}
