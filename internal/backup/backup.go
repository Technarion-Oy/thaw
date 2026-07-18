// SPDX-License-Identifier: GPL-3.0-or-later

package backup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"thaw/internal/snowflake"
)

// BackupPolicyRow holds one row from SHOW BACKUP POLICIES.
type BackupPolicyRow struct {
	Name            string `json:"name"`
	CreatedOn       string `json:"createdOn"`
	Owner           string `json:"owner"`
	Schedule        string `json:"schedule"`
	ExpireAfterDays int64  `json:"expireAfterDays"`
	RetentionLock   bool   `json:"retentionLock"`
	Comment         string `json:"comment"`
}

// BackupRow holds one row from SHOW BACKUPS IN BACKUP SET.
type BackupRow struct {
	ID        string `json:"id"`   // UUID used in IDENTIFIER clause of CREATE ... FROM BACKUP SET
	Name      string `json:"name"` // human-readable name / timestamp label
	CreatedOn string `json:"createdOn"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"sizeBytes"`
	Comment   string `json:"comment"`
}

// BackupSetRow holds one row from SHOW BACKUP SETS.
type BackupSetRow struct {
	Name            string `json:"name"`
	BackupSetDb     string `json:"backupSetDb"`
	BackupSetSchema string `json:"backupSetSchema"`
	CreatedOn       string `json:"createdOn"`
	ObjectType      string `json:"objectType"`
	ObjectName      string `json:"objectName"`
	ObjectDb        string `json:"objectDb"`
	ObjectSchema    string `json:"objectSchema"`
	Status          string `json:"status"`
	Comment         string `json:"comment"`
}

// bsFQN builds a (possibly fully-qualified) backup set identifier.
func bsFQN(name, bsDb, bsSchema string) string {
	if bsDb != "" && bsSchema != "" {
		return snowflake.Qualify(bsDb, bsSchema, name)
	}
	if bsDb != "" {
		return snowflake.Qualify(bsDb, name)
	}
	return snowflake.QuoteIdent(name)
}

func cell(row []interface{}, idx int) interface{} {
	if idx < 0 || idx >= len(row) {
		return nil
	}
	return row[idx]
}

// BuildListBackupSetsSql builds SHOW BACKUP SETS, optionally filtered by a
// case-insensitive name substring, scoped to the whole account.
func BuildListBackupSetsSql(nameFilter string) string {
	var sb strings.Builder
	sb.WriteString("SHOW BACKUP SETS")
	if strings.TrimSpace(nameFilter) != "" {
		// EscapeLikePattern escapes both the single-quote and the LIKE wildcards
		// (% and _) so a literal wildcard in the typed name matches literally,
		// while the surrounding %…% remain the substring wildcards.
		escapedFilter := snowflake.EscapeLikePattern(nameFilter)
		sb.WriteString(fmt.Sprintf(" LIKE '%%%s%%'", escapedFilter))
	}
	sb.WriteString(" IN ACCOUNT")
	return sb.String()
}

