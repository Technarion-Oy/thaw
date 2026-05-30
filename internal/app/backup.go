// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"
	"strconv"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"time"
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

// ListBackupSets returns backup sets scoped to a database, schema, or table.
// bsFQN builds a (possibly fully-qualified) backup set identifier.
func bsFQN(name, bsDb, bsSchema string) string {
	if bsDb != "" && bsSchema != "" {
		return snowflake.QuoteIdent(bsDb) + "." + snowflake.QuoteIdent(bsSchema) + "." + snowflake.QuoteIdent(name)
	}
	if bsDb != "" {
		return snowflake.QuoteIdent(bsDb) + "." + snowflake.QuoteIdent(name)
	}
	return snowflake.QuoteIdent(name)
}

// ListBackupSets returns backup sets whose backed-up object matches the right-clicked item.
// It uses SHOW BACKUP SETS IN DATABASE <db> and post-filters by object_kind / object_name /
// object_database_name / object_schema_name so that only backup sets actually covering the
// specified database, schema, or table are returned — not all backup sets stored there.
// ListBackupSets returns backup sets whose backed-up object matches the right-clicked item.
// It searches the entire account to find the backups, optionally filtering by the backup set's name.
func (a *App) ListBackupSets(scopeType, db, schema, table, nameFilter string) ([]BackupSetRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}

	// 1. Build the base query
	var queryBuilder strings.Builder
	queryBuilder.WriteString("SHOW BACKUP SETS")

	// 2. Apply the optional name filter using LIKE
	if strings.TrimSpace(nameFilter) != "" {
		// Escape single quotes in the filter to prevent SQL injection/syntax errors
		escapedFilter := strings.ReplaceAll(nameFilter, "'", "''")
		// Wrap in % wildcards for a 'contains' search, e.g., LIKE '%my_backup%'
		queryBuilder.WriteString(fmt.Sprintf(" LIKE '%%%s%%'", escapedFilter))
	}

	// 3. Append the scope (from your previous fix)
	queryBuilder.WriteString(" IN ACCOUNT")

	res, err := a.client.Execute(a.ctx, queryBuilder.String())
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	nameIdx := colIdx(res.Columns, "name")
	bsDbIdx := colIdx(res.Columns, "database_name")
	bsSchIdx := colIdx(res.Columns, "schema_name")
	createdIdx := colIdx(res.Columns, "created_on")
	otypeIdx := colIdx(res.Columns, "object_kind")
	onameIdx := colIdx(res.Columns, "object_name")
	objDbIdx := colIdx(res.Columns, "object_database_name")
	objSchIdx := colIdx(res.Columns, "object_schema_name")
	statusIdx := colIdx(res.Columns, "backup_policy_state", "status")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	upperScope := strings.ToUpper(scopeType)
	rows := make([]BackupSetRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		otype := strings.ToUpper(toString(get(row, otypeIdx)))
		oname := toString(get(row, onameIdx))
		objDb := toString(get(row, objDbIdx))
		objSch := toString(get(row, objSchIdx))

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

		rowBsDb := toString(get(row, bsDbIdx))
		rowBsSch := toString(get(row, bsSchIdx))
		if rowBsDb == "" {
			rowBsDb = db
		}
		rows = append(rows, BackupSetRow{
			Name:            toString(get(row, nameIdx)),
			BackupSetDb:     rowBsDb,
			BackupSetSchema: rowBsSch,
			CreatedOn:       toString(get(row, createdIdx)),
			ObjectType:      otype,
			ObjectName:      oname,
			ObjectDb:        objDb,
			ObjectSchema:    objSch,
			Status:          toString(get(row, statusIdx)),
			Comment:         toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// CreateBackupSet creates a new backup set for a DATABASE, SCHEMA, or TABLE.
// forType must be "DATABASE", "SCHEMA", or "TABLE".
// nameDb and nameSchema locate the backup set object itself (its fully-qualified name).
// db is the database name used to set the session context before the CREATE.
// objectFQN is the fully-qualified target object, e.g. "MY_DB" or "MY_DB"."MY_SCHEMA"."MY_TABLE".
// CreateBackupSet creates a new backup set for a DATABASE, SCHEMA, or TABLE.
// caseSensitive controls whether the backup set name is double-quoted (preserving
// exact case) or left unquoted when it is a valid bare identifier.
func (a *App) CreateBackupSet(name, nameDb, nameSchema, forType, objectFQN, db string, orReplace, ifNotExists, caseSensitive bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	// Snowflake requires a current database to be set for CREATE BACKUP SET,
	// even when the object name is fully qualified.
	if db != "" {
		if _, err := a.client.Execute(a.ctx, fmt.Sprintf("USE DATABASE %s", snowflake.QuoteIdent(db))); err != nil {
			return err
		}
	}

	// Build the (optionally fully-qualified) backup set name.
	nameToken := snowflake.QuoteOrBare(name, caseSensitive)
	var nameFQN string
	switch {
	case nameDb != "" && nameSchema != "":
		nameFQN = snowflake.QuoteIdent(nameDb) + "." + snowflake.QuoteIdent(nameSchema) + "." + nameToken
	case nameDb != "":
		nameFQN = snowflake.QuoteIdent(nameDb) + "." + nameToken
	default:
		nameFQN = nameToken
	}

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

	_, err := a.client.Execute(a.ctx, sb.String())
	return err
}

// DropBackupSet drops the named backup set.
func (a *App) DropBackupSet(name, bsDb, bsSchema string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	fqn := bsFQN(name, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("DROP BACKUP SET %s", fqn))
	return err
}

// AlterBackupSet executes ALTER BACKUP SET <fqn> <alteration>.
// alteration is the full action fragment, e.g. "RENAME TO new_name",
// "SET COMMENT = 'text'", "UNSET COMMENT", "SUSPEND BACKUP POLICY", etc.
func (a *App) AlterBackupSet(name, bsDb, bsSchema, alteration string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	fqn := bsFQN(name, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP SET %s %s", fqn, alteration))
	return err
}

// ListBackupPolicies runs SHOW BACKUP POLICIES and returns all visible policies.
func (a *App) ListBackupPolicies() ([]BackupPolicyRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	res, err := a.client.Execute(a.ctx, "SHOW BACKUP POLICIES")
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	toBool := func(v interface{}) bool {
		s := strings.ToUpper(toString(v))
		return s == "TRUE" || s == "YES" || s == "1"
	}

	nameIdx := colIdx(res.Columns, "name")
	createdIdx := colIdx(res.Columns, "created_on")
	ownerIdx := colIdx(res.Columns, "owner")
	schedIdx := colIdx(res.Columns, "schedule")
	expireIdx := colIdx(res.Columns, "expire_after_days")
	lockIdx := colIdx(res.Columns, "retention_lock", "with_retention_lock")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	toInt64 := func(v interface{}) int64 {
		s := toString(v)
		if s == "" {
			return 0
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		return 0
	}

	rows := make([]BackupPolicyRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, BackupPolicyRow{
			Name:            toString(get(row, nameIdx)),
			CreatedOn:       toString(get(row, createdIdx)),
			Owner:           toString(get(row, ownerIdx)),
			Schedule:        toString(get(row, schedIdx)),
			ExpireAfterDays: toInt64(get(row, expireIdx)),
			RetentionLock:   toBool(get(row, lockIdx)),
			Comment:         toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// CreateBackupPolicy creates a new backup policy.
// schedule: optional, e.g. "60 MINUTE", "6 HOUR", "USING CRON 0 2 * * * UTC"
// expireAfterDays: 0 means not set
// tags: optional raw tag expression e.g. `"MY_TAG" = 'value'`
// caseSensitive: when true the policy name is double-quoted (preserving exact
// case); when false the name is left unquoted if it is a valid bare identifier.
func (a *App) CreateBackupPolicy(name, schedule string, expireAfterDays int64, retentionLock bool, comment, tags string, orReplace, ifNotExists, caseSensitive bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
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
		sb.WriteString(fmt.Sprintf(" SCHEDULE = '%s'", snowflake.EscapeStringLit(schedule)))
	}
	if expireAfterDays > 0 {
		sb.WriteString(fmt.Sprintf(" EXPIRE_AFTER_DAYS = %d", expireAfterDays))
	}
	if comment != "" {
		sb.WriteString(fmt.Sprintf(" COMMENT = '%s'", snowflake.EscapeStringLit(comment)))
	}

	_, err := a.client.Execute(a.ctx, sb.String())
	return err
}

// DropBackupPolicy drops the named backup policy.
func (a *App) DropBackupPolicy(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("DROP BACKUP POLICY %s", snowflake.QuoteIdent(name)))
	return err
}

// AlterBackupPolicy executes ALTER BACKUP POLICY <name> <alteration>.
// alteration is the full action fragment, e.g. "RENAME TO new_name",
// "SET SCHEDULE = '60 MINUTE'", "SET COMMENT = 'text'", "UNSET COMMENT", etc.
func (a *App) AlterBackupPolicy(name, alteration string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP POLICY %s %s", snowflake.QuoteIdent(name), alteration))
	return err
}

// ListBackups runs SHOW BACKUPS IN BACKUP SET <name> and returns the result.
// db must be non-empty; it is used to set a current database context first.
func (a *App) ListBackups(backupSetName, bsDb, bsSchema string) ([]BackupRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := bsFQN(backupSetName, bsDb, bsSchema)
	res, err := a.client.Execute(a.ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", fqn))
	if err != nil {
		return nil, err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	toInt64 := func(v interface{}) int64 {
		s := toString(v)
		if s == "" {
			return 0
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	}

	// Snowflake internally uses "snapshot" terminology; column names vary by version.
	idIdx := colIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	nameIdx := colIdx(res.Columns, "name", "backup_name", "snapshot_name", "backup", "snapshot")
	createdIdx := colIdx(res.Columns, "created_on")
	statusIdx := colIdx(res.Columns, "status")
	sizeIdx := colIdx(res.Columns, "size_bytes", "size")
	commentIdx := colIdx(res.Columns, "comment")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	rows := make([]BackupRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		idVal := toString(get(row, idIdx))
		nameVal := toString(get(row, nameIdx))
		// If no dedicated name column was found, fall back to created_on — Snowflake
		// uses the creation timestamp as the backup identifier in DROP BACKUP.
		if nameVal == "" {
			nameVal = toString(get(row, createdIdx))
		}
		rows = append(rows, BackupRow{
			ID:        idVal,
			Name:      nameVal,
			CreatedOn: toString(get(row, createdIdx)),
			Status:    toString(get(row, statusIdx)),
			SizeBytes: toInt64(get(row, sizeIdx)),
			Comment:   toString(get(row, commentIdx)),
		})
	}
	return rows, nil
}

// AddBackup triggers ALTER BACKUP SET <fqn> ADD BACKUP to create a new backup snapshot.
func (a *App) AddBackup(backupSetName, bsDb, bsSchema string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	fqn := bsFQN(backupSetName, bsDb, bsSchema)
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER BACKUP SET %s ADD BACKUP", fqn))
	return err
}

// RestoreFromBackup executes RESTORE [OR REPLACE] <objectType> <targetName> FROM BACKUP <backupName>.
// db must be non-empty; it is used to set a current database context first.
// targetName is the fully-qualified target object name (may differ from the original to restore into a new object).
// RestoreFromBackup executes:
//
//	CREATE <objectType> <targetName>
//	  FROM BACKUP SET <backupSetName>
//	  IDENTIFIER '<backupID>'
//
// RestoreFromBackup executes RESTORE [OR REPLACE] <objectType> <targetName> FROM BACKUP <backupName>.
// db must be non-empty; it is used to set a current database context first.
// targetName is used as-is (caller provides the identifier, quoted or unquoted).
// backupID is the UUID returned by SHOW BACKUPS (stored as a single-quoted string literal).
func (a *App) RestoreFromBackup(objectType, targetName, backupSetName, bsDb, bsSchema, backupID, db string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	objType := strings.ToUpper(strings.TrimSpace(objectType))
	if objType == "" {
		return fmt.Errorf("object type must be DATABASE, SCHEMA, or TABLE")
	}
	if targetName == "" {
		return fmt.Errorf("target name must not be empty")
	}
	if backupSetName == "" {
		return fmt.Errorf("backup set name must not be empty")
	}
	if db != "" {
		if _, err := a.client.Execute(a.ctx, fmt.Sprintf("USE DATABASE %s", snowflake.QuoteIdent(db))); err != nil {
			return err
		}
	}

	// Safely construct the fully qualified name (e.g. "DB"."SCHEMA"."BACKUP_SET")
	fqn := bsFQN(backupSetName, bsDb, bsSchema)

	var sb strings.Builder
	sb.WriteString("CREATE ")
	sb.WriteString(objType)
	sb.WriteString(" ")
	sb.WriteString(targetName)
	sb.WriteString(" FROM BACKUP SET ")
	sb.WriteString(fqn) // Inject the fully qualified name here
	sb.WriteString(" IDENTIFIER '")
	sb.WriteString(strings.ReplaceAll(backupID, "'", "''"))
	sb.WriteString("'")

	// Must use QuerySingle (plain db.QueryContext) — multi-statement mode breaks
	// the FROM BACKUP SET ... IDENTIFIER syntax just like TABLE() function calls.
	_, err := a.client.QuerySingle(a.ctx, sb.String())
	return err
}

// DeleteOldestBackup finds the oldest backup in the set that has no legal hold
// and deletes it using ALTER BACKUP SET … DELETE BACKUP IDENTIFIER '<id>'.
// Snowflake only permits deleting the single oldest eligible backup at a time.
func (a *App) DeleteOldestBackup(backupSetName, bsDb, bsSchema string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	fqn := bsFQN(backupSetName, bsDb, bsSchema)

	res, err := a.client.Execute(a.ctx, fmt.Sprintf("SHOW BACKUPS IN BACKUP SET %s", fqn))
	if err != nil {
		return err
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case []byte:
			return string(t)
		case string:
			return t
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprintf("%v", t)
		}
	}

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	idIdx := colIdx(res.Columns, "backup_id", "snapshot_id", "id", "identifier", "uuid")
	createdIdx := colIdx(res.Columns, "created_on")
	legalHoldIdx := colIdx(res.Columns, "is_under_legal_hold", "legal_hold", "under_legal_hold")

	type candidate struct {
		id        string
		createdOn string
	}
	var best *candidate

	for _, row := range res.Rows {
		lh := strings.ToUpper(strings.TrimSpace(toString(get(row, legalHoldIdx))))
		if lh == "Y" || lh == "TRUE" || lh == "YES" || lh == "1" {
			continue
		}
		id := toString(get(row, idIdx))
		if id == "" {
			continue
		}
		created := toString(get(row, createdIdx))
		if best == nil || created < best.createdOn {
			best = &candidate{id: id, createdOn: created}
		}
	}

	if best == nil {
		return fmt.Errorf("no eligible backup found (all backups may be under legal hold)")
	}

	escapedID := strings.ReplaceAll(best.id, "'", "''")
	_, err = a.client.Execute(a.ctx, fmt.Sprintf(
		"ALTER BACKUP SET %s DELETE BACKUP IDENTIFIER '%s'",
		fqn, escapedID,
	))
	return err
}
