// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "regexp"

// secretLiteralRe matches a secret-bearing SQL assignment — an identifier whose
// name contains one of the sensitive substrings (SECRET, TOKEN, PASSWORD,
// PASSPHRASE, CREDENTIAL), or the exact MASTER_KEY, followed by `= '<literal>'`.
// The single-quoted literal may embed doubled '' escapes. Group 1 is the
// keyword and the run of whitespace/`=` up to the opening quote, which is kept
// verbatim so only the value is masked.
//
// This deliberately over-matches (e.g. a column literally named TOKEN_COUNT):
// masking a non-secret value in a diagnostic log is harmless, under-masking a
// real credential is not.
var secretLiteralRe = regexp.MustCompile(
	`(?i)([A-Za-z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSPHRASE|CREDENTIAL)[A-Za-z0-9_]*|MASTER_KEY)(\s*=\s*)'(?:[^']|'')*'`,
)

// redactSQLSecrets masks single-quoted values assigned to secret-bearing
// keywords so credential DDL (CREATE/ALTER SECRET, integration OAuth/bearer/
// webhook secrets, CREATE/ALTER USER … PASSWORD=…, key PASSPHRASE, stage
// CREDENTIALS/MASTER_KEY, …) can be written to the on-disk application log
// without leaking the secret itself. The statement is still injection-safe
// before this runs; redaction protects the disk artifact (thaw.log is
// gzip-rotated and retained up to 30 days — exactly what a user attaches to a
// bug report). See issue #804 F1.
func redactSQLSecrets(sql string) string {
	return secretLiteralRe.ReplaceAllString(sql, "$1$2'***'")
}