// ParseBackupSets converts SHOW BACKUP SETS rows into BackupSetRow values,
// post-filtering by the backed-up object so only backup sets actually covering
// the specified database, schema, or table are returned.
func ParseBackupSets(res *snowflake.QueryResult, scopeType, db, schema, table string) ([]BackupSetRow, error) {
	if res == nil {
		return []BackupSetRow{}, nil
	}
	nameIdx := snowflake.ColIdx(res.Columns, "name")
	bsDbIdx := snowflake.ColIdx(res.Columns, "database_name")
	bsSchIdx := snowflake.ColIdx(res.Columns, "schema_name")
	createdIdx := snowflake.ColIdx(res.Columns, "created_on")
	otypeIdx := snowflake.ColIdx(res.Columns, "object_kind")
	onameIdx := snowflake.ColIdx(res.Columns, "object_name")
	objDbIdx := snowflake.ColIdx(res.Columns, "object_database_name")
	objSchIdx := snowflake.ColIdx(res.Columns, "object_schema_name")
	statusIdx := snowflake.ColIdx(res.Columns, "backup_policy_state", "status")
	commentIdx := snowflake.ColIdx(res.Columns, "comment")

	upperScope := strings.ToUpper(scopeType)
	rows := make([]BackupSetRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		otype := strings.ToUpper(snowflake.CellString(cell(row, otypeIdx)))
		oname := snowflake.CellString(cell(row, onameIdx))
		objDb := snowflake.CellString(cell(row, objDbIdx))
		objSch := snowflake.CellString(cell(row, objSchIdx))

		// Post-filter: only include backup sets whose backed-up object matches
		// the right-clicked item.
		var match bool
		switch upperScope {
		case "DATABASE":
			match = otype == "DATABASE" && strings.EqualFold(oname, db)
		case "SCHEMA":
			match = otype == "SCHEMA" &&
				strings.EqualFold(objDb, db) &&
				strings.EqualFold(oname, schema)
		case "TABLE":
			match = (otype == "TABLE" || otype == "EXTERNAL TABLE") &&
				strings.EqualFold(objDb, db) &&
				strings.EqualFold(objSch, schema) &&
				strings.EqualFold(oname, table)
		default:
			return nil, fmt.Errorf("unsupported scope: %s", scopeType)
		}
		if !match {
			continue
		}

		// Use the backup set's own database/schema exactly as SHOW reports them.
		// If SHOW returns them empty we leave them empty so follow-up calls emit
		// a bare quoted name resolved against the session context — do NOT fall
		// back to the backed-up object's database (db), which is unrelated to
		// where the backup set itself lives and would build a wrong FQN.
		rowBsDb := snowflake.CellString(cell(row, bsDbIdx))
		rowBsSch := snowflake.CellString(cell(row, bsSchIdx))
		rows = append(rows, BackupSetRow{
			Name:            snowflake.CellString(cell(row, nameIdx)),
			BackupSetDb:     rowBsDb,
			BackupSetSchema: rowBsSch,
			CreatedOn:       snowflake.CellString(cell(row, createdIdx)),
			ObjectType:      otype,
			ObjectName:      oname,
			ObjectDb:        objDb,
			ObjectSchema:    objSch,
			Status:          snowflake.CellString(cell(row, statusIdx)),
			Comment:         snowflake.CellString(cell(row, commentIdx)),
		})
	}
	return rows, nil
}

// ParseBackupPolicies converts SHOW BACKUP POLICIES rows into BackupPolicyRow values.
func ParseBackupPolicies(res *snowflake.QueryResult) []BackupPolicyRow {
	if res == nil {
		return []BackupPolicyRow{}
	}
	nameIdx := snowflake.ColIdx(res.Columns, "name")
	createdIdx := snowflake.ColIdx(res.Columns, "created_on")
	ownerIdx := snowflake.ColIdx(res.Columns, "owner")
	schedIdx := snowflake.ColIdx(res.Columns, "schedule")
	expireIdx := snowflake.ColIdx(res.Columns, "expire_after_days")
	lockIdx := snowflake.ColIdx(res.Columns, "retention_lock", "with_retention_lock")
	commentIdx := snowflake.ColIdx(res.Columns, "comment")

	rows := make([]BackupPolicyRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, BackupPolicyRow{
			Name:            snowflake.CellString(cell(row, nameIdx)),
			CreatedOn:       snowflake.CellString(cell(row, createdIdx)),
			Owner:           snowflake.CellString(cell(row, ownerIdx)),
			Schedule:        snowflake.CellString(cell(row, schedIdx)),
			ExpireAfterDays: snowflake.CellInt64(cell(row, expireIdx)),
			RetentionLock:   snowflake.CellBool(cell(row, lockIdx)),
			Comment:         snowflake.CellString(cell(row, commentIdx)),
		})
	}
	return rows
}

// ParseBackups converts SHOW BACKUPS IN BACKUP SET rows into BackupRow values.
func ParseBackups(res *snowflake.QueryResult) []BackupRow {
	if res == nil {
		return []BackupRow{}
	}
	// Snowflake internally uses "snapshot" terminology; column names vary by version.
	idIdx := snowflake.ColIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	nameIdx := snowflake.ColIdx(res.Columns, "name", "backup_name", "snapshot_name", "backup", "snapshot")
	createdIdx := snowflake.ColIdx(res.Columns, "created_on")
	statusIdx := snowflake.ColIdx(res.Columns, "status")
	sizeIdx := snowflake.ColIdx(res.Columns, "size_bytes", "size")
	commentIdx := snowflake.ColIdx(res.Columns, "comment")

	rows := make([]BackupRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		nameVal := snowflake.CellString(cell(row, nameIdx))
		// If no dedicated name column was found, fall back to created_on — Snowflake
		// uses the creation timestamp as the backup identifier in DROP BACKUP.
		if nameVal == "" {
			nameVal = snowflake.CellString(cell(row, createdIdx))
		}
		rows = append(rows, BackupRow{
			ID:        snowflake.CellString(cell(row, idIdx)),
			Name:      nameVal,
			CreatedOn: snowflake.CellString(cell(row, createdIdx)),
			Status:    snowflake.CellString(cell(row, statusIdx)),
			SizeBytes: snowflake.CellInt64(cell(row, sizeIdx)),
			Comment:   snowflake.CellString(cell(row, commentIdx)),
		})
	}
	return rows
}

// FindOldestEligibleBackup returns the ID of the oldest backup in the set that
// is not under legal hold. ok is false when no eligible backup exists.
func FindOldestEligibleBackup(res *snowflake.QueryResult) (id string, ok bool) {
	if res == nil {
		return "", false
	}
	idIdx := snowflake.ColIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	createdIdx := snowflake.ColIdx(res.Columns, "created_on")
	legalHoldIdx := snowflake.ColIdx(res.Columns, "is_under_legal_hold", "legal_hold", "under_legal_hold")

	bestID := ""
	bestCreated := ""
	var bestTime time.Time
	bestTimeOK := false
	for _, row := range res.Rows {
		if snowflake.CellBool(cell(row, legalHoldIdx)) {
			continue
		}
		rowID := snowflake.CellString(cell(row, idIdx))
		if rowID == "" {
			continue
		}
		created := snowflake.CellString(cell(row, createdIdx))
		rowTime, rowTimeOK := parseBackupTime(created)
		if bestID == "" {
			bestID, bestCreated, bestTime, bestTimeOK = rowID, created, rowTime, rowTimeOK
			continue
		}
		// Prefer a real chronological comparison so rows whose timestamps carry
		// different UTC offsets (e.g. across a DST transition) still order by the
		// actual instant. Fall back to a lexicographic compare only when either
		// side failed to parse.
		var older bool
		if rowTimeOK && bestTimeOK {
			older = rowTime.Before(bestTime)
		} else {
			older = created < bestCreated
		}
		if older {
			bestID, bestCreated, bestTime, bestTimeOK = rowID, created, rowTime, rowTimeOK
		}
	}
	return bestID, bestID != ""
}

// parseBackupTime parses a SHOW BACKUPS created_on cell into a time.Time. The
// value reaches us as a string (CellString formats a time.Time as RFC3339, but
// Snowflake may also return the timestamp as a raw string in other layouts), so
// several common layouts are tried. ok is false when the value is empty or none
// of the layouts match, signalling the caller to fall back to a string compare.
func parseBackupTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// createBackupSetNameFQN builds the backup set's own (possibly qualified) name
// reference for CREATE / follow-up APPLY BACKUP POLICY. The db and schema parts
// are always double-quoted; the trailing name goes through QuoteOrBare so a
// case-insensitive name is emitted bare — matching how CREATE stores it
// (uppercased) and how the immediate APPLY must reference it. This differs from
// bsFQN, which always double-quotes the name because there it comes from SHOW
// output (already in stored form).
func createBackupSetNameFQN(name, nameDb, nameSchema string, caseSensitive bool) string {
	nameToken := snowflake.QuoteOrBare(name, caseSensitive)
	switch {
	case nameDb != "" && nameSchema != "":
		return snowflake.QuoteIdent(nameDb) + "." + snowflake.QuoteIdent(nameSchema) + "." + nameToken
	case nameDb != "":
		return snowflake.QuoteIdent(nameDb) + "." + nameToken
	default:
		return nameToken
	}
}

// BuildCreateBackupSetSql builds the CREATE BACKUP SET statement.
func BuildCreateBackupSetSql(name, nameDb, nameSchema, forType, objectFQN string, orReplace, ifNotExists, caseSensitive bool) string {
	nameFQN := createBackupSetNameFQN(name, nameDb, nameSchema, caseSensitive)

	var sb strings.Builder
	sb.WriteString("CREATE ")
	if orReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("BACKUP SET ")
	if ifNotExists && !orReplace {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(nameFQN)
	sb.WriteString(" FOR ")
	sb.WriteString(strings.ToUpper(forType))
	sb.WriteString(" ")
	sb.WriteString(objectFQN)
	return sb.String()
}

// BuildCreateBackupPolicySql builds the CREATE BACKUP POLICY statement.
func BuildCreateBackupPolicySql(name, schedule string, expireAfterDays int64, retentionLock bool, comment, tags string, orReplace, ifNotExists, caseSensitive bool) string {
	var sb strings.Builder
	sb.WriteString("CREATE ")
	if orReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("BACKUP POLICY ")
	if ifNotExists && !orReplace {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(snowflake.QuoteOrBare(name, caseSensitive))
	if tags != "" {
		sb.WriteString(fmt.Sprintf(" WITH TAG (%s)", tags))
	}
	if retentionLock {
		sb.WriteString(" WITH RETENTION LOCK")
	}
	if schedule != "" {
		// Free-text values: EscapeTextLit doubles backslashes as well as quotes so
		// a value ending in "\" (or containing "\'") cannot break out of the literal.
		sb.WriteString(fmt.Sprintf(" SCHEDULE = %s", snowflake.QuoteTextLit(schedule)))
	}
	if expireAfterDays > 0 {
		sb.WriteString(fmt.Sprintf(" EXPIRE_AFTER_DAYS = %d", expireAfterDays))
	}
	if comment != "" {
		sb.WriteString(fmt.Sprintf(" COMMENT = %s", snowflake.QuoteTextLit(comment)))
	}
	return sb.String()
}

// BuildApplyBackupPolicySql builds ALTER BACKUP SET <setFQN> APPLY BACKUP POLICY
// <policy>. policyName comes from SHOW BACKUP POLICIES (its stored form), so it is
// always double-quoted via QuoteIdent to match a case-sensitively created policy
// (a bare emission would be uppercased and miss e.g. a policy named "my_policy").
func BuildApplyBackupPolicySql(setFQN, policyName string) string {
	return fmt.Sprintf("ALTER BACKUP SET %s APPLY BACKUP POLICY %s", setFQN, snowflake.QuoteIdent(policyName))
}

// BuildRestoreFromBackupSql builds:
//
//	CREATE <objectType> <targetName>
//	  FROM BACKUP SET <backupSetFQN>
//	  IDENTIFIER '<backupID>'
//
// targetName is used as-is (the caller provides the identifier, quoted or not).
func BuildRestoreFromBackupSql(objectType, targetName, backupSetName, bsDb, bsSchema, backupID string) (string, error) {
	objType := strings.ToUpper(strings.TrimSpace(objectType))
	if objType == "" {
		return "", fmt.Errorf("object type must be DATABASE, SCHEMA, or TABLE")
	}
	if targetName == "" {
		return "", fmt.Errorf("target name must not be empty")
	}
	if backupSetName == "" {
		return "", fmt.Errorf("backup set name must not be empty")
	}
	fqn := bsFQN(backupSetName, bsDb, bsSchema)

	var sb strings.Builder
	sb.WriteString("CREATE ")
	sb.WriteString(objType)
	sb.WriteString(" ")
	sb.WriteString(targetName)
	sb.WriteString(" FROM BACKUP SET ")
	sb.WriteString(fqn)
	sb.WriteString(" IDENTIFIER ")
	// The backup ID is user-editable in the restore modal — escape it as free
	// text so a stray backslash or quote can't break out of the literal.
	sb.WriteString(snowflake.QuoteTextLit(backupID))
	return sb.String(), nil
}

// BuildDeleteOldestBackupSql builds ALTER BACKUP SET <fqn> DELETE BACKUP
// IDENTIFIER '<id>'.
func BuildDeleteOldestBackupSql(name, bsDb, bsSchema, backupID string) string {
	return fmt.Sprintf(
		"ALTER BACKUP SET %s DELETE BACKUP IDENTIFIER %s",
		bsFQN(name, bsDb, bsSchema), snowflake.QuoteTextLit(backupID),
	)
}

// ListBackupSets returns backup sets whose backed-up object matches the
// right-clicked item, optionally filtered by the backup set's name.
func ListBackupSets(ctx context.Context, client *snowflake.Client, scopeType, db, schema, table, nameFilter string) ([]BackupSetRow, error) {
	res, err := client.Execute(ctx, BuildListBackupSetsSql(nameFilter))
	if err != nil {
		return nil, err
	}
	return ParseBackupSets(res, scopeType, db, schema, table)
}

// CreateBackupSet creates a new backup set for a DATABASE, SCHEMA, or TABLE.
// db is used to set the current database context (required by Snowflake even
// when the object name is fully qualified). When backupPolicy is non-blank the
// policy is applied in a follow-up ALTER using the SAME name reference CREATE
// stored — building the APPLY here (rather than in a separate frontend call that
// re-quotes the name) is what keeps a bare, case-insensitive name matching.
func CreateBackupSet(ctx context.Context, client *snowflake.Client, name, nameDb, nameSchema, forType, objectFQN, db, backupPolicy string, orReplace, ifNotExists, caseSensitive bool) error {
	if db != "" {
		if _, err := client.Execute(ctx, fmt.Sprintf("USE DATABASE %s", snowflake.QuoteIdent(db))); err != nil {
			return err
		}
	}
	if _, err := client.Execute(ctx, BuildCreateBackupSetSql(name, nameDb, nameSchema, forType, objectFQN, orReplace, ifNotExists, caseSensitive)); err != nil {
		return err
	}
	if policy := strings.TrimSpace(backupPolicy); policy != "" {
		setFQN := createBackupSetNameFQN(name, nameDb, nameSchema, caseSensitive)
		if _, err := client.Execute(ctx, BuildApplyBackupPolicySql(setFQN, policy)); err != nil {
			return err
		}
	}
	return nil
}

// DropBackupSet drops the named backup set.
func DropBackupSet(ctx context.Context, client *snowflake.Client, name, bsDb, bsSchema string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("DROP BACKUP SET %s", bsFQN(name, bsDb, bsSchema)))
	return err
}

// AlterBackupSet executes ALTER BACKUP SET <fqn> <alteration>.
func AlterBackupSet(ctx context.Context, client *snowflake.Client, name, bsDb, bsSchema, alteration string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER BACKUP SET %s %s", bsFQN(name, bsDb, bsSchema), alteration))
	return err
}

// ListBackupPolicies runs SHOW BACKUP POLICIES and returns all visible policies.
func ListBackupPolicies(ctx context.Context, client *snowflake.Client) ([]BackupPolicyRow, error) {
	res, err := client.Execute(ctx, "SHOW BACKUP POLICIES")
	if err != nil {
		return nil, err
	}
	return ParseBackupPolicies(res), nil
}

// CreateBackupPolicy creates a new backup policy.
func CreateBackupPolicy(ctx context.Context, client *snowflake.Client, name, schedule string, expireAfterDays int64, retentionLock bool, comment, tags string, orReplace, ifNotExists, caseSensitive bool) error {
	_, err := client.Execute(ctx, BuildCreateBackupPolicySql(name, schedule, expireAfterDays, retentionLock, comment, tags, orReplace, ifNotExists, caseSensitive))
	return err
}

// DropBackupPolicy drops the named backup policy.
func DropBackupPolicy(ctx context.Context, client *snowflake.Client, name string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("DROP BACKUP POLICY %s", snowflake.QuoteIdent(name)))
	return err
}

// AlterBackupPolicy executes ALTER BACKUP POLICY <name> <alteration>.
func AlterBackupPolicy(ctx context.Context, client *snowflake.Client, name, alteration string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER BACKUP POLICY %s %s", snowflake.QuoteIdent(name), alteration))
	return err
}

// ListBackups runs SHOW BACKUPS IN BACKUP SET <fqn> and returns the result.
func ListBackups(ctx context.Context, client *snowflake.Client, backupSetName, bsDb, bsSchema string) ([]BackupRow, error) {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", bsFQN(backupSetName, bsDb, bsSchema)))
	if err != nil {
		return nil, err
	}
	return ParseBackups(res), nil
}

// AddBackup triggers ALTER BACKUP SET <fqn> ADD BACKUP to create a new snapshot.
func AddBackup(ctx context.Context, client *snowflake.Client, backupSetName, bsDb, bsSchema string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER BACKUP SET %s ADD BACKUP", bsFQN(backupSetName, bsDb, bsSchema)))
	return err
}

// RestoreFromBackup creates a new object from a specific backup. db is used to
// set the current database context first.
func RestoreFromBackup(ctx context.Context, client *snowflake.Client, objectType, targetName, backupSetName, bsDb, bsSchema, backupID, db string) error {
	sql, err := BuildRestoreFromBackupSql(objectType, targetName, backupSetName, bsDb, bsSchema, backupID)
	if err != nil {
		return err
	}
	if db != "" {
		if _, err := client.Execute(ctx, fmt.Sprintf("USE DATABASE %s", snowflake.QuoteIdent(db))); err != nil {
			return err
		}
	}
	// Must use QuerySingle (plain db.QueryContext) — multi-statement mode breaks
	// the FROM BACKUP SET ... IDENTIFIER syntax just like TABLE() function calls.
	_, err = client.QuerySingle(ctx, sql)
	return err
}

// DeleteOldestBackup finds the oldest backup in the set with no legal hold and
// deletes it. Snowflake only permits deleting the single oldest eligible
// backup at a time.
func DeleteOldestBackup(ctx context.Context, client *snowflake.Client, backupSetName, bsDb, bsSchema string) error {
	res, err := client.Execute(ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", bsFQN(backupSetName, bsDb, bsSchema)))
	if err != nil {
		return err
	}
	id, ok := FindOldestEligibleBackup(res)
	if !ok {
		return fmt.Errorf("no eligible backup found (all backups may be under legal hold)")
	}
	_, err = client.Execute(ctx, BuildDeleteOldestBackupSql(backupSetName, bsDb, bsSchema, id))
	return err
}
