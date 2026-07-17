package sqlgrammar

import "thaw/internal/sqltok"

// CREATE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseCreateObj validates the Snowflake `CREATE <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateObj() bool {
	// Lenient skeleton: CREATE [ OR REPLACE ] [ modifiers ] <OBJECT_KIND> [ IF NOT
	// EXISTS ] <name> followed by a permissive run of the remaining tokens
	// (balanced parens, scalars, operators) to EOF.
	consumeToken := func() bool {
		if v.AtEnd() {
			return false
		}
		v.advance()
		return true
	}
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen {
						return false
					}
					return consumeToken()
				},
			)
		})
	}
	// match one identifier-like token (the object-kind word)
	kindWord := func() bool {
		t := v.Peek()
		if t.Kind.IsIdentLike() {
			v.advance()
			return true
		}
		v.expect("object kind")
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		// at least one object-kind word, plus any further kind words before the
		// name (e.g. APPLICATION ROLE). We stop greedily consuming via the name.
		kindWord,
		v.ifNotExists,
		v.parseIdentPath,
		consumeBalanced,
	)
}

// ParseCreateAccount validates the Snowflake `CREATE ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-account
//
// Syntax:
//
//	CREATE ACCOUNT <name>
//	      ADMIN_NAME = '<string_literal>'
//	    { ADMIN_PASSWORD = '<string_literal>' | ADMIN_RSA_PUBLIC_KEY = '<string_literal>' }
//	    [ ADMIN_USER_TYPE = { PERSON | SERVICE | LEGACY_SERVICE | NULL } ]
//	    [ FIRST_NAME = '<string_literal>' ]
//	    [ LAST_NAME = '<string_literal>' ]
//	      EMAIL = '<string_literal>'
//	    [ MUST_CHANGE_PASSWORD = { TRUE | FALSE } ]
//	      EDITION = { STANDARD | ENTERPRISE | BUSINESS_CRITICAL }
//	    [ REGION_GROUP = <region_group_id> ]
//	    [ REGION = <snowflake_region_id> ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ POLARIS = { TRUE | FALSE } ]
func (v *Validator) ParseCreateAccount() bool {
	str := v.parseString
	prop := func() bool {
		return v.Choice(
			v.option("ADMIN_NAME", str),
			v.option("ADMIN_PASSWORD", str),
			v.option("ADMIN_RSA_PUBLIC_KEY", str),
			v.option("ADMIN_USER_TYPE", v.wordsValue("PERSON", "SERVICE", "LEGACY_SERVICE", "NULL")),
			v.option("FIRST_NAME", str),
			v.option("LAST_NAME", str),
			v.option("EMAIL", str),
			v.option("MUST_CHANGE_PASSWORD", v.parseBool),
			v.option("EDITION", v.wordsValue("STANDARD", "ENTERPRISE", "BUSINESS_CRITICAL")),
			v.option("REGION_GROUP", v.parseScalar),
			v.option("REGION", v.parseScalar),
			v.commentOption(),
			v.option("POLARIS", v.parseBool),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ACCOUNT") },
		v.parseIdentPath,
		// At minimum a single property must be present; the rest are order-independent.
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateAgent validates the Snowflake `CREATE AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-agent
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AGENT [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<comment>' ]
//	  [ PROFILE = '<profile_object>' ]
//	  FROM SPECIFICATION
//	  $$
//	  <specification_object>
//	  $$;
func (v *Validator) ParseCreateAgent() bool {
	prop := func() bool {
		return v.Choice(
			v.commentOption(),
			v.option("PROFILE", v.parseString),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("AGENT") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.MatchWord("SPECIFICATION") },
		// $$ <specification> $$  — a single DollarQuoted token, or a quoted string.
		func() bool {
			return v.Choice(
				func() bool { return v.Match(sqltok.DollarQuoted) },
				v.parseString,
			)
		},
	)
}

// ParseCreateAggregationPolicy validates the Snowflake `CREATE AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-aggregation-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AGGREGATION POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS AGGREGATION_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateAggregationPolicy() bool {
	// Free-form policy body after `->`: consume a permissive, paren-balanced run
	// until a trailing COMMENT option or EOF.
	var balanced func() bool
	balanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						balanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					t := v.Peek()
					if t.Kind == sqltok.EOF || t.Kind == sqltok.RParen {
						return false
					}
					// stop at a top-level COMMENT = option
					if t.Kind.IsIdentLike() {
						p := v.save()
						if v.commentOption()() {
							v.restore(p)
							return false
						}
						v.restore(p)
					}
					v.advance()
					return true
				},
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("AGGREGATION") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.Match(sqltok.RParen) },
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("AGGREGATION_CONSTRAINT") },
		// -> (two operator tokens)
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		// body must consume at least one token
		func() bool {
			before := v.save()
			balanced()
			return v.pos > before
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateAlert validates the Snowflake `CREATE ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-alert
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ALERT [ IF NOT EXISTS ] <name>
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	  [ WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ CONFIG = '<configuration_string>' ]
//	  [ RUNBOOK = '<string_literal>' ]
//	  [ SUSPEND_ALERT_AFTER_NUM_FAILURES = <number> ]
//	  IF( EXISTS(
//	    <condition>
//	  ))
//	  THEN
//	    <action>
func (v *Validator) ParseCreateAlert() bool {
	// balanced parenthesized group: ( ... ) with nested parens.
	var parenGroup func() bool
	parenGroup = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						parenGroup,
						func() bool {
							if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	consumeRest := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			v.advance()
			return true
		})
	}
	prop := func() bool {
		return v.Choice(
			v.tagClause,
			v.option("SCHEDULE", v.parseString),
			v.option("WAREHOUSE", v.parseIdentPath),
			v.commentOption(),
			v.option("CONFIG", v.parseString),
			v.option("RUNBOOK", v.parseString),
			v.option("SUSPEND_ALERT_AFTER_NUM_FAILURES", v.parseNumber),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ALERT") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
		// IF ( EXISTS ( <condition> ) )
		func() bool { return v.MatchKeyword("IF") },
		parenGroup,
		func() bool { return v.MatchWord("THEN") },
		// <action> — free-form, must have at least one token.
		func() bool {
			before := v.save()
			consumeRest()
			return v.pos > before
		},
	)
}

// ParseCreateApiIntegration validates the Snowflake `CREATE API INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-api-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = { aws_api_gateway | aws_private_api_gateway | aws_gov_api_gateway | aws_gov_private_api_gateway }
//	  API_AWS_ROLE_ARN = '<iam_role>'
//	  [ API_KEY = '<api_key>' ]
//	  API_ALLOWED_PREFIXES = ('<...>')
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = azure_api_management
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  AZURE_AD_APPLICATION_ID = '<azure_application_id>'
//	  [ API_KEY = '<api_key>' ]
//	  API_ALLOWED_PREFIXES = ( '<...>' )
//	  [ API_BLOCKED_PREFIXES = ( '<...>' ) ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = google_api_gateway
//	  GOOGLE_AUDIENCE = '<google_audience_claim>'
//	  API_ALLOWED_PREFIXES = ( '<...>' )
//	  [ API_BLOCKED_PREFIXES = ( '<...>' ) ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
//
//	CREATE [ OR REPLACE ] API INTEGRATION [ IF NOT EXISTS ] <integration_name>
//	  API_PROVIDER = git_https_api
//	  API_ALLOWED_PREFIXES = ('<...>')
//	  [ API_BLOCKED_PREFIXES = ('<...>') ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = ( { <secret_name> [, <secret_name>, ... ] } ) | all | none ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//	  ;
func (v *Validator) ParseCreateApiIntegration() bool {
	str := v.parseString
	strList := func() bool { return v.parseParenList(str) }
	prop := func() bool {
		return v.Choice(
			v.option("API_PROVIDER", v.parseScalar),
			v.option("API_AWS_ROLE_ARN", str),
			v.option("API_KEY", str),
			v.option("API_ALLOWED_PREFIXES", strList),
			v.option("API_BLOCKED_PREFIXES", strList),
			v.option("AZURE_TENANT_ID", str),
			v.option("AZURE_AD_APPLICATION_ID", str),
			v.option("GOOGLE_AUDIENCE", str),
			// ALLOWED_AUTHENTICATION_SECRETS = ( <secret> [, ...] ) | ALL | NONE
			v.option("ALLOWED_AUTHENTICATION_SECRETS", func() bool {
				return v.Choice(
					func() bool { return v.parseParenList(v.parseIdentPath) },
					v.wordsValue("ALL", "NONE"),
				)
			}),
			v.option("ENABLED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("API") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateApplication validates the Snowflake `CREATE APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application
//
// Syntax:
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	   [ USING RELEASE CHANNEL { QA | ALPHA | DEFAULT } ]
//	   [ COMMENT = '<string_literal>' ]
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	   [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	   [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	  USING <path_to_version_directory>
//	  [ DEBUG_MODE = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [, ...] ) ]
//	  [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	  [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM APPLICATION PACKAGE <package_name>
//	  USING VERSION  <version_identifier> [ PATCH <patch_num> ]
//	  [ DEBUG_MODE = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	  [ WITH FEATURE POLICY = <policy_name> ]
//
//	CREATE APPLICATION <name> FROM LISTING <listing_name>
//	   [ USING RELEASE CHANNEL { QA | ALPHA | DEFAULT } ]
//	   [ COMMENT = '<string_literal>' ]
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	   [ BACKGROUND_INSTALL = { TRUE | FALSE } ]
//	   [ AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE } ]
//	   [ WITH FEATURE POLICY = <policy_name> ]
func (v *Validator) ParseCreateApplication() bool {
	// USING clause variants.
	usingClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("USING") },
			func() bool {
				return v.Choice(
					// RELEASE CHANNEL { QA | ALPHA | DEFAULT }
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("RELEASE") },
							func() bool { return v.MatchWord("CHANNEL") },
							v.wordsValue("QA", "ALPHA", "DEFAULT"),
						)
					},
					// VERSION <version> [ PATCH <num> ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("VERSION") },
							v.parseIdentPath,
							func() bool {
								return v.Optional(func() bool {
									return v.Sequence(
										func() bool { return v.MatchWord("PATCH") },
										v.parseScalar,
									)
								})
							},
						)
					},
					// <path_to_version_directory> — string or stage-style path
					func() bool { return v.Match(sqltok.StringLit) },
					v.parseIdentPath,
				)
			},
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("DEBUG_MODE", v.parseBool),
			v.commentOption(),
			v.tagClause,
			v.option("AUTHORIZE_TELEMETRY_EVENT_SHARING", v.parseBool),
			v.option("BACKGROUND_INSTALL", v.parseBool),
			// WITH FEATURE POLICY = <name>
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("FEATURE") },
					func() bool { return v.MatchWord("POLICY") },
					func() bool { return v.MatchOp("=") },
					v.parseIdentPath,
				)
			},
		)
	}
	fromClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("FROM") },
			func() bool {
				return v.Choice(
					// APPLICATION PACKAGE <pkg> [ USING ... ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("APPLICATION") },
							func() bool { return v.MatchWord("PACKAGE") },
							v.parseIdentPath,
							func() bool { return v.Optional(usingClause) },
						)
					},
					// LISTING <listing> [ USING RELEASE CHANNEL ... ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("LISTING") },
							v.parseIdentPath,
							func() bool { return v.Optional(usingClause) },
						)
					},
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
		fromClause,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateApplicationPackage validates the Snowflake `CREATE APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-package
//
// Syntax:
//
//	CREATE APPLICATION PACKAGE [ IF NOT EXISTS ] <name>
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ DISTRIBUTION = { INTERNAL | EXTERNAL } ]
//	  [ LISTING_AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ MULTIPLE_INSTANCES = TRUE ]
//	  [ ENABLE_RELEASE_CHANNELS = TRUE ]
func (v *Validator) ParseCreateApplicationPackage() bool {
	intLit := v.parseNumber
	prop := func() bool {
		return v.Choice(
			v.option("DATA_RETENTION_TIME_IN_DAYS", intLit),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", intLit),
			v.option("DEFAULT_DDL_COLLATION", v.parseString),
			v.commentOption(),
			v.tagClause,
			v.option("DISTRIBUTION", v.wordsValue("INTERNAL", "EXTERNAL")),
			v.option("LISTING_AUTO_REFRESH", v.parseBool),
			v.option("MULTIPLE_INSTANCES", v.parseBool),
			v.option("ENABLE_RELEASE_CHANNELS", v.parseBool),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.MatchWord("PACKAGE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateApplicationRole validates the Snowflake `CREATE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] APPLICATION ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER APPLICATION ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateApplicationRole() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.MatchWord("ROLE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateAuthenticationPolicy validates the Snowflake `CREATE AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-authentication-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] AUTHENTICATION POLICY [ IF NOT EXISTS ] <name>
//	  [ AUTHENTICATION_METHODS = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_TYPES = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_POLICY = ( <client_type> = ( MINIMUM_VERSION = '<version>' ) [ , ... ] ) ]
//	  [ SECURITY_INTEGRATIONS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	  [ MFA_ENROLLMENT = { 'REQUIRED' | 'REQUIRED_PASSWORD_ONLY' | 'OPTIONAL' } ]
//	  [ MFA_POLICY= ( <list_of_properties> ) ]
//	  [ PAT_POLICY = ( <list_of_properties> ) ]
//	  [ WORKLOAD_IDENTITY_POLICY = ( <list_of_properties> ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER AUTHENTICATION POLICY <name>
//	  [ ... same properties as above ... ]
func (v *Validator) ParseCreateAuthenticationPolicy() bool {
	str := v.parseString
	strList := func() bool { return v.parseParenList(str) }
	// A balanced parenthesized value (for the structured property lists).
	var parenValue func() bool
	parenValue = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						parenValue,
						func() bool {
							if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("AUTHENTICATION_METHODS", strList),
			v.option("CLIENT_TYPES", strList),
			v.option("SECURITY_INTEGRATIONS", strList),
			v.option("MFA_ENROLLMENT", str),
			v.option("CLIENT_POLICY", parenValue),
			v.option("MFA_POLICY", parenValue),
			v.option("PAT_POLICY", parenValue),
			v.option("WORKLOAD_IDENTITY_POLICY", parenValue),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("AUTHENTICATION") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateBackupPolicy validates the Snowflake `CREATE BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] BACKUP POLICY [ IF NOT EXISTS ] <name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH RETENTION LOCK ]
//	   [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	   [ EXPIRE_AFTER_DAYS = <days_integer> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateBackupPolicy() bool {
	prop := func() bool {
		return v.Choice(
			v.tagClause,
			// WITH RETENTION LOCK
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("RETENTION") },
					func() bool { return v.MatchWord("LOCK") },
				)
			},
			v.option("SCHEDULE", v.parseString),
			v.option("EXPIRE_AFTER_DAYS", v.parseNumber),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("BACKUP") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateBackupSet validates the Snowflake `CREATE BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-backup-set
//
// Syntax:
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	   FOR [ DYNAMIC ] TABLE <table_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	  FOR SCHEMA <schema_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] BACKUP SET [ IF NOT EXISTS ] <name>
//	  FOR DATABASE <database_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH BACKUP POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateBackupSet() bool {
	forClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("FOR") },
			func() bool {
				return v.Choice(
					// [ DYNAMIC ] TABLE <name>
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchWord("DYNAMIC") }) },
							func() bool { return v.MatchKeyword("TABLE") },
							v.parseIdentPath,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("SCHEMA") },
							v.parseIdentPath,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("DATABASE") },
							v.parseIdentPath,
						)
					},
				)
			},
		)
	}
	prop := func() bool {
		return v.Choice(
			v.tagClause,
			// WITH BACKUP POLICY <policy_name>
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("BACKUP") },
					func() bool { return v.MatchWord("POLICY") },
					v.parseIdentPath,
				)
			},
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("BACKUP") },
		func() bool { return v.MatchKeyword("SET") },
		v.ifNotExists,
		v.parseIdentPath,
		forClause,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateCatalogIntegration validates the Snowflake `CREATE CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateCatalogIntegration() bool {
	// Lenient: CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
	// followed by a permissive, paren-balanced run of options to EOF.
	var balanced func() bool
	balanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						balanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		// at least one option token must follow the name
		func() bool {
			before := v.save()
			balanced()
			return v.pos > before
		},
	)
}

// ParseCreateCatalogIntegrationAwsGlue validates the Snowflake `CREATE CATALOG INTEGRATION (AWS Glue)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-glue
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  CATALOG_SOURCE = GLUE
//	  TABLE_FORMAT = ICEBERG
//	  GLUE_AWS_ROLE_ARN = '<arn-for-AWS-role-to-assume>'
//	  GLUE_CATALOG_ID = '<glue-catalog-id>'
//	  [ GLUE_REGION = '<AWS-region-of-the-glue-catalog>' ]
//	  [ CATALOG_NAMESPACE = '<catalog-namespace>' ]
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationAwsGlue() bool {
	str := v.parseString
	prop := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.wordsValue("GLUE")),
			v.option("TABLE_FORMAT", v.wordsValue("ICEBERG")),
			v.option("GLUE_AWS_ROLE_ARN", str),
			v.option("GLUE_CATALOG_ID", str),
			v.option("GLUE_REGION", str),
			v.option("CATALOG_NAMESPACE", str),
			v.option("ENABLED", v.parseBool),
			v.option("REFRESH_INTERVAL_SECONDS", v.parseNumber),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateCatalogIntegrationObjectStorage validates the Snowflake `CREATE CATALOG INTEGRATION (Object storage)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-object-storage
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  CATALOG_SOURCE = OBJECT_STORE
//	  TABLE_FORMAT = { ICEBERG | DELTA }
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationObjectStorage() bool {
	prop := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.wordsValue("OBJECT_STORE")),
			v.option("TABLE_FORMAT", v.wordsValue("ICEBERG", "DELTA")),
			v.option("ENABLED", v.parseBool),
			v.option("REFRESH_INTERVAL_SECONDS", v.parseNumber),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateCatalogIntegrationSnowflakeOpenCatalog validates the Snowflake `CREATE CATALOG INTEGRATION (Snowflake Open Catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-open-catalog
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ]
//	  <name>
//	  CATALOG_SOURCE = POLARIS
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<open_catalog_namespace>' ]
//	  REST_CONFIG = (
//	    CATALOG_URI = '<open_catalog_account_url>'
//	    [ CATALOG_API_TYPE = PUBLIC ]
//	    CATALOG_NAME = '<open_catalog_catalog_name>'
//	    [ ACCESS_DELEGATION_MODE = { VENDED_CREDENTIALS | EXTERNAL_VOLUME_CREDENTIALS } ]
//	  )
//	  REST_AUTHENTICATION = (
//	    TYPE = OAUTH
//	    [ OAUTH_TOKEN_URI = 'https://<token_server_uri>' ]
//	    OAUTH_CLIENT_ID = '<oauth_client_id>'
//	    OAUTH_CLIENT_SECRET = '<oauth_secret>'
//	    OAUTH_ALLOWED_SCOPES = ('<scope 1>', '<scope 2>')
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateCatalogIntegrationSnowflakeOpenCatalog() bool {
	str := v.parseString
	strList := func() bool { return v.parseParenList(str) }
	// REST_CONFIG = ( <sub-options> )
	restConfig := func() bool {
		sub := func() bool {
			return v.Choice(
				v.option("CATALOG_URI", str),
				v.option("CATALOG_API_TYPE", v.wordsValue("PUBLIC")),
				v.option("CATALOG_NAME", str),
				v.option("ACCESS_DELEGATION_MODE", v.wordsValue("VENDED_CREDENTIALS", "EXTERNAL_VOLUME_CREDENTIALS")),
			)
		}
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			sub,
			func() bool { return v.ZeroOrMore(sub) },
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// REST_AUTHENTICATION = ( <sub-options> )
	restAuth := func() bool {
		sub := func() bool {
			return v.Choice(
				v.option("TYPE", v.wordsValue("OAUTH")),
				v.option("OAUTH_TOKEN_URI", str),
				v.option("OAUTH_CLIENT_ID", str),
				v.option("OAUTH_CLIENT_SECRET", str),
				v.option("OAUTH_ALLOWED_SCOPES", strList),
			)
		}
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			sub,
			func() bool { return v.ZeroOrMore(sub) },
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.wordsValue("POLARIS")),
			v.option("TABLE_FORMAT", v.wordsValue("ICEBERG")),
			v.option("CATALOG_NAMESPACE", str),
			v.option("REST_CONFIG", restConfig),
			v.option("REST_AUTHENTICATION", restAuth),
			v.option("ENABLED", v.parseBool),
			v.option("REFRESH_INTERVAL_SECONDS", v.parseNumber),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateCatalogIntegrationApacheIcebergRest validates the Snowflake `CREATE CATALOG INTEGRATION (Apache Iceberg REST)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-rest
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = ICEBERG_REST
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<namespace>' ]
//	  REST_CONFIG = (
//	    restConfigParams
//	  )
//	  REST_AUTHENTICATION = (
//	    restAuthenticationParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	restConfigParams ::=
//	  CATALOG_URI = '<rest_api_endpoint_url>'
//	  [ PREFIX = '<prefix>' ]
//	  [ CATALOG_NAME = '<catalog_name>' ]
//	  [ CATALOG_API_TYPE = { PUBLIC | PRIVATE | AWS_API_GATEWAY | AWS_PRIVATE_API_GATEWAY | AWS_GLUE | AWS_PRIVATE_GLUE} ]
//	  [ ACCESS_DELEGATION_MODE = { VENDED_CREDENTIALS | EXTERNAL_VOLUME_CREDENTIALS } ]
//
//	restAuthenticationParams (for OAuth) ::=
//	  TYPE = OAUTH
//	  [ OAUTH_TOKEN_URI = 'https://<token_server_uri>' ]
//	  OAUTH_CLIENT_ID = '<oauth_client_id>'
//	  OAUTH_CLIENT_SECRET = '<oauth_client_secret>'
//	  OAUTH_ALLOWED_SCOPES = ('<scope_1>', '<scope_2>')
//
//	restAuthenticationParams (for Bearer token) ::=
//	  TYPE = BEARER
//	  BEARER_TOKEN = '<bearer_token>'
//
//	restAuthenticationParams (for SigV4) ::=
//	  TYPE = SIGV4
//	  SIGV4_IAM_ROLE = '<iam_role_arn>'
//	  [ SIGV4_SIGNING_REGION = '<region>' ]
//	  [ SIGV4_EXTERNAL_ID = '<external_id>' ]
func (v *Validator) ParseCreateCatalogIntegrationApacheIcebergRest() bool {
	str := v.parseString
	strList := func() bool { return v.parseParenList(str) }
	restConfig := func() bool {
		sub := func() bool {
			return v.Choice(
				v.option("CATALOG_URI", str),
				v.option("PREFIX", str),
				v.option("CATALOG_NAME", str),
				v.option("CATALOG_API_TYPE", v.wordsValue("PUBLIC", "PRIVATE",
					"AWS_API_GATEWAY", "AWS_PRIVATE_API_GATEWAY", "AWS_GLUE", "AWS_PRIVATE_GLUE")),
				v.option("ACCESS_DELEGATION_MODE", v.wordsValue("VENDED_CREDENTIALS", "EXTERNAL_VOLUME_CREDENTIALS")),
			)
		}
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			sub,
			func() bool { return v.ZeroOrMore(sub) },
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	restAuth := func() bool {
		sub := func() bool {
			return v.Choice(
				v.option("TYPE", v.wordsValue("OAUTH", "BEARER", "SIGV4")),
				v.option("OAUTH_TOKEN_URI", str),
				v.option("OAUTH_CLIENT_ID", str),
				v.option("OAUTH_CLIENT_SECRET", str),
				v.option("OAUTH_ALLOWED_SCOPES", strList),
				v.option("BEARER_TOKEN", str),
				v.option("SIGV4_IAM_ROLE", str),
				v.option("SIGV4_SIGNING_REGION", str),
				v.option("SIGV4_EXTERNAL_ID", str),
			)
		}
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			sub,
			func() bool { return v.ZeroOrMore(sub) },
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.wordsValue("ICEBERG_REST")),
			v.option("TABLE_FORMAT", v.wordsValue("ICEBERG")),
			v.option("CATALOG_NAMESPACE", str),
			v.option("REST_CONFIG", restConfig),
			v.option("REST_AUTHENTICATION", restAuth),
			v.option("ENABLED", v.parseBool),
			v.option("REFRESH_INTERVAL_SECONDS", v.parseNumber),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateObjClone validates the Snowflake `CREATE <object> CLONE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-clone
//
// Syntax:
//
//	CREATE [ OR REPLACE ] { DATABASE | SCHEMA } [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	    [ IGNORE HYBRID TABLES ]
//	    [ INCLUDE INTERNAL STAGES ]
//	  ...
//
//	CREATE [ OR REPLACE ] TABLE [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	  ...
//
//	CREATE [ OR REPLACE ] DYNAMIC TABLE <name>
//	  CLONE <source_dynamic_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	  [
//	    TARGET_LAG = { '<num> { seconds | minutes | hours | days }' | DOWNSTREAM }
//	    WAREHOUSE = <warehouse_name>
//	  ]
//
//	CREATE [ OR REPLACE ] EVENT TABLE <name>
//	  CLONE <source_event_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <name>
//	  CLONE <source_iceberg_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ COPY GRANTS ]
//	    ...
//
//	CREATE [ OR REPLACE ] DATABASE ROLE [ IF NOT EXISTS ] <database_role_name>
//	  CLONE <source_database_role_name>
//
//	CREATE [ OR REPLACE ] { ALERT | FILE FORMAT | SEQUENCE | STAGE | STREAM | TASK }
//	  [ IF NOT EXISTS ] <object_name>
//	  CLONE <source_object_name>
//	  ...
func (v *Validator) ParseCreateObjClone() bool {
	// [ { AT | BEFORE } ( { TIMESTAMP => … | OFFSET => … | STATEMENT => … } ) ]
	pointInTime := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				v.wordsValue("AT", "BEFORE"),
				func() bool { return v.Match(sqltok.LParen) },
				func() bool {
					return v.Choice(
						v.arrowOption("TIMESTAMP"),
						v.arrowOption("OFFSET"),
						v.arrowOption("STATEMENT"),
					)
				},
				func() bool { return v.Match(sqltok.RParen) },
			)
		})
	}
	// The various object-type prefixes that may precede the name.
	objKind := func() bool {
		return v.Choice(
			func() bool { return v.phrase("DATABASE", "ROLE") },
			func() bool { return v.phrase("DYNAMIC", "TABLE") },
			func() bool { return v.phrase("EVENT", "TABLE") },
			func() bool { return v.phrase("ICEBERG", "TABLE") },
			func() bool { return v.phrase("FILE", "FORMAT") },
			v.wordsValue("DATABASE", "SCHEMA", "TABLE"),
			v.wordsValue("ALERT", "SEQUENCE", "STAGE", "STREAM", "TASK"),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		objKind,
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("CLONE") },
		v.parseIdentPath,
		pointInTime,
		// Permissive trailing options (TARGET_LAG/WAREHOUSE, COPY GRANTS,
		// IGNORE … , INCLUDE INTERNAL STAGES, etc.).
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("TARGET_LAG", v.parseScalar),
					v.option("WAREHOUSE", v.parseIdentPath),
					func() bool { return v.phrase("COPY", "GRANTS") },
					func() bool { return v.phrase("INCLUDE", "INTERNAL", "STAGES") },
					func() bool { return v.phrase("IGNORE", "HYBRID", "TABLES") },
					func() bool {
						return v.phrase("IGNORE", "TABLES", "WITH", "INSUFFICIENT", "DATA", "RETENTION")
					},
				)
			})
		},
	)
}

// ParseCreateComputePool validates the Snowflake `CREATE COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-compute-pool
//
// Syntax:
//
//	CREATE COMPUTE POOL [ IF NOT EXISTS ] <name>
//	  [ FOR APPLICATION <app-name> ]
//	  MIN_NODES = <num>
//	  MAX_NODES = <num>
//	  INSTANCE_FAMILY = <instance_family_name>
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ INITIALLY_SUSPENDED = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND_SECS = <num>  ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ PLACEMENT_GROUP = '<placement_group_name>' ]
func (v *Validator) ParseCreateComputePool() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("COMPUTE") },
		func() bool { return v.MatchWord("POOL") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ FOR APPLICATION <app-name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("FOR") },
					func() bool { return v.MatchWord("APPLICATION") },
					v.parseIdentPath,
				)
			})
		},
		// The remaining options may appear in any order (MIN_NODES / MAX_NODES /
		// INSTANCE_FAMILY are required but order-independent in practice).
		func() bool {
			return v.unorderedOnce(
				v.option("MIN_NODES", num),
				v.option("MAX_NODES", num),
				v.option("INSTANCE_FAMILY", v.parseIdentPath),
				v.option("AUTO_RESUME", v.parseBool),
				v.option("INITIALLY_SUSPENDED", v.parseBool),
				v.option("AUTO_SUSPEND_SECS", num),
				v.option("PLACEMENT_GROUP", v.parseString),
				v.tagClause,
				v.commentOption(),
			)
		},
	)
}

// ParseCreateConnection validates the Snowflake `CREATE CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-connection
//
// Syntax:
//
//	-- Primary Connection
//	CREATE CONNECTION [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	-- Secondary Connection
//	CREATE CONNECTION [ IF NOT EXISTS ] <name>
//	  AS REPLICA OF <organization_name>.<account_name>.<name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateConnection() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("CONNECTION") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ AS REPLICA OF <org>.<account>.<name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("AS") },
					func() bool { return v.MatchWord("REPLICA") },
					func() bool { return v.MatchKeyword("OF") },
					v.parseIdentPath,
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateContact validates the Snowflake `CREATE CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-contact
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CONTACT [ IF NOT EXISTS ] <name>
//	  [ {
//	    USERS = ( '<user_name>' [ , '<user_name>' ... ] )
//	    | EMAIL_DISTRIBUTION_LIST = '<email>'
//	    | URL = '<url>'
//	    } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateContact() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CONTACT") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ { USERS = ( ... ) | EMAIL_DISTRIBUTION_LIST = '...' | URL = '...' } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					v.option("USERS", func() bool { return v.parseParenList(v.parseString) }),
					v.option("EMAIL_DISTRIBUTION_LIST", v.parseString),
					v.option("URL", v.parseString),
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateCortexSearchService validates the Snowflake `CREATE CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-cortex-search
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CORTEX SEARCH SERVICE [ IF NOT EXISTS ] <name>
//	  ON <search_column>
//	  [ PRIMARY KEY ( <col_name> [, ... ] ) ]
//	  ATTRIBUTES <col_name> [ , ... ]
//	  WAREHOUSE = <warehouse_name>
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  [ EMBEDDING_MODEL = <embedding_model_name> ]
//	  [ REFRESH_MODE = { FULL | INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ FULL_INDEX_BUILD_INTERVAL_DAYS = <num> ]
//	  [ REQUEST_LOGGING = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND = <num_seconds> ]
//	  [ COMMENT = '<comment>' ]
//	AS <query>;
//
//	CREATE [ OR REPLACE ] CORTEX SEARCH SERVICE <name>
//	  TEXT INDEXES <text_column_name> [ , ... ]
//	  VECTOR INDEXES <column_specification> [ , ... ]
//	  [ PRIMARY KEY ( <col_name> [, ... ] ) ]
//	  ATTRIBUTES <col_name> [ , ... ]
//	  WAREHOUSE = <warehouse_name>
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  [ REFRESH_MODE = { FULL | INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ FULL_INDEX_BUILD_INTERVAL_DAYS = <num> ]
//	  [ REQUEST_LOGGING = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND = <num_seconds> ]
//	  [ COMMENT = '<comment>' ]
//	AS <query>;
func (v *Validator) ParseCreateCortexSearchService() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// A comma-separated, unparenthesized list of items.
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(
				item,
				func() bool {
					return v.ZeroOrMore(func() bool {
						return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
					})
				},
			)
		}
	}
	// [ PRIMARY KEY ( <col> [, ...] ) ]
	primaryKey := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				func() bool { return v.MatchKeyword("PRIMARY") },
				func() bool { return v.MatchKeyword("KEY") },
				func() bool { return v.parseParenList(v.parseIdentPath) },
			)
		})
	}
	// ATTRIBUTES <col> [, ...]
	attributes := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("ATTRIBUTES") },
			commaList(v.parseIdentPath),
		)
	}
	// The shared trailing option list.
	options := func() bool {
		return v.unorderedOnce(
			v.option("WAREHOUSE", v.parseIdentPath),
			v.option("TARGET_LAG", v.parseString),
			v.option("EMBEDDING_MODEL", v.parseIdentPath),
			v.option("REFRESH_MODE", v.wordsValue("FULL", "INCREMENTAL")),
			v.option("INITIALIZE", v.wordsValue("ON_CREATE", "ON_SCHEDULE")),
			v.option("FULL_INDEX_BUILD_INTERVAL_DAYS", num),
			v.option("REQUEST_LOGGING", v.parseBool),
			v.option("AUTO_SUSPEND", num),
			v.commentOption(),
		)
	}
	// AS <query> — consume a permissive run of the remaining tokens.
	asQuery := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("AS") },
			func() bool { return !v.AtEnd() },
			func() bool {
				return v.ZeroOrMore(func() bool {
					if v.AtEnd() {
						return false
					}
					v.advance()
					return true
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CORTEX") },
		func() bool { return v.MatchWord("SEARCH") },
		func() bool { return v.MatchWord("SERVICE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// Form 1: ON <col> [ PRIMARY KEY ] ATTRIBUTES … options AS …
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("ON") },
						v.parseIdentPath,
						primaryKey,
						attributes,
						options,
						asQuery,
					)
				},
				// Form 2: TEXT INDEXES … VECTOR INDEXES … [ PRIMARY KEY ] ATTRIBUTES … options AS …
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("TEXT") },
						func() bool { return v.MatchWord("INDEXES") },
						commaList(v.parseIdentPath),
						func() bool { return v.MatchWord("VECTOR") },
						func() bool { return v.MatchWord("INDEXES") },
						commaList(v.parseIdentPath),
						primaryKey,
						attributes,
						options,
						asQuery,
					)
				},
			)
		},
	)
}

// ParseCreateDataMetricFunction validates the Snowflake `CREATE DATA METRIC FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-data-metric-function
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] DATA METRIC FUNCTION [ IF NOT EXISTS ] <name>
//	  ( <table_arg> TABLE( <col_arg> <data_type> [ , ... ] )
//	    [ , <table_arg> TABLE( <col_arg> <data_type> [ , ... ] ) ] )
//	  RETURNS NUMBER [ [ NOT ] NULL ]
//	  [ LANGUAGE SQL ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS
//	  '<expression>'
func (v *Validator) ParseCreateDataMetricFunction() bool {
	// <col_arg> <data_type> — a name followed by a (permissive) type token path.
	colArg := func() bool {
		return v.Sequence(v.parseIdentPath, v.parseIdentPath)
	}
	// <table_arg> TABLE( <col_arg> <data_type> [ , ... ] )
	tableArg := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.MatchWord("TABLE") },
			func() bool { return v.parseParenList(colArg) },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURE") }) },
		func() bool { return v.MatchWord("DATA") },
		func() bool { return v.MatchWord("METRIC") },
		func() bool { return v.MatchWord("FUNCTION") },
		v.ifNotExists,
		v.parseIdentPath,
		// ( <table_arg> TABLE(...) [ , <table_arg> TABLE(...) ] )
		func() bool { return v.parseParenList(tableArg) },
		// RETURNS NUMBER [ [ NOT ] NULL ]
		func() bool { return v.MatchKeyword("RETURNS") },
		func() bool { return v.MatchWord("NUMBER") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("NOT") }) },
					func() bool { return v.MatchWord("NULL") },
				)
			})
		},
		// [ LANGUAGE SQL ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("LANGUAGE") },
					func() bool { return v.MatchWord("SQL") },
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
		// AS '<expression>'
		func() bool { return v.MatchKeyword("AS") },
		func() bool {
			return v.Choice(
				v.parseString,
				// permissive run for an unquoted expression body
				func() bool {
					return v.Sequence(
						func() bool { return !v.AtEnd() },
						func() bool {
							return v.ZeroOrMore(func() bool {
								if v.AtEnd() {
									return false
								}
								v.advance()
								return true
							})
						},
					)
				},
			)
		},
	)
}

// ParseCreateDatabase validates the Snowflake `CREATE DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] DATABASE [ IF NOT EXISTS ] <name>
//	    [ CLONE <source_schema>
//	        [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	        [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	        [ IGNORE HYBRID TABLES ] ]
//	    [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	    [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	    [ EXTERNAL_VOLUME = <external_volume_name> ]
//	    [ CATALOG = <catalog_integration_name> ]
//	    [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	    [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	    [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	    [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	    [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	    [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	    [ CATALOG_SYNC_NAMESPACE_MODE = { NEST | FLATTEN } ]
//	    [ CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER = '<string_literal>' ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	    [ OBJECT_VISIBILITY = { <object_visibility_spec> | PRIVILEGED } ]
//	    [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//
//	CREATE DATABASE <name> FROM BACKUP SET <backup_set> IDENTIFIER '<backup_id>'
//
//	CREATE DATABASE <name> FROM LISTING '<listing_global_name>'
//
//	CREATE DATABASE <name> FROM SHARE <provider_account>.<share_name>
//
//	CREATE DATABASE <name>
//	    AS REPLICA OF <account_identifier>.<primary_db_name>
//	    [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//
//	CREATE OR ALTER [ TRANSIENT ] DATABASE <name>
//	    [ ... database properties ... ]
func (v *Validator) ParseCreateDatabase() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		// [ TRANSIENT ]
		func() bool { return v.Optional(func() bool { return v.MatchWord("TRANSIENT") }) },
		func() bool { return v.MatchKeyword("DATABASE") },
		// [ IF NOT EXISTS ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("IF") },
					func() bool { return v.MatchKeyword("NOT") },
					func() bool { return v.MatchKeyword("EXISTS") },
				)
			})
		},
		// <name>
		v.parseIdentPath,
		// One of the trailing forms: FROM …, AS REPLICA OF …, or
		// [ CLONE … ] followed by the database property list.
		func() bool {
			return v.Choice(
				v.parseDatabaseFromClause,
				v.parseDatabaseReplicaClause,
				v.parseDatabaseCreateBody,
			)
		},
	)
}

// parseDatabaseFromClause matches the source-backed creation forms:
//
//	FROM BACKUP SET <backup_set> IDENTIFIER '<backup_id>'
//	FROM LISTING '<listing_global_name>'
//	FROM SHARE <provider_account>.<share_name>
func (v *Validator) parseDatabaseFromClause() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("FROM") },
		func() bool {
			return v.Choice(
				// BACKUP SET <name> IDENTIFIER '<id>'
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("BACKUP") },
						func() bool { return v.MatchKeyword("SET") },
						v.parseIdentPath,
						func() bool { return v.MatchWord("IDENTIFIER") },
						func() bool { return v.Match(sqltok.StringLit) },
					)
				},
				// LISTING '<global_name>'
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("LISTING") },
						func() bool { return v.Match(sqltok.StringLit) },
					)
				},
				// SHARE <provider_account>.<share_name>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("SHARE") },
						v.parseIdentPath,
					)
				},
			)
		},
	)
}

// parseDatabaseReplicaClause matches:
//
//	AS REPLICA OF <account_identifier>.<primary_db_name>
//	    [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
func (v *Validator) parseDatabaseReplicaClause() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return v.MatchWord("REPLICA") },
		func() bool { return v.MatchKeyword("OF") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.option("DATA_RETENTION_TIME_IN_DAYS",
				func() bool { return v.Match(sqltok.NumberLit) }))
		},
	)
}

// parseDatabaseCreateBody matches the optional CLONE clause followed by the
// (order-independent) database property list. It always succeeds — a bare
// `CREATE DATABASE <name>` has no trailing clauses.
func (v *Validator) parseDatabaseCreateBody() bool {
	v.Optional(v.parseDatabaseCloneClause)
	return v.ZeroOrMore(v.parseDatabaseProperty)
}

// parseDatabaseCloneClause matches:
//
//	CLONE <source_db>
//	    [ { AT | BEFORE } ( { TIMESTAMP => … | OFFSET => … | STATEMENT => … } ) ]
//	    [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	    [ IGNORE HYBRID TABLES ]
func (v *Validator) parseDatabaseCloneClause() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CLONE") },
		v.parseIdentPath,
		// [ { AT | BEFORE } ( <point-in-time> ) ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("AT", "BEFORE"),
					func() bool { return v.Match(sqltok.LParen) },
					func() bool {
						return v.Choice(
							v.arrowOption("TIMESTAMP"),
							v.arrowOption("OFFSET"),
							v.arrowOption("STATEMENT"),
						)
					},
					func() bool { return v.Match(sqltok.RParen) },
				)
			})
		},
		// [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("IGNORE") },
					func() bool { return v.MatchWord("TABLES") },
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("INSUFFICIENT") },
					func() bool { return v.MatchWord("DATA") },
					func() bool { return v.MatchWord("RETENTION") },
				)
			})
		},
		// [ IGNORE HYBRID TABLES ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("IGNORE") },
					func() bool { return v.MatchWord("HYBRID") },
					func() bool { return v.MatchWord("TABLES") },
				)
			})
		},
	)
}

// arrowOption builds a rule matching `<key> => <value>` (the AT/BEFORE point-in-
// time arguments).
func (v *Validator) arrowOption(key string) Rule {
	return func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord(key) },
			func() bool { return v.MatchOp("=>") },
			v.parseScalar,
		)
	}
}

// parseDatabaseProperty matches one entry of the database property list. The
// properties may appear in any order, so the caller drives this with ZeroOrMore.
func (v *Validator) parseDatabaseProperty() bool {
	str := func() bool { return v.Match(sqltok.StringLit) }
	intLit := func() bool { return v.Match(sqltok.NumberLit) }
	name := v.parseIdentPath

	return v.Choice(
		v.option("DATA_RETENTION_TIME_IN_DAYS", intLit),
		v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", intLit),
		v.option("EXTERNAL_VOLUME", name),
		v.option("CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER", str),
		v.option("CATALOG_SYNC_NAMESPACE_MODE", v.wordsValue("NEST", "FLATTEN")),
		v.option("CATALOG_SYNC", str),
		v.option("CATALOG", name),
		v.option("ICEBERG_VERSION_DEFAULT", intLit),
		v.option("ICEBERG_MERGE_ON_READ_BEHAVIOR", str),
		v.option("ENABLE_ICEBERG_MERGE_ON_READ", v.parseBool),
		v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
		v.option("DEFAULT_DDL_COLLATION", str),
		v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
		v.option("COMMENT", str),
		v.option("OBJECT_VISIBILITY", v.parseScalar),
		v.option("ENABLE_DATA_COMPACTION", v.parseBool),
		v.parseDatabaseTagClause,
		v.parseDatabaseContactClause,
	)
}

// parseDatabaseTagClause matches:
//
//	[ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ] )
func (v *Validator) parseDatabaseTagClause() bool {
	return v.Sequence(
		func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
		func() bool { return v.MatchWord("TAG") },
		func() bool {
			return v.parseParenList(v.option2(v.parseIdentPath, func() bool { return v.Match(sqltok.StringLit) }))
		},
	)
}

// parseDatabaseContactClause matches:
//
//	WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] )
func (v *Validator) parseDatabaseContactClause() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("WITH") },
		func() bool { return v.MatchWord("CONTACT") },
		func() bool {
			return v.parseParenList(v.option2(v.parseIdentPath, v.parseIdentPath))
		},
	)
}

// option2 builds a rule matching `<key> = <value>` where both sides are given
// as rules (used for the `( name = value, … )` entries of TAG / CONTACT).
func (v *Validator) option2(keyRule, valueRule Rule) Rule {
	return func() bool {
		return v.Sequence(
			keyRule,
			func() bool { return v.MatchOp("=") },
			valueRule,
		)
	}
}

// parseParenList matches `( item [ , item ]* )`.
func (v *Validator) parseParenList(item Rule) bool {
	return v.Sequence(
		func() bool { return v.Match(sqltok.LParen) },
		item,
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
			})
		},
		func() bool { return v.Match(sqltok.RParen) },
	)
}

// ParseCreateDatabaseCatalogLinked validates the Snowflake `CREATE DATABASE (catalog-linked)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-catalog-linked
//
// Syntax:
//
//	CREATE DATABASE <name>
//	  LINKED_CATALOG = ( catalogParams ),
//	  [ EXTERNAL_VOLUME = '<external_vol>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ CATALOG_CASE_SENSITIVITY = { CASE_SENSITIVE | CASE_INSENSITIVE } ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	Where:
//
//	catalogParams ::=
//	  CATALOG = '<catalog_int>',
//	  [ ALLOWED_NAMESPACES = ('<namespace1>', '<namespace2>', ... ) ]
//	  [ BLOCKED_NAMESPACES = ('<namespace1>', '<namespace2>', ... ) ]
//	  [ ALLOWED_WRITE_OPERATIONS = { NONE | ALL } ]
//	  [ NAMESPACE_MODE = { IGNORE_NESTED_NAMESPACE | FLATTEN_NESTED_NAMESPACE } ]
//	  [ NAMESPACE_FLATTEN_DELIMITER = '<string_literal>' ]
//	  [ SYNC_INTERVAL_SECONDS = <value> ]
func (v *Validator) ParseCreateDatabaseCatalogLinked() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// catalogParams: a CATALOG = '...' plus optional namespace/operation params,
	// each optionally separated by commas.
	catalogParam := func() bool {
		return v.Choice(
			v.option("CATALOG", v.parseString),
			v.option("ALLOWED_NAMESPACES", func() bool { return v.parseParenList(v.parseString) }),
			v.option("BLOCKED_NAMESPACES", func() bool { return v.parseParenList(v.parseString) }),
			v.option("ALLOWED_WRITE_OPERATIONS", v.wordsValue("NONE", "ALL")),
			v.option("NAMESPACE_MODE", v.wordsValue("IGNORE_NESTED_NAMESPACE", "FLATTEN_NESTED_NAMESPACE")),
			v.option("NAMESPACE_FLATTEN_DELIMITER", v.parseString),
			v.option("SYNC_INTERVAL_SECONDS", num),
		)
	}
	catalogParams := func() bool {
		return v.Sequence(
			catalogParam,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(
						func() bool { return v.Optional(func() bool { return v.Match(sqltok.Comma) }) },
						catalogParam,
					)
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchKeyword("DATABASE") },
		v.parseIdentPath,
		// LINKED_CATALOG = ( catalogParams )
		func() bool { return v.MatchWord("LINKED_CATALOG") },
		func() bool { return v.MatchOp("=") },
		func() bool { return v.Match(sqltok.LParen) },
		catalogParams,
		func() bool { return v.Match(sqltok.RParen) },
		// the trailing properties, comma-separated or not
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.Match(sqltok.Comma) }) },
					func() bool {
						return v.Choice(
							v.option("EXTERNAL_VOLUME", v.parseString),
							v.option("CATALOG_CASE_SENSITIVITY", v.wordsValue("CASE_SENSITIVE", "CASE_INSENSITIVE")),
							v.commentOption(),
							v.tagClause,
							func() bool {
								return v.Sequence(
									func() bool { return v.MatchKeyword("WITH") },
									func() bool { return v.MatchWord("CONTACT") },
									func() bool {
										return v.parseParenList(v.option2(v.parseIdentPath, v.parseIdentPath))
									},
								)
							},
						)
					},
				)
			})
		},
	)
}

// ParseCreateDatabaseRole validates the Snowflake `CREATE DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-database-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DATABASE ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER DATABASE ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateDatabaseRole() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchKeyword("DATABASE") },
		func() bool { return v.MatchWord("ROLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ CLONE <source_database_role_name> ] or [ COMMENT = '...' ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("CLONE") },
					v.parseIdentPath,
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateDataset validates the Snowflake `CREATE DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dataset
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ IF NOT EXISTS ] DATASET <name>
func (v *Validator) ParseCreateDataset() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		v.ifNotExists,
		func() bool { return v.MatchWord("DATASET") },
		v.parseIdentPath,
	)
}

// ParseCreateDbtProject validates the Snowflake `CREATE DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dbt-project
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DBT PROJECT [ IF NOT EXISTS ] <name>
//	  [ FROM '<source_location>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ DBT_VERSION = <version_number> ]
//	  [ DEFAULT_TARGET = <default_target> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
func (v *Validator) ParseCreateDbtProject() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("DBT") },
		func() bool { return v.MatchWord("PROJECT") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ FROM '<source_location>' ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("FROM") },
					v.parseString,
				)
			})
		},
		func() bool {
			return v.unorderedOnce(
				v.commentOption(),
				v.option("DBT_VERSION", v.parseScalar),
				v.option("DEFAULT_TARGET", v.parseIdentPath),
				v.option("EXTERNAL_ACCESS_INTEGRATIONS", func() bool { return v.parseParenList(v.parseIdentPath) }),
			)
		},
	)
}

// ParseCreateDcmProject validates the Snowflake `CREATE DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dcm-project
//
// Syntax:
//
//	CREATE [ OR REPLACE ] DCM PROJECT [ IF NOT EXISTS ] <name>
//	  [LOG_LEVEL = { DEBUG | INFO | WARN | ERROR }]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateDcmProject() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("DCM") },
		func() bool { return v.MatchWord("PROJECT") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("LOG_LEVEL", v.wordsValue("DEBUG", "INFO", "WARN", "ERROR")),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateDynamicTable validates the Snowflake `CREATE DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-dynamic-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] DYNAMIC TABLE [ IF NOT EXISTS ] <name> (
//	    -- Column definition
//	    <col_name> <col_type>
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	      [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	    -- Additional column definitions
//	    [ , <col_name> <col_type> [ ... ] ]
//	  )
//	  TARGET_LAG = { '<num> { seconds | minutes | hours | days }' | DOWNSTREAM }
//	  [ SCHEDULER = DISABLE | ENABLE ]
//	  WAREHOUSE = <warehouse_name>
//	  [ INITIALIZATION_WAREHOUSE = <warehouse_name> ]
//	  [ REFRESH_MODE = { AUTO | FULL | INCREMENTAL | ADAPTIVE | CUSTOM_INCREMENTAL } ]
//	  [ INITIALIZE = { ON_CREATE | ON_SCHEDULE } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ REQUIRE USER ]
//	  [ FROZEN WHERE ( <expr> ) ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ BACKFILL FROM <table_name> ]
//	  [ START AT ({ STREAM => '<stream_name>' | TIMESTAMP => <timestamp> | STATEMENT => <query_id> | OFFSET => -<seconds> }) ]
//	  [ EXECUTE AS USER <user_name>
//	    [ USE SECONDARY ROLES { ALL | NONE | <role> [ , ... ] } ]
//	  ]
//	  [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//	  { AS <query> | REFRESH USING ( <dml_statement> ) }
func (v *Validator) ParseCreateDynamicTable() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// Permissive consumer for a balanced-paren span (column definition list,
	// CLUSTER BY exprs, etc.). Assumes the opening LParen is the current token.
	colList := func() bool { return v.Optional(v.consumeBalancedParens) }
	policyOn := func(words ...string) Rule {
		return func() bool {
			return v.Sequence(
				func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
				func() bool { return v.phrase(words...) },
				v.parseIdentPath,
				func() bool {
					return v.Optional(func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("ON") },
							v.consumeBalancedParens,
						)
					})
				},
			)
		}
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("TRANSIENT") }) },
		func() bool { return v.MatchWord("DYNAMIC") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// GET_DDL emits CLUSTER BY before the column-alias list; accept it there
		// as well as in the trailing option list below (issue #776).
		func() bool { return v.Optional(v.clusterByClause(v.consumeBalancedParens)) },
		colList,
		// Order-independent option list.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("TARGET_LAG", func() bool { return v.Choice(v.parseString, v.wordsValue("DOWNSTREAM")) }),
					v.option("SCHEDULER", v.wordsValue("DISABLE", "ENABLE")),
					v.option("WAREHOUSE", v.parseIdentPath),
					v.option("INITIALIZATION_WAREHOUSE", v.parseIdentPath),
					v.option("REFRESH_MODE", v.wordsValue("AUTO", "FULL", "INCREMENTAL", "ADAPTIVE", "CUSTOM_INCREMENTAL")),
					v.option("INITIALIZE", v.wordsValue("ON_CREATE", "ON_SCHEDULE")),
					v.option("DATA_RETENTION_TIME_IN_DAYS", num),
					v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
					v.option("ROW_TIMESTAMP", v.parseBool),
					v.commentOption(),
					v.clusterByClause(v.consumeBalancedParens),
					func() bool { return v.phrase("COPY", "GRANTS") },
					func() bool { return v.phrase("COPY", "TAGS") },
					func() bool { return v.phrase("REQUIRE", "USER") },
					policyOn("ROW", "ACCESS", "POLICY"),
					policyOn("STORAGE", "LIFECYCLE", "POLICY"),
					// [ WITH ] AGGREGATION POLICY <name> [ ENTITY KEY ( ... ) ]
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
							func() bool { return v.phrase("AGGREGATION", "POLICY") },
							v.parseIdentPath,
							func() bool {
								return v.Optional(func() bool {
									return v.Sequence(func() bool { return v.phrase("ENTITY", "KEY") }, v.consumeBalancedParens)
								})
							},
						)
					},
					v.tagClause,
					// FROZEN WHERE ( <expr> )
					func() bool { return v.phrase("FROZEN", "WHERE") && v.consumeBalancedParens() },
					// BACKFILL FROM <table_name>
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("BACKFILL") },
							func() bool { return v.MatchKeyword("FROM") },
							v.parseIdentPath,
						)
					},
					// START AT ( ... )
					func() bool { return v.phrase("START", "AT") && v.consumeBalancedParens() },
					// EXECUTE AS USER <user> [ USE SECONDARY ROLES { ALL | NONE | <role> [, ...] } ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("EXECUTE") },
							func() bool { return v.MatchKeyword("AS") },
							func() bool { return v.MatchWord("USER") },
							v.parseIdentPath,
							func() bool {
								return v.Optional(func() bool {
									return v.Sequence(
										func() bool { return v.phrase("USE", "SECONDARY", "ROLES") },
										func() bool {
											return v.Choice(
												v.wordsValue("ALL", "NONE"),
												func() bool {
													return v.Sequence(
														v.parseIdentPath,
														func() bool {
															return v.ZeroOrMore(func() bool {
																return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseIdentPath)
															})
														},
													)
												},
											)
										},
									)
								})
							},
						)
					},
				)
			})
		},
		// { AS <query> | REFRESH USING ( <dml> ) }
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("AS") },
						func() bool { return !v.AtEnd() },
						func() bool {
							return v.ZeroOrMore(func() bool {
								if v.AtEnd() {
									return false
								}
								v.advance()
								return true
							})
						},
					)
				},
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("REFRESH") },
						func() bool { return v.MatchWord("USING") },
						v.consumeBalancedParens,
					)
				},
			)
		},
	)
}

// ParseCreateEventTable validates the Snowflake `CREATE EVENT TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-event-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EVENT TABLE [ IF NOT EXISTS ] <name>
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COPY GRANTS ]
//	  [ [ WITH ] COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	CREATE [ OR REPLACE ] EVENT TABLE [ IF NOT EXISTS ] <name>
//	  CLONE <source_table>
//	    [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	    [ COPY GRANTS ]
//	    [ ... ]
func (v *Validator) ParseCreateEventTable() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// [ { AT | BEFORE } ( … ) ]
	pointInTime := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				v.wordsValue("AT", "BEFORE"),
				func() bool { return v.Match(sqltok.LParen) },
				func() bool {
					return v.Choice(
						v.arrowOption("TIMESTAMP"),
						v.arrowOption("OFFSET"),
						v.arrowOption("STATEMENT"),
					)
				},
				func() bool { return v.Match(sqltok.RParen) },
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EVENT") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// CLONE form
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("CLONE") },
						v.parseIdentPath,
						pointInTime,
						func() bool {
							return v.Optional(func() bool { return v.phrase("COPY", "GRANTS") })
						},
					)
				},
				// property form
				func() bool {
					return v.ZeroOrMore(func() bool {
						return v.Choice(
							v.clusterByClause(v.consumeBalancedParens),
							v.option("DATA_RETENTION_TIME_IN_DAYS", num),
							v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
							v.option("CHANGE_TRACKING", v.parseBool),
							v.option("DEFAULT_DDL_COLLATION", v.parseString),
							func() bool { return v.phrase("COPY", "GRANTS") },
							// [ WITH ] COMMENT = '...'
							func() bool {
								return v.Sequence(
									func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
									v.commentOption(),
								)
							},
							// [ WITH ] ROW ACCESS POLICY <name> ON ( ... )
							func() bool {
								return v.Sequence(
									func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
									func() bool { return v.phrase("ROW", "ACCESS", "POLICY") },
									v.parseIdentPath,
									func() bool { return v.MatchKeyword("ON") },
									v.consumeBalancedParens,
								)
							},
							v.tagClause,
							// WITH CONTACT ( ... )
							func() bool {
								return v.Sequence(
									func() bool { return v.MatchKeyword("WITH") },
									func() bool { return v.MatchWord("CONTACT") },
									func() bool {
										return v.parseParenList(v.option2(v.parseIdentPath, v.parseIdentPath))
									},
								)
							},
						)
					})
				},
			)
		},
	)
}

// ParseCreateExperiment validates the Snowflake `CREATE EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-experiment
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXPERIMENT [ IF NOT EXISTS ] <name>
func (v *Validator) ParseCreateExperiment() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EXPERIMENT") },
		v.ifNotExists,
		v.parseIdentPath,
	)
}

// ParseCreateExternalAgent validates the Snowflake `CREATE EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-agent
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL AGENT [ IF NOT EXISTS ] <name>
//	  [ WITH VERSION <version_name> ]
//	  [ COMMENT = '<comment>' ]
func (v *Validator) ParseCreateExternalAgent() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("AGENT") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ WITH VERSION <version_name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("VERSION") },
					v.parseIdentPath,
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateExternalAccessIntegration validates the Snowflake `CREATE EXTERNAL ACCESS INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-access-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL ACCESS INTEGRATION <name>
//	  ALLOWED_NETWORK_RULES = ( <rule_name_1> [, <rule_name_2>, ... ] )
//	  [ ALLOWED_API_AUTHENTICATION_INTEGRATIONS = { ( <integration_name_1> [, <integration_name_2>, ... ] ) | none } ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = { ( <secret_name_1> [, <secret_name_2>, ... ] ) | all | none } ]
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateExternalAccessIntegration() bool {
	parenIdentList := func() bool { return v.parseParenList(v.parseIdentPath) }
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("ACCESS") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.parseIdentPath,
		// ALLOWED_NETWORK_RULES and ENABLED are required; the optional ones may
		// be interleaved, so accept the whole set order-independently.
		func() bool {
			return v.unorderedOnce(
				v.option("ALLOWED_NETWORK_RULES", parenIdentList),
				v.option("ALLOWED_API_AUTHENTICATION_INTEGRATIONS",
					func() bool { return v.Choice(parenIdentList, v.wordsValue("NONE")) }),
				v.option("ALLOWED_AUTHENTICATION_SECRETS",
					func() bool { return v.Choice(parenIdentList, v.wordsValue("ALL", "NONE")) }),
				v.option("ENABLED", v.parseBool),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateExternalFunction validates the Snowflake `CREATE EXTERNAL FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-function
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] EXTERNAL FUNCTION <name> ( [ <arg_name> <arg_data_type> ] [ , ... ] )
//	  RETURNS <result_data_type>
//	  [ [ NOT ] NULL ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  API_INTEGRATION = <api_integration_name>
//	  [ HEADERS = ( '<header_1>' = '<value_1>' [ , '<header_2>' = '<value_2>' ... ] ) ]
//	  [ CONTEXT_HEADERS = ( <context_function_1> [ , <context_function_2> ...] ) ]
//	  [ MAX_BATCH_ROWS = <integer> ]
//	  [ COMPRESSION = <compression_type> ]
//	  [ REQUEST_TRANSLATOR = <request_translator_udf_name> ]
//	  [ RESPONSE_TRANSLATOR = <response_translator_udf_name> ]
//	  AS '<url_of_proxy_and_resource>';
//
//	CREATE [ OR ALTER ] EXTERNAL FUNCTION ...
func (v *Validator) ParseCreateExternalFunction() bool {
	// Permissive consumer for a balanced-paren span (the argument list / RETURNS
	// TABLE column list). Assumes the opening LParen is the current token.
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// <result_data_type> — a (possibly parameterized) type name.
	dataType := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.Optional(v.consumeBalancedParens) },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURE") }) },
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("FUNCTION") },
		v.parseIdentPath,
		// ( [ <arg_name> <arg_data_type> ] [ , ... ] )
		v.consumeBalancedParens,
		func() bool { return v.MatchKeyword("RETURNS") },
		dataType,
		// [ [ NOT ] NULL ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("NOT") }) },
					func() bool { return v.MatchWord("NULL") },
				)
			})
		},
		// [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.phrase("CALLED", "ON", "NULL", "INPUT") },
					func() bool { return v.phrase("RETURNS", "NULL", "ON", "NULL", "INPUT") },
					func() bool { return v.MatchWord("STRICT") },
				)
			})
		},
		// [ { VOLATILE | IMMUTABLE } ]
		func() bool { return v.Optional(v.wordsValue("VOLATILE", "IMMUTABLE")) },
		// Order-independent option list (API_INTEGRATION is required but accept
		// any order). The trailing AS clause is matched after the loop.
		func() bool {
			return v.unorderedOnce(
				v.commentOption(),
				v.option("API_INTEGRATION", v.parseIdentPath),
				v.option("HEADERS", v.consumeBalancedParens),
				v.option("CONTEXT_HEADERS", v.consumeBalancedParens),
				v.option("MAX_BATCH_ROWS", num),
				v.option("COMPRESSION", v.parseScalar),
				v.option("REQUEST_TRANSLATOR", v.parseIdentPath),
				v.option("RESPONSE_TRANSLATOR", v.parseIdentPath),
			)
		},
		// AS '<url_of_proxy_and_resource>'
		func() bool { return v.MatchKeyword("AS") },
		v.parseString,
	)
}

// ParseCreateExternalTable validates the Snowflake `CREATE EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-table
//
// Syntax:
//
//	-- Partitions computed from expressions
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( [ <col_name> <col_type> AS <expr> | <part_col_name> <col_type> AS <part_expr> ]
//	      [ inlineConstraint ]
//	      [ , ... ] )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  [ REFRESH_ON_CREATE =  { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ PATTERN = '<regex_pattern>' ]
//	  FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET } [ formatTypeOptions ] } )
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON (VALUE) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	-- Partitions added and removed manually
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( ... )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  PARTITION_TYPE = USER_SPECIFIED
//	  FILE_FORMAT = ( ... )
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ... ]
//
//	-- Delta Lake
//	CREATE [ OR REPLACE ] EXTERNAL TABLE [IF NOT EXISTS]
//	  <table_name>
//	    ( ... )
//	  cloudProviderParams
//	  [ PARTITION BY ( <part_col_name> [, <part_col_name> ... ] ) ]
//	  [ WITH ] LOCATION = externalStage
//	  PARTITION_TYPE = USER_SPECIFIED
//	  FILE_FORMAT = ( ... )
//	  [ TABLE_FORMAT = DELTA ]
//	  [ COPY GRANTS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ... ]
func (v *Validator) ParseCreateExternalTable() bool {
	// externalStage value: @stage[/path] , a string literal, or an identifier path.
	stageValue := func() bool {
		return v.Choice(
			v.parseStageRef,
			v.parseString,
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// ( column / partition definitions ) — permissive paren span.
		func() bool { return v.Optional(v.consumeBalancedParens) },
		// Order-independent option / clause list.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					// [ PARTITION BY ( ... ) ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("PARTITION") },
							func() bool { return v.MatchKeyword("BY") },
							v.consumeBalancedParens,
						)
					},
					// [ WITH ] LOCATION = externalStage
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
							func() bool { return v.MatchWord("LOCATION") },
							func() bool { return v.MatchOp("=") },
							stageValue,
						)
					},
					v.option("REFRESH_ON_CREATE", v.parseBool),
					v.option("AUTO_REFRESH", v.parseBool),
					v.option("PATTERN", v.parseString),
					v.option("PARTITION_TYPE", v.wordsValue("USER_SPECIFIED")),
					v.option("FILE_FORMAT", v.consumeBalancedParens),
					v.option("TABLE_FORMAT", v.wordsValue("DELTA")),
					v.option("AWS_SNS_TOPIC", v.parseString),
					// [ COPY GRANTS ]
					func() bool { return v.phrase("COPY", "GRANTS") },
					v.commentOption(),
					// [ WITH ] ROW ACCESS POLICY <name> ON ( ... )
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
							func() bool { return v.phrase("ROW", "ACCESS", "POLICY") },
							v.parseIdentPath,
							func() bool { return v.MatchKeyword("ON") },
							v.consumeBalancedParens,
						)
					},
					v.tagClause,
					// WITH CONTACT ( ... )
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("WITH") },
							func() bool { return v.MatchWord("CONTACT") },
							v.consumeBalancedParens,
						)
					},
				)
			})
		},
	)
}

// ParseCreateExternalVolume validates the Snowflake `CREATE EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-external-volume
//
// Syntax:
//
//	CREATE [ OR REPLACE ] EXTERNAL VOLUME [ IF NOT EXISTS ]
//	  <name>
//	  STORAGE_LOCATIONS =
//	    (
//	      (
//	        NAME = '<storage_location_name>'
//	        { cloudProviderParams | s3CompatibleStorageParams }
//	      )
//	      [, (...), ...]
//	    )
//	  [ ALLOW_WRITES = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	cloudProviderParams (for Amazon S3) ::=
//	  STORAGE_PROVIDER = '{ S3 | S3GOV }'
//	  STORAGE_AWS_ROLE_ARN = '<iam_role>'
//	  STORAGE_BASE_URL = '<protocol>://<bucket>[/<path>/]'
//	  [ STORAGE_AWS_ACCESS_POINT_ARN = '<string>' ]
//	  [ STORAGE_AWS_EXTERNAL_ID = '<external_id>' ]
//	  [ ENCRYPTION = ( [ TYPE = 'AWS_SSE_S3' ] |
//	              [ TYPE = 'AWS_SSE_KMS' [ KMS_KEY_ID = '<string>' ] ] |
//	              [ TYPE = 'NONE' ] ) ]
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	cloudProviderParams (for Google Cloud Storage) ::=
//	  STORAGE_PROVIDER = 'GCS'
//	  STORAGE_BASE_URL = 'gcs://<bucket>[/<path>/]'
//	  [ ENCRYPTION = ( [ TYPE = 'GCS_SSE_KMS' ] [ KMS_KEY_ID = '<string>' ] |
//	              [ TYPE = 'NONE' ] ) ]
//
//	cloudProviderParams (for Microsoft Azure) ::=
//	  STORAGE_PROVIDER = 'AZURE'
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  STORAGE_BASE_URL = 'azure://...'
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	s3CompatibleStorageParams ::=
//	  STORAGE_PROVIDER = 'S3COMPAT'
//	  STORAGE_BASE_URL = 's3compat://<bucket>[/<path>/]'
//	  CREDENTIALS = ( AWS_KEY_ID = '<string>' AWS_SECRET_KEY = '<string>' )
//	  STORAGE_ENDPOINT = '<s3_api_compatible_endpoint>'
func (v *Validator) ParseCreateExternalVolume() bool {
	// STORAGE_LOCATIONS is an outer paren list whose entries are themselves
	// parenthesized property blocks; consume the whole value permissively as a
	// balanced-paren span.
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("VOLUME") },
		v.ifNotExists,
		v.parseIdentPath,
		// Order-independent: STORAGE_LOCATIONS (required) plus options.
		func() bool {
			return v.unorderedOnce(
				v.option("STORAGE_LOCATIONS", v.consumeBalancedParens),
				v.option("ALLOW_WRITES", v.parseBool),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateFailoverGroup validates the Snowflake `CREATE FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-failover-group
//
// Syntax:
//
//	CREATE FAILOVER GROUP [ IF NOT EXISTS ] <name>
//	    OBJECT_TYPES = <object_type> [ , <object_type> , ... ]
//	    [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	    [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	    [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//	    [ ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> , ... ] ]
//	    ALLOWED_ACCOUNTS = <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ]
//	    [ IGNORE EDITION CHECK ]
//	    [ REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	    [ OPTIMIZED_REFRESH = { TRUE | FALSE } ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//
//	CREATE FAILOVER GROUP [ IF NOT EXISTS ] <secondary_name>
//	    AS REPLICA OF <org_name>.<source_account_name>.<name>
func (v *Validator) ParseCreateFailoverGroup() bool {
	// A bare comma list of identifier-like values: A [ , A ... ].
	identList := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseIdentPath)
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("FAILOVER") },
		func() bool { return v.MatchWord("GROUP") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// AS REPLICA OF <org>.<account>.<name>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("AS") },
						func() bool { return v.MatchWord("REPLICA") },
						func() bool { return v.MatchKeyword("OF") },
						v.parseIdentPath,
					)
				},
				// Primary form: order-independent option list.
				func() bool {
					return v.ZeroOrMore(func() bool {
						return v.Choice(
							v.option("OBJECT_TYPES", identList),
							v.option("ALLOWED_DATABASES", identList),
							v.option("ALLOWED_EXTERNAL_VOLUMES", identList),
							v.option("ALLOWED_SHARES", identList),
							v.option("ALLOWED_INTEGRATION_TYPES", identList),
							v.option("ALLOWED_ACCOUNTS", identList),
							func() bool { return v.phrase("IGNORE", "EDITION", "CHECK") },
							v.option("REPLICATION_SCHEDULE", v.parseString),
							v.option("OPTIMIZED_REFRESH", v.parseBool),
							v.tagClause,
							v.option("ERROR_INTEGRATION", v.parseIdentPath),
						)
					})
				},
			)
		},
	)
}

// ParseCreateFeaturePolicy validates the Snowflake `CREATE FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-feature-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] FEATURE POLICY [ IF NOT EXISTS ] <name>
//	  BLOCKED_OBJECT_TYPES_FOR_CREATION = ( <type> [ , ... ] )
//	  [ COMMENT = '<string-literal>' ]
func (v *Validator) ParseCreateFeaturePolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("FEATURE") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		v.option("BLOCKED_OBJECT_TYPES_FOR_CREATION",
			func() bool { return v.parseParenList(v.parseIdentPath) }),
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateFileFormat validates the Snowflake `CREATE FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-file-format
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY | VOLATILE } ] FILE FORMAT [ IF NOT EXISTS ] <name>
//	  [ TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML } [ formatTypeOptions ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	formatTypeOptions ::=
//	-- If TYPE = CSV
//	     COMPRESSION = AUTO | GZIP | BZ2 | BROTLI | ZSTD | DEFLATE | RAW_DEFLATE | NONE
//	     RECORD_DELIMITER = '<string>' | NONE
//	     FIELD_DELIMITER = '<string>' | NONE
//	     MULTI_LINE = TRUE | FALSE
//	     FILE_EXTENSION = '<string>'
//	     PARSE_HEADER = TRUE | FALSE
//	     SKIP_HEADER = <integer>
//	     SKIP_BLANK_LINES = TRUE | FALSE
//	     DATE_FORMAT = '<string>' | AUTO
//	     TIME_FORMAT = '<string>' | AUTO
//	     TIMESTAMP_FORMAT = '<string>' | AUTO
//	     BINARY_FORMAT = HEX | BASE64 | UTF8
//	     ESCAPE = '<character>' | NONE
//	     ESCAPE_UNENCLOSED_FIELD = '<character>' | NONE
//	     TRIM_SPACE = TRUE | FALSE
//	     FIELD_OPTIONALLY_ENCLOSED_BY = '<character>' | NONE
//	     NULL_IF = ( '<string>' [ , '<string>' ... ] )
//	     ERROR_ON_COLUMN_COUNT_MISMATCH = TRUE | FALSE
//	     REPLACE_INVALID_CHARACTERS = TRUE | FALSE
//	     EMPTY_FIELD_AS_NULL = TRUE | FALSE
//	     SKIP_BYTE_ORDER_MARK = TRUE | FALSE
//	     ENCODING = '<string>' | UTF8
//	-- If TYPE = JSON | AVRO | ORC | PARQUET | XML
//	     ... (per-format options; see Reference URL for the full per-type lists)
func (v *Validator) ParseCreateFileFormat() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// formatTypeOptions: an order-independent KEY = value list. The CSV options
	// are modeled explicitly; other-format options are accepted via the generic
	// fallthrough (any identifier KEY = value).
	formatOption := func() bool {
		return v.Choice(
			v.option("COMPRESSION", v.parseScalar),
			v.option("RECORD_DELIMITER", v.parseScalar),
			v.option("FIELD_DELIMITER", v.parseScalar),
			v.option("MULTI_LINE", v.parseBool),
			v.option("FILE_EXTENSION", v.parseString),
			v.option("PARSE_HEADER", v.parseBool),
			v.option("SKIP_HEADER", num),
			v.option("SKIP_BLANK_LINES", v.parseBool),
			v.option("DATE_FORMAT", v.parseScalar),
			v.option("TIME_FORMAT", v.parseScalar),
			v.option("TIMESTAMP_FORMAT", v.parseScalar),
			v.option("BINARY_FORMAT", v.parseScalar),
			v.option("ESCAPE", v.parseScalar),
			v.option("ESCAPE_UNENCLOSED_FIELD", v.parseScalar),
			v.option("TRIM_SPACE", v.parseBool),
			v.option("FIELD_OPTIONALLY_ENCLOSED_BY", v.parseScalar),
			v.option("NULL_IF", func() bool { return v.parseParenList(v.parseString) }),
			v.option("ERROR_ON_COLUMN_COUNT_MISMATCH", v.parseBool),
			v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
			v.option("EMPTY_FIELD_AS_NULL", v.parseBool),
			v.option("SKIP_BYTE_ORDER_MARK", v.parseBool),
			v.option("ENCODING", v.parseScalar),
			// Generic fallthrough for per-format options not enumerated above.
			func() bool { return v.option2(v.parseIdentPath, v.parseScalar)() },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool {
			return v.Optional(v.wordsValue("TEMP", "TEMPORARY", "VOLATILE"))
		},
		func() bool { return v.MatchWord("FILE") },
		func() bool { return v.MatchWord("FORMAT") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ TYPE = { ... } [ formatTypeOptions ] ] and [ COMMENT = '...' ],
		// in any order.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("TYPE", v.wordsValue("CSV", "JSON", "AVRO", "ORC", "PARQUET", "XML")),
					v.commentOption(),
					formatOption,
				)
			})
		},
	)
}

// ParseCreateFunction validates the Snowflake `CREATE FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function
//
// Syntax:
//
//	-- Java handler (in-line)
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] FUNCTION [ IF NOT EXISTS ] <name> (
//	    [ <arg_name> <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( <col_name> <col_data_type> [ , ... ] ) }
//	  [ [ NOT ] NULL ]
//	  LANGUAGE JAVA
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ RUNTIME_VERSION = <java_jdk_version> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ IMPORTS = ( '<stage_path...>' [ , ... ] ) ]
//	  [ PACKAGES = ( '<package_name_and_version>' [ , ... ] ) ]
//	  HANDLER = '<path_to_method>'
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <name_of_integration> [ , ... ] ) ]
//	  [ SECRETS = ('<secret_variable_name>' = <secret_name> [ , ... ] ) ]
//	  [ TARGET_PATH = '<stage_path_and_file_name_to_write>' ]
//	  AS '<function_definition>'
//
//	-- Other handlers: LANGUAGE JAVASCRIPT, LANGUAGE PYTHON ([ AGGREGATE ],
//	-- RUNTIME_VERSION, ARTIFACT_REPOSITORY), LANGUAGE SCALA, and SQL.
//
//	-- SQL handler
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] FUNCTION <name> ( ... )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( <col_name> <col_data_type> [ , ... ] ) }
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ MEMOIZABLE ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS '<function_definition>'
//
//	-- CREATE OR ALTER FUNCTION ... is also supported.
func (v *Validator) ParseCreateFunction() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// RETURNS { <result_data_type> | TABLE ( ... ) }
	returnsType := func() bool {
		return v.Choice(
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("TABLE") },
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
			func() bool {
				return v.Sequence(
					v.parseIdentPath,
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
		)
	}
	parenIdentList := func() bool { return v.parseParenList(v.parseIdentPath) }
	parenStringList := func() bool { return v.parseParenList(v.parseString) }
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.Optional(v.wordsValue("TEMP", "TEMPORARY")) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURE") }) },
		func() bool { return v.MatchWord("FUNCTION") },
		v.ifNotExists,
		v.parseIdentPath,
		// argument list ( ... )
		v.consumeBalancedParens,
		// [ COPY GRANTS ]
		func() bool { return v.Optional(func() bool { return v.phrase("COPY", "GRANTS") }) },
		func() bool { return v.MatchKeyword("RETURNS") },
		returnsType,
		// [ [ NOT ] NULL ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("NOT") }) },
					func() bool { return v.MatchWord("NULL") },
				)
			})
		},
		// Order-independent body of property clauses (LANGUAGE, HANDLER, etc.).
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("LANGUAGE") },
							v.parseIdentPath,
						)
					},
					func() bool { return v.phrase("CALLED", "ON", "NULL", "INPUT") },
					func() bool { return v.phrase("RETURNS", "NULL", "ON", "NULL", "INPUT") },
					func() bool { return v.MatchWord("STRICT") },
					v.wordsValue("VOLATILE", "IMMUTABLE"),
					func() bool { return v.MatchWord("MEMOIZABLE") },
					func() bool { return v.MatchWord("AGGREGATE") },
					v.option("RUNTIME_VERSION", v.parseScalar),
					v.option("ARTIFACT_REPOSITORY", v.parseIdentPath),
					v.commentOption(),
					v.option("IMPORTS", parenStringList),
					v.option("PACKAGES", parenStringList),
					v.option("HANDLER", v.parseString),
					v.option("EXTERNAL_ACCESS_INTEGRATIONS", parenIdentList),
					v.option("SECRETS", v.consumeBalancedParens),
					v.option("TARGET_PATH", v.parseString),
					v.option("MAX_BATCH_ROWS", num),
				)
			})
		},
		// AS '<function_definition>' — accept a string literal or a permissive
		// balanced run / consume-to-EOF fallback for dollar-quoted-like bodies.
		func() bool {
			return v.Sequence(
				func() bool { return v.MatchKeyword("AS") },
				func() bool {
					return v.Choice(
						v.parseString,
						func() bool {
							// consume the remaining tokens as a free-form body
							if v.AtEnd() {
								return false
							}
							for !v.AtEnd() {
								v.advance()
							}
							return true
						},
					)
				},
			)
		},
	)
}

// ParseCreateFunctionSnowparkContainerServices validates the Snowflake `CREATE FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-function-spcs
//
// Syntax:
//
//	CREATE [ OR REPLACE ] FUNCTION <name> ( [ <arg_name> <arg_data_type> ] [ , ... ] )
//	  RETURNS <result_data_type>
//	  [ [ NOT ] NULL ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  SERVICE = <service_name>
//	  ENDPOINT = <endpoint_name>
//	  [ COMMENT = '<string_literal>' ]
//	  [ CONTEXT_HEADERS = ( <context_function_1> [ , <context_function_2> ...] ) ]
//	  [ MAX_BATCH_ROWS = <integer> ]
//	  [ MAX_BATCH_RETRIES = <integer> ]
//	  [ ON_BATCH_FAILURE = { ABORT | RETURN_NULL } ]
//	  [ BATCH_TIMEOUT_SECS = <integer> ]
//	  AS '<http_path_to_request_handler>'
//
//	CREATE [ OR ALTER ] FUNCTION ...
func (v *Validator) ParseCreateFunctionSnowparkContainerServices() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	dataType := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.Optional(v.consumeBalancedParens) },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("FUNCTION") },
		v.parseIdentPath,
		v.consumeBalancedParens,
		func() bool { return v.MatchKeyword("RETURNS") },
		dataType,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("NOT") }) },
					func() bool { return v.MatchWord("NULL") },
				)
			})
		},
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.phrase("CALLED", "ON", "NULL", "INPUT") },
					func() bool { return v.phrase("RETURNS", "NULL", "ON", "NULL", "INPUT") },
					func() bool { return v.MatchWord("STRICT") },
				)
			})
		},
		func() bool { return v.Optional(v.wordsValue("VOLATILE", "IMMUTABLE")) },
		// SERVICE and ENDPOINT are required; accept the whole set in any order.
		func() bool {
			return v.unorderedOnce(
				v.option("SERVICE", v.parseIdentPath),
				v.option("ENDPOINT", v.parseIdentPath),
				v.commentOption(),
				v.option("CONTEXT_HEADERS", v.consumeBalancedParens),
				v.option("MAX_BATCH_ROWS", num),
				v.option("MAX_BATCH_RETRIES", num),
				v.option("ON_BATCH_FAILURE", v.wordsValue("ABORT", "RETURN_NULL")),
				v.option("BATCH_TIMEOUT_SECS", num),
			)
		},
		func() bool { return v.MatchKeyword("AS") },
		v.parseString,
	)
}

// ParseCreateGateway validates the Snowflake `CREATE GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-gateway
//
// Syntax:
//
//	CREATE [ OR REPLACE ] GATEWAY [ IF NOT EXISTS ] <name>
//	  FROM SPECIFICATION <specification_text>
func (v *Validator) ParseCreateGateway() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("GATEWAY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.MatchWord("SPECIFICATION") },
		// <specification_text> — free-form; accept a string literal or a
		// permissive consume-to-EOF run.
		func() bool {
			return v.Choice(
				v.parseString,
				func() bool {
					if v.AtEnd() {
						return false
					}
					for !v.AtEnd() {
						v.advance()
					}
					return true
				},
			)
		},
	)
}

// ParseCreateGitRepository validates the Snowflake `CREATE GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-git-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] GIT REPOSITORY [ IF NOT EXISTS ] <name>
//	  ORIGIN = '<repository_url>'
//	  API_INTEGRATION = <integration_name>
//	  [ GIT_CREDENTIALS = <secret_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateGitRepository() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("GIT") },
		func() bool { return v.MatchWord("REPOSITORY") },
		v.ifNotExists,
		v.parseIdentPath,
		// ORIGIN and API_INTEGRATION are required; accept the whole set in any order.
		func() bool {
			return v.unorderedOnce(
				v.option("ORIGIN", v.parseString),
				v.option("API_INTEGRATION", v.parseIdentPath),
				v.option("GIT_CREDENTIALS", v.parseIdentPath),
				v.commentOption(),
				v.tagClause,
			)
		},
	)
}

// ParseCreateHybridTable validates the Snowflake `CREATE HYBRID TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-hybrid-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] HYBRID TABLE [ IF NOT EXISTS ] <table_name>
//	  ( <col_name> <col_type>
//	    [
//	      {
//	        DEFAULT <expr>
//	        | { AUTOINCREMENT | IDENTITY }
//	          [ { ( <start_num> , <step_num> ) | START <num> INCREMENT <num> } ]
//	          [ { ORDER | NOORDER } ]
//	      }
//	    ]
//	    [ NOT NULL ]
//	    [ inlineConstraint ]
//	    [ COLLATE '<collation_specification>' ]
//	    [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ ... ] ]
//	    [ , outoflineConstraint ]
//	    [ , outoflineIndex ]
//	    [ , ... ]
//	  )
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	inlineConstraint ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY | { [ FOREIGN KEY ] REFERENCES <ref_table_name> [ ( <ref_col_name> ) ] } }
//	  [ <constraint_properties> ]
//
//	outoflineConstraint ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE [ ( <col_name> [ , <col_name> , ... ] ) ]
//	    | PRIMARY KEY [ ( <col_name> [ , <col_name> , ... ] ) ]
//	    | [ FOREIGN KEY ] [ ( <col_name> [ , <col_name> , ... ] ) ]
//	      REFERENCES <ref_table_name> [ ( <ref_col_name> [ , <ref_col_name> , ... ] ) ]
//	  }
//	  [ <constraint_properties> ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineIndex ::=
//	  INDEX <index_name> ( <col_name> [ , <col_name> , ... ] )
//	    [ INCLUDE ( <col_name> [ , <col_name> , ... ] ) ]
func (v *Validator) ParseCreateHybridTable() bool {
	// The column/constraint/index list is free-form; consume it as a balanced
	// paren span.
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("HYBRID") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// ( column / constraint / index definitions )
		v.consumeBalancedParens,
		// [ COMMENT = '<string_literal>' ]
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateIcebergTable validates the Snowflake `CREATE ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name> (
//	    -- Column definition
//	    <col_name> <col_type> [ DEFAULT <col_default> ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ DEFAULT <col_default> ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ PARTITION BY ( partitionExpression [, ...] ) ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = 'SNOWFLAKE' ]
//	  [ BASE_LOCATION = '<directory_for_table_files>' ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ CATALOG_SYNC = '<open_catalog_integration_name>']
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ICEBERG_VERSION = <integer> ]
//	  [ ... ]
//
//	-- Additional forms: CTAS (AS SELECT), LIKE, and create-from-catalog/files
//	-- variants (CATALOG_TABLE_NAME / METADATA_FILE_PATH / BASE_LOCATION).
func (v *Validator) ParseCreateIcebergTable() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// The order-independent body of Iceberg-table options.
	optionList := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				// PARTITION BY ( ... )
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("PARTITION") },
						func() bool { return v.MatchKeyword("BY") },
						v.consumeBalancedParens,
					)
				},
				// CLUSTER BY ( ... )
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("CLUSTER") },
						func() bool { return v.MatchKeyword("BY") },
						v.consumeBalancedParens,
					)
				},
				v.option("PATH_LAYOUT", v.wordsValue("FLAT", "HIERARCHICAL")),
				v.option("EXTERNAL_VOLUME", v.parseScalar),
				v.option("CATALOG", v.parseScalar),
				v.option("CATALOG_TABLE_NAME", v.parseString),
				v.option("CATALOG_NAMESPACE", v.parseString),
				v.option("METADATA_FILE_PATH", v.parseString),
				v.option("BASE_LOCATION", v.parseString),
				v.option("TARGET_FILE_SIZE", v.parseString),
				v.option("CATALOG_SYNC", v.parseString),
				v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
				v.option("DATA_RETENTION_TIME_IN_DAYS", num),
				v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
				v.option("CHANGE_TRACKING", v.parseBool),
				v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
				v.option("AUTO_REFRESH", v.parseBool),
				func() bool { return v.phrase("COPY", "GRANTS") },
				func() bool { return v.phrase("COPY", "TAGS") },
				v.option("ERROR_LOGGING", v.parseBool),
				v.commentOption(),
				v.option("ICEBERG_VERSION", num),
				v.option("ICEBERG_MERGE_ON_READ_BEHAVIOR", v.parseString),
				v.option("ENABLE_ICEBERG_MERGE_ON_READ", v.parseBool),
				v.tagClause,
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("WITH") },
						func() bool { return v.MatchWord("CONTACT") },
						v.consumeBalancedParens,
					)
				},
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("TRANSIENT") }) },
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// GET_DDL emits CLUSTER BY between the table name and the column list;
		// accept it there as well as inside the option list (issue #776).
		func() bool { return v.Optional(v.clusterByClause(v.consumeBalancedParens)) },
		// One of: column list + options, LIKE <src>, AS <query>, or just options.
		func() bool {
			return v.Choice(
				// LIKE <source_table>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("LIKE") },
						v.parseIdentPath,
						optionList,
					)
				},
				// ( column definitions ) [ options ] [ AS <query> ]
				func() bool {
					return v.Sequence(
						v.consumeBalancedParens,
						optionList,
						func() bool {
							return v.Optional(func() bool {
								return v.Sequence(
									func() bool { return v.MatchKeyword("AS") },
									func() bool {
										if v.AtEnd() {
											return false
										}
										for !v.AtEnd() {
											v.advance()
										}
										return true
									},
								)
							})
						},
					)
				},
				// options only [ AS <query> ]
				func() bool {
					return v.Sequence(
						optionList,
						func() bool {
							return v.Optional(func() bool {
								return v.Sequence(
									func() bool { return v.MatchKeyword("AS") },
									func() bool {
										if v.AtEnd() {
											return false
										}
										for !v.AtEnd() {
											v.advance()
										}
										return true
									},
								)
							})
						},
					)
				},
			)
		},
	)
}

// ParseCreateIcebergTableAwsGlueCatalog validates the Snowflake `CREATE ICEBERG TABLE (AWS Glue catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-aws-glue
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  CATALOG_TABLE_NAME = '<catalog_table_name>'
//	  [ CATALOG_NAMESPACE = '<catalog_namespace>' ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableAwsGlueCatalog() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// CATALOG_TABLE_NAME is required; accept the whole set order-independently.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("EXTERNAL_VOLUME", v.parseString),
					v.option("CATALOG", v.parseString),
					v.option("CATALOG_TABLE_NAME", v.parseString),
					v.option("CATALOG_NAMESPACE", v.parseString),
					v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
					v.option("AUTO_REFRESH", v.parseBool),
					v.commentOption(),
					v.tagClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("WITH") },
							func() bool { return v.MatchWord("CONTACT") },
							v.consumeBalancedParens,
						)
					},
				)
			})
		},
	)
}

// ParseCreateIcebergTableDeltaFiles validates the Snowflake `CREATE ICEBERG TABLE (Delta files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-delta
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  BASE_LOCATION = '<relative_path_from_external_volume>'
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableDeltaFiles() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// BASE_LOCATION is required; accept the whole set order-independently.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("EXTERNAL_VOLUME", v.parseString),
					v.option("CATALOG", v.parseString),
					v.option("BASE_LOCATION", v.parseString),
					v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
					v.option("AUTO_REFRESH", v.parseBool),
					v.commentOption(),
					v.tagClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("WITH") },
							func() bool { return v.MatchWord("CONTACT") },
							v.consumeBalancedParens,
						)
					},
				)
			})
		},
	)
}

// ParseCreateIcebergTableIcebergFiles validates the Snowflake `CREATE ICEBERG TABLE (Iceberg files)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-iceberg-files
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  METADATA_FILE_PATH = '<metadata_file_path>'
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableIcebergFiles() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// METADATA_FILE_PATH is required; accept the whole set order-independently.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("EXTERNAL_VOLUME", v.parseString),
					v.option("CATALOG", v.parseString),
					v.option("METADATA_FILE_PATH", v.parseString),
					v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
					v.commentOption(),
					v.tagClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("WITH") },
							func() bool { return v.MatchWord("CONTACT") },
							v.consumeBalancedParens,
						)
					},
				)
			})
		},
	)
}

// ParseCreateIcebergTableIcebergRestCatalog validates the Snowflake `CREATE ICEBERG TABLE (Iceberg REST catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-rest
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name>
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = '<catalog_integration_name>' ]
//	  CATALOG_TABLE_NAME = '<rest_catalog_table_name>'
//	  [ CATALOG_NAMESPACE = '<catalog_namespace>' ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
func (v *Validator) ParseCreateIcebergTableIcebergRestCatalog() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// CATALOG_TABLE_NAME is required; accept the whole set order-independently.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("EXTERNAL_VOLUME", v.parseString),
					v.option("CATALOG", v.parseString),
					v.option("CATALOG_TABLE_NAME", v.parseString),
					v.option("CATALOG_NAMESPACE", v.parseString),
					v.option("PATH_LAYOUT", v.wordsValue("FLAT", "HIERARCHICAL")),
					v.option("TARGET_FILE_SIZE", v.parseString),
					v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
					v.option("AUTO_REFRESH", v.parseBool),
					v.commentOption(),
					v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
					v.option("ICEBERG_MERGE_ON_READ_BEHAVIOR", v.parseString),
					v.option("ENABLE_ICEBERG_MERGE_ON_READ", v.parseBool),
					v.tagClause,
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("WITH") },
							func() bool { return v.MatchWord("CONTACT") },
							v.consumeBalancedParens,
						)
					},
				)
			})
		},
	)
}

// ParseCreateIcebergTableSnowflakeCatalog validates the Snowflake `CREATE ICEBERG TABLE (Snowflake catalog)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-iceberg-table-snowflake
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] ICEBERG TABLE [ IF NOT EXISTS ] <table_name> (
//	    -- Column definition
//	    <col_name> <col_type> [ DEFAULT <col_default> ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ DEFAULT <col_default> ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ PARTITION BY ( partitionExpression [, ...] ) ]
//	  [ PATH_LAYOUT = { FLAT | HIERARCHICAL } ]
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ EXTERNAL_VOLUME = '<external_volume_name>' ]
//	  [ CATALOG = 'SNOWFLAKE' ]
//	  [ BASE_LOCATION = '<directory_for_table_files>' ]
//	  [ TARGET_FILE_SIZE = '{ AUTO | 16MB | 32MB | 64MB | 128MB }' ]
//	  [ CATALOG_SYNC = '<open_catalog_integration_name>']
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ ICEBERG_VERSION = <integer> ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
func (v *Validator) ParseCreateIcebergTableSnowflakeCatalog() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	phraseRule := func(words ...string) Rule { return func() bool { return v.phrase(words...) } }
	// The column-definition list is free-form; consume a balanced ( ... ) span.
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	parenSpan := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			consumeBalanced,
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	policyPolicy := func(words ...string) Rule {
		return func() bool {
			return v.Sequence(
				func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
				phraseRule(words...),
				v.parseIdentPath,
			)
		}
	}
	prop := func() bool {
		return v.Choice(
			func() bool {
				return v.Sequence(phraseRule("PARTITION", "BY"), parenSpan)
			},
			v.option("PATH_LAYOUT", v.wordsValue("FLAT", "HIERARCHICAL")),
			v.clusterByClause(parenSpan),
			v.option("EXTERNAL_VOLUME", v.parseString),
			v.option("CATALOG_SYNC", v.parseString),
			v.option("CATALOG", v.parseString),
			v.option("BASE_LOCATION", v.parseString),
			v.option("TARGET_FILE_SIZE", v.parseString),
			v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
			v.option("DATA_RETENTION_TIME_IN_DAYS", num),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
			v.option("CHANGE_TRACKING", v.parseBool),
			func() bool { return v.phrase("COPY", "GRANTS") },
			func() bool { return v.phrase("COPY", "TAGS") },
			v.option("ERROR_LOGGING", v.parseBool),
			v.option("ICEBERG_VERSION", num),
			v.option("ICEBERG_MERGE_ON_READ_BEHAVIOR", v.parseString),
			v.option("ENABLE_ICEBERG_MERGE_ON_READ", v.parseBool),
			v.option("ENABLE_DATA_COMPACTION", v.parseBool),
			func() bool {
				return v.Sequence(policyPolicy("ROW", "ACCESS", "POLICY"),
					func() bool { return v.MatchWord("ON") }, parenSpan)
			},
			policyPolicy("AGGREGATION", "POLICY"),
			v.tagClause,
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("CONTACT") },
					parenSpan,
				)
			},
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("TRANSIENT") }) },
		func() bool { return v.MatchWord("ICEBERG") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// GET_DDL emits CLUSTER BY before the column list; accept it there as
		// well as inside the trailing property list (issue #776).
		func() bool { return v.Optional(v.clusterByClause(parenSpan)) },
		parenSpan,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateImageRepository validates the Snowflake `CREATE IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-image-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] IMAGE REPOSITORY [ IF NOT EXISTS ] <name>
//	  [ ENCRYPTION = ( TYPE = 'SNOWFLAKE_FULL' | TYPE = 'SNOWFLAKE_SSE' ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateImageRepository() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("IMAGE") },
		func() bool { return v.MatchWord("REPOSITORY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENCRYPTION", func() bool {
					return v.parseParenList(v.option("TYPE", v.parseString))
				}),
				v.commentOption(),
				v.tagClause,
			)
		},
	)
}

// ParseCreateIndex validates the Snowflake `CREATE INDEX` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-index
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INDEX [ IF NOT EXISTS ] <index_name>
//	  ON <table_name>
//	    ( <col_name> [ , <col_name> , ... ] )
//	    [ INCLUDE ( <col_name> [ , <col_name> , ... ] ) ]
func (v *Validator) ParseCreateIndex() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("INDEX") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchWord("ON") },
		v.parseIdentPath,
		func() bool { return v.parseParenList(v.parseIdentPath) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("INCLUDE") },
					func() bool { return v.parseParenList(v.parseIdentPath) },
				)
			})
		},
	)
}

// ParseCreateIntegration validates the Snowflake `CREATE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] <integration_type> INTEGRATION [ IF NOT EXISTS ] <object_name>
//	  [ <integration_type_params> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateIntegration() bool {
	// CREATE [ OR REPLACE ] <integration_type> INTEGRATION [ IF NOT EXISTS ]
	// <name> <permissive parameter run>. The integration_type words and the
	// type-specific parameter block are too varied to model strictly, so consume
	// any identifier-like words before INTEGRATION and a balanced run afterward.
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		// at least one type word, then the INTEGRATION keyword. Consume any
		// leading words (e.g. API, SECURITY, STORAGE, NOTIFICATION) up to it.
		func() bool {
			// a type word is any ident-like token that is NOT "INTEGRATION".
			typeWord := func() bool {
				saved := v.save()
				if v.MatchWord("INTEGRATION") {
					v.restore(saved)
					return false
				}
				t := v.Peek()
				if t.Kind.IsIdentLike() {
					v.advance()
					return true
				}
				v.expect("integration type")
				return false
			}
			return v.Sequence(typeWord, func() bool { return v.ZeroOrMore(typeWord) })
		},
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		consumeBalanced,
	)
}

// ParseCreateInteractiveTable validates the Snowflake `CREATE INTERACTIVE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INTERACTIVE TABLE [ IF NOT EXISTS ] <table_name>
//	  (
//	    <col_name> <col_type>
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ , <col_name> <col_type> [ ... ] ]
//	  )
//	  CLUSTER BY ( <expr> [ , <expr> , ... ] )
//	  [ TARGET_LAG = '<num> { seconds | minutes | hours | days }' ]
//	  [ WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  AS <query>
func (v *Validator) ParseCreateInteractiveTable() bool {
	phraseRule := func(words ...string) Rule { return func() bool { return v.phrase(words...) } }
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	parenSpan := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			consumeBalanced,
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// AS <query>: consume everything to EOF.
	consumeRest := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			v.advance()
			return true
		})
	}
	prop := func() bool {
		return v.Choice(
			v.option("TARGET_LAG", v.parseString),
			v.option("WAREHOUSE", v.parseIdentPath),
			v.commentOption(),
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("ROW", "ACCESS", "POLICY"),
					v.parseIdentPath,
					func() bool { return v.MatchWord("ON") },
					parenSpan,
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("AGGREGATION", "POLICY"),
					v.parseIdentPath,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(phraseRule("ENTITY", "KEY"), parenSpan)
						})
					},
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("JOIN", "POLICY"),
					v.parseIdentPath,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(phraseRule("ALLOWED", "JOIN", "KEYS"), parenSpan)
						})
					},
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("STORAGE", "LIFECYCLE", "POLICY"),
					v.parseIdentPath,
					func() bool { return v.MatchWord("ON") },
					parenSpan,
				)
			},
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("INTERACTIVE") },
		func() bool { return v.MatchWord("TABLE") },
		v.ifNotExists,
		v.parseIdentPath,
		// CLUSTER BY is required for interactive tables; the docs place it after
		// the column list, but GET_DDL emits it before. Require it in exactly one
		// of the two positions (issue #776).
		func() bool {
			return v.Choice(
				func() bool { return v.Sequence(v.clusterByClause(parenSpan), parenSpan) },
				func() bool { return v.Sequence(parenSpan, v.clusterByClause(parenSpan)) },
			)
		},
		func() bool { return v.ZeroOrMore(prop) },
		func() bool { return v.MatchKeyword("AS") },
		consumeRest,
	)
}

// ParseCreateInteractiveWarehouse validates the Snowflake `CREATE INTERACTIVE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-interactive-warehouse
//
// Syntax:
//
//	CREATE [ OR REPLACE ] INTERACTIVE WAREHOUSE [ IF NOT EXISTS ] <name>
//	       [ TABLES ( <table_name> [ , <table_name> ... ] ) ]
//	       [ [ WITH ] objectProperties ]
//	       [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	       [ objectParams ]
//
//	Where:
//
//	objectProperties ::=
//	  WAREHOUSE_SIZE = { XSMALL | SMALL | MEDIUM | LARGE | XLARGE | XXLARGE | XXXLARGE | X4LARGE | X5LARGE | X6LARGE }
//	  MAX_CLUSTER_COUNT = <num>
//	  MIN_CLUSTER_COUNT = <num>
//	  AUTO_SUSPEND = { <num> | NULL }
//	  AUTO_RESUME = { TRUE | FALSE }
//	  INITIALLY_SUSPENDED = { TRUE | FALSE }
//	  RESOURCE_MONITOR = <monitor_name>
//	  COMMENT = '<string_literal>'
//
//	objectParams ::=
//	  MAX_CONCURRENCY_LEVEL = <num>
//	  STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = <num>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
func (v *Validator) ParseCreateInteractiveWarehouse() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	prop := func() bool {
		return v.Choice(
			v.option("WAREHOUSE_SIZE", v.parseScalar),
			v.option("MAX_CLUSTER_COUNT", num),
			v.option("MIN_CLUSTER_COUNT", num),
			v.option("AUTO_SUSPEND", func() bool { return v.Choice(num, func() bool { return v.MatchWord("NULL") }) }),
			v.option("AUTO_RESUME", v.parseBool),
			v.option("INITIALLY_SUSPENDED", v.parseBool),
			v.option("RESOURCE_MONITOR", v.parseIdentPath),
			v.option("MAX_CONCURRENCY_LEVEL", num),
			v.option("STATEMENT_QUEUED_TIMEOUT_IN_SECONDS", num),
			v.option("STATEMENT_TIMEOUT_IN_SECONDS", num),
			v.commentOption(),
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("INTERACTIVE") },
		func() bool { return v.MatchWord("WAREHOUSE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("TABLES") },
					func() bool { return v.parseParenList(v.parseIdentPath) },
				)
			})
		},
		func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateJoinPolicy validates the Snowflake `CREATE JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-join-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] JOIN POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS JOIN_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateJoinPolicy() bool {
	// The policy body after `->` is a free-form expression; consume it permissively
	// up to an optional trailing COMMENT = '...' or EOF.
	consumeBody := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			// stop at a trailing COMMENT option
			saved := v.save()
			if v.MatchWord("COMMENT") && v.MatchOp("=") {
				v.restore(saved)
				return false
			}
			v.restore(saved)
			v.advance()
			return true
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("JOIN") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.Match(sqltok.RParen) },
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("JOIN_CONSTRAINT") },
		// `->` tokenizes as two operators "-" then ">".
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		consumeBody,
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateListing validates the Snowflake `CREATE LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-listing
//
// Syntax:
//
//	CREATE EXTERNAL LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  AS '<yaml_manifest_string>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//	  [ REVIEW = { TRUE | FALSE } ]
//	  [ COMMENT = '<string>' ]
//
//	CREATE EXTERNAL LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  FROM '<yaml_manifest_stage_location>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//	  [ REVIEW = { TRUE | FALSE } ]
func (v *Validator) ParseCreateListing() bool {
	option := func() bool {
		return v.Choice(
			v.option("PUBLISH", v.parseBool),
			v.option("REVIEW", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("EXTERNAL") },
		func() bool { return v.MatchWord("LISTING") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ { SHARE <share> | APPLICATION PACKAGE <package> } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(func() bool { return v.MatchKeyword("SHARE") }, v.parseIdentPath)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("APPLICATION") },
							func() bool { return v.MatchWord("PACKAGE") },
							v.parseIdentPath,
						)
					},
				)
			})
		},
		// { AS '<manifest>' | FROM '<stage_location>' }
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(func() bool { return v.MatchKeyword("AS") }, v.parseString)
				},
				func() bool {
					return v.Sequence(func() bool { return v.MatchKeyword("FROM") }, v.parseString)
				},
			)
		},
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateMaintenancePolicy validates the Snowflake `CREATE MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-maintenance-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MAINTENANCE POLICY [ IF NOT EXISTS ] <name>
//	  SCHEDULE = 'USING CRON <cron_spec> <timezone>'
//	  [ COMMENT = '<comment>' ]
func (v *Validator) ParseCreateMaintenancePolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("MAINTENANCE") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		v.option("SCHEDULE", v.parseString),
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateManagedAccount validates the Snowflake `CREATE MANAGED ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-managed-account
//
// Syntax:
//
//	CREATE MANAGED ACCOUNT <name>
//	    ADMIN_NAME = <username> , ADMIN_PASSWORD = <user_password> ,
//	    TYPE = READER ,
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateManagedAccount() bool {
	// Params are comma-separated and order may vary; accept the documented set.
	param := func() bool {
		return v.Choice(
			v.option("ADMIN_NAME", v.parseScalar),
			v.option("ADMIN_PASSWORD", v.parseScalar),
			v.option("TYPE", func() bool { return v.MatchWord("READER") }),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("MANAGED") },
		func() bool { return v.MatchWord("ACCOUNT") },
		v.parseIdentPath,
		param,
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.Match(sqltok.Comma) }) },
					param,
				)
			})
		},
	)
}

// ParseCreateMaskingPolicy validates the Snowflake `CREATE MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-masking-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MASKING POLICY [ IF NOT EXISTS ] <name> AS
//	( <arg_name_to_mask> <arg_type_to_mask> [ , <arg_1> <arg_type_1> ... ] )
//	RETURNS <arg_type_to_mask> -> <body>
//	[ COMMENT = '<string_literal>' ]
//	[ EXEMPT_OTHER_POLICIES = { TRUE | FALSE } ]
//
//	CREATE OR ALTER MASKING POLICY <name> AS
//	( <arg_name_to_mask> <arg_type_to_mask> [ , <arg_1> <arg_type_1> ... ] )
//	RETURNS <arg_type_to_mask> -> <body>
//	[ COMMENT = '<string_literal>' ]
//	[ EXEMPT_OTHER_POLICIES = { TRUE | FALSE } ]
func (v *Validator) ParseCreateMaskingPolicy() bool {
	// arg list: ( <name> <type> [ , <name> <type> ... ] ) — consume balanced parens.
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	parenSpan := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			consumeBalanced,
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// body after `->`: free-form expression up to trailing COMMENT/EXEMPT or EOF.
	consumeBody := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			saved := v.save()
			stop := (v.MatchWord("COMMENT") || v.MatchWord("EXEMPT_OTHER_POLICIES")) && v.MatchOp("=")
			v.restore(saved)
			if stop {
				return false
			}
			v.advance()
			return true
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// OR REPLACE | OR ALTER
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("MASKING") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		parenSpan,
		func() bool { return v.MatchWord("RETURNS") },
		v.parseIdentPath,
		// `->` tokenizes as two operators "-" then ">".
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		consumeBody,
		func() bool {
			return v.unorderedOnce(
				v.commentOption(),
				v.option("EXEMPT_OTHER_POLICIES", v.parseBool),
			)
		},
	)
}

// ParseCreateMaterializedView validates the Snowflake `CREATE MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-materialized-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] [ INTERACTIVE ] MATERIALIZED VIEW [ IF NOT EXISTS ] <name>
//	  [ COPY GRANTS ]
//	  ( <column_list> )
//	  [ <col1> [ WITH ] MASKING POLICY <policy_name> [ USING ( <col1> , <cond_col1> , ... ) ]
//	           [ WITH ] PROJECTION POLICY <policy_name>
//	           [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ , <col2> [ ... ] ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ CLUSTER BY ( <expr1> [, <expr2> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  AS <select_statement>
func (v *Validator) ParseCreateMaterializedView() bool {
	phraseRule := func(words ...string) Rule { return func() bool { return v.phrase(words...) } }
	var consumeBalanced func() bool
	consumeBalanced = func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.Match(sqltok.LParen) },
						consumeBalanced,
						func() bool { return v.Match(sqltok.RParen) },
					)
				},
				func() bool {
					if v.Peek().Kind == sqltok.RParen || v.AtEnd() {
						return false
					}
					v.advance()
					return true
				},
			)
		})
	}
	parenSpan := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			consumeBalanced,
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	consumeRest := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			v.advance()
			return true
		})
	}
	prop := func() bool {
		return v.Choice(
			v.commentOption(),
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("ROW", "ACCESS", "POLICY"),
					v.parseIdentPath,
					func() bool { return v.MatchWord("ON") },
					parenSpan,
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					phraseRule("AGGREGATION", "POLICY"),
					v.parseIdentPath,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(phraseRule("ENTITY", "KEY"), parenSpan)
						})
					},
				)
			},
			v.clusterByClause(parenSpan),
			v.tagClause,
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("CONTACT") },
					parenSpan,
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURE") }) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("INTERACTIVE") }) },
		func() bool { return v.MatchWord("MATERIALIZED") },
		func() bool { return v.MatchKeyword("VIEW") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(func() bool { return v.phrase("COPY", "GRANTS") }) },
		// GET_DDL emits CLUSTER BY before the column list; accept it there as well
		// as inside the trailing property list (issue #776).
		func() bool { return v.Optional(v.clusterByClause(parenSpan)) },
		// optional column list / per-column policy span
		func() bool { return v.Optional(parenSpan) },
		func() bool { return v.ZeroOrMore(prop) },
		func() bool { return v.MatchKeyword("AS") },
		consumeRest,
	)
}

// ParseCreateMcpServer validates the Snowflake `CREATE MCP SERVER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-mcp-server
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MCP SERVER [ IF NOT EXISTS ] <name>
//	  FROM SPECIFICATION $$<specification_yaml>$$
func (v *Validator) ParseCreateMcpServer() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("MCP") },
		func() bool { return v.MatchWord("SERVER") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.MatchWord("SPECIFICATION") },
		// $$<specification_yaml>$$ — a dollar-quoted literal (or a plain string).
		func() bool {
			return v.Choice(
				func() bool { return v.Match(sqltok.DollarQuoted) },
				func() bool { return v.Match(sqltok.StringLit) },
			)
		},
	)
}

// ParseCreateModel validates the Snowflake `CREATE MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MODEL [ IF NOT EXISTS ] <name> [ WITH VERSION <version_name> ]
//	    FROM MODEL <source_model_name> [ VERSION <source_version_or_alias_name> ]
//
//	CREATE [ OR REPLACE ] MODEL [ IF NOT EXISTS ] <name> [ WITH VERSION <version_name> ]
//	  FROM internalStage
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	  | @[<namespace>.]%<table_name>[/<path>]
//	  | @~[/<path>]
func (v *Validator) ParseCreateModel() bool {
	// stage reference: @[ns.]name[/path] or @~[/path] or @[ns.]%table[/path].
	// Consume the @ then a permissive run of path tokens (ident/%/dot/operators).
	stageRef := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.At) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					if v.AtEnd() {
						return false
					}
					t := v.Peek()
					switch t.Kind {
					case sqltok.Identifier, sqltok.QuotedIdent, sqltok.Keyword,
						sqltok.Dot, sqltok.Operator, sqltok.NumberLit:
						v.advance()
						return true
					}
					return false
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("MODEL") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ WITH VERSION <version_name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("VERSION") },
					v.parseIdentPath,
				)
			})
		},
		func() bool { return v.MatchKeyword("FROM") },
		// { MODEL <source> [ VERSION <v> ] | <internalStage> }
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("MODEL") },
						v.parseIdentPath,
						func() bool {
							return v.Optional(func() bool {
								return v.Sequence(
									func() bool { return v.MatchWord("VERSION") },
									v.parseIdentPath,
								)
							})
						},
					)
				},
				stageRef,
			)
		},
	)
}

// ParseCreateModelMonitor validates the Snowflake `CREATE MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-model-monitor
//
// Syntax:
//
//	CREATE [ OR REPLACE ] MODEL MONITOR [ IF NOT EXISTS ] <monitor_name> WITH
//	    MODEL = <model_name>
//	    VERSION = '<version_name>'
//	    FUNCTION = '<function_name>'
//	    SOURCE = <source_name>
//	    WAREHOUSE = <warehouse_name>
//	    REFRESH_INTERVAL = '<num> { seconds | minutes | hours | days }'
//	    AGGREGATION_WINDOW = '<num> days'
//	    TIMESTAMP_COLUMN = <timestamp_name>
//	    [ BASELINE = <baseline_name> ]
//	    [ ID_COLUMNS = <id_column_name_array> ]
//	    [ PREDICTION_CLASS_COLUMNS = <prediction_class_column_name_array> ]
//	    [ PREDICTION_SCORE_COLUMNS = <prediction_column-name_array> ]
//	    [ ACTUAL_CLASS_COLUMNS = <actual_class_column_name_array> ]
//	    [ ACTUAL_SCORE_COLUMNS = <actual_column_name_array> ]
//	    [ SEGMENT_COLUMNS = <segment_column_name_array> ]
//	    [ CUSTOM_METRIC_COLUMNS = <custom_metric_column_name_array> ]
func (v *Validator) ParseCreateModelMonitor() bool {
	// Array RHS like [c1, c2] or a parenthesized/identifier value — accept a
	// bracketed span, a parenthesized span, a string, or an ident path.
	consumeBalanced := func(open, closeK sqltok.TokenKind) func() bool {
		return func() bool {
			return v.Sequence(
				func() bool { return v.Match(open) },
				func() bool {
					return v.ZeroOrMore(func() bool {
						if v.Peek().Kind == closeK || v.AtEnd() {
							return false
						}
						v.advance()
						return true
					})
				},
				func() bool { return v.Match(closeK) },
			)
		}
	}
	value := func() bool {
		return v.Choice(
			consumeBalanced(sqltok.LBracket, sqltok.RBracket),
			consumeBalanced(sqltok.LParen, sqltok.RParen),
			v.parseString,
			v.parseIdentPath,
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("MODEL", v.parseIdentPath),
			v.option("VERSION", v.parseString),
			v.option("FUNCTION", v.parseString),
			v.option("SOURCE", v.parseIdentPath),
			v.option("WAREHOUSE", v.parseIdentPath),
			v.option("REFRESH_INTERVAL", v.parseString),
			v.option("AGGREGATION_WINDOW", v.parseString),
			v.option("TIMESTAMP_COLUMN", v.parseIdentPath),
			v.option("BASELINE", v.parseIdentPath),
			v.option("ID_COLUMNS", value),
			v.option("PREDICTION_CLASS_COLUMNS", value),
			v.option("PREDICTION_SCORE_COLUMNS", value),
			v.option("ACTUAL_CLASS_COLUMNS", value),
			v.option("ACTUAL_SCORE_COLUMNS", value),
			v.option("SEGMENT_COLUMNS", value),
			v.option("CUSTOM_METRIC_COLUMNS", value),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.MatchWord("MONITOR") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchWord("WITH") },
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateNetworkPolicy validates the Snowflake `CREATE NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NETWORK POLICY [ IF NOT EXISTS ] <name>
//	  [ ALLOWED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	  [ BLOCKED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	  [ ALLOWED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' , ... ] ) ]
//	  [ BLOCKED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNetworkPolicy() bool {
	// list of strings, possibly empty: ( [ '<s>' ] [ , '<s>' ... ] )
	strList := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						v.parseString,
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseString)
							})
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("ALLOWED_NETWORK_RULE_LIST", strList),
			v.option("BLOCKED_NETWORK_RULE_LIST", strList),
			v.option("ALLOWED_IP_LIST", strList),
			v.option("BLOCKED_IP_LIST", strList),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NETWORK") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateNetworkRule validates the Snowflake `CREATE NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-network-rule
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NETWORK RULE <name>
//	   TYPE = { IPV4 | IPV6 | AWSVPCEID | AZURELINKID | GCPPSCID | HOST_PORT | PRIVATE_HOST_PORT | COMPUTE_POOL }
//	   VALUE_LIST = ( '<value>' [, '<value>', ... ] )
//	   MODE = { INGRESS | INTERNAL_STAGE | SNOWFLAKE_MANAGED_STORAGE_VOLUME | EGRESS }
//	   [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER NETWORK RULE <name>
//	   TYPE = { IPV4 | IPV6 | AWSVPCEID | AZURELINKID | GCPPSCID | HOST_PORT | PRIVATE_HOST_PORT | COMPUTE_POOL }
//	   VALUE_LIST = ( '<value>' [, '<value>', ... ] )
//	   MODE = { INGRESS | INTERNAL_STAGE | SNOWFLAKE_MANAGED_STORAGE_VOLUME | EGRESS }
//	   [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNetworkRule() bool {
	typeValue := v.wordsValue("IPV4", "IPV6", "AWSVPCEID", "AZURELINKID",
		"GCPPSCID", "HOST_PORT", "PRIVATE_HOST_PORT", "COMPUTE_POOL")
	modeValue := v.wordsValue("INGRESS", "INTERNAL_STAGE",
		"SNOWFLAKE_MANAGED_STORAGE_VOLUME", "EGRESS")
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR REPLACE ] | [ OR ALTER ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("NETWORK") },
		func() bool { return v.MatchWord("RULE") },
		v.parseIdentPath,
		// Order-independent options; TYPE/VALUE_LIST/MODE required, COMMENT optional.
		func() bool {
			return v.unorderedOnce(
				v.option("TYPE", typeValue),
				v.option("VALUE_LIST", func() bool { return v.parseParenList(v.parseString) }),
				v.option("MODE", modeValue),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotebook validates the Snowflake `CREATE NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTEBOOK [ IF NOT EXISTS ] <name>
//	  [ FROM '<source_location>' ]
//	  [ MAIN_FILE = '<main_file_name>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_to_run_nb_and_sql_queries_in> ]
//	  [ IDLE_AUTO_SHUTDOWN_TIME_SECONDS = <number_of_seconds> ]
//	  [ RUNTIME_NAME = '<runtime_name>' ]
//	  [ COMPUTE_POOL = '<compute_pool_name>' ]
//	  [ WAREHOUSE = <warehouse_to_run_notebook_python_runtime> ]
func (v *Validator) ParseCreateNotebook() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTEBOOK") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ FROM '<source_location>' ] then order-independent options.
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("FROM") },
					v.parseString,
				)
			})
		},
		func() bool {
			return v.unorderedOnce(
				v.option("MAIN_FILE", v.parseString),
				v.commentOption(),
				v.option("QUERY_WAREHOUSE", v.parseIdentPath),
				v.option("IDLE_AUTO_SHUTDOWN_TIME_SECONDS", v.parseNumber),
				v.option("RUNTIME_NAME", v.parseString),
				v.option("COMPUTE_POOL", v.parseScalar),
				v.option("WAREHOUSE", v.parseIdentPath),
			)
		},
	)
}

// ParseCreateNotebookProject validates the Snowflake `CREATE NOTEBOOK PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notebook-project
//
// Syntax:
//
//	CREATE NOTEBOOK PROJECT <database_name>.<schema_name>.<project_name>
//	  FROM 'snow://workspace/<workspace_path>'
//	  [ COMMENT = '<string_literal>' ];
//
//	CREATE NOTEBOOK PROJECT [ IF NOT EXISTS ] <database_name>.<schema_name>.<project_name>
//	  FROM '@<database_name>.<schema_name>.<stage_name>'
//	  [ COMMENT = '<string_literal>' ];
func (v *Validator) ParseCreateNotebookProject() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("NOTEBOOK") },
		func() bool { return v.MatchWord("PROJECT") },
		v.ifNotExists,
		v.parseIdentPath,
		// FROM '<location>' (workspace snow:// URI or @stage path — both string literals)
		func() bool { return v.MatchKeyword("FROM") },
		v.parseString,
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateNotificationIntegration validates the Snowflake `CREATE NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateNotificationIntegration() bool {
	// Lenient dispatcher: require the CREATE NOTIFICATION INTEGRATION skeleton +
	// name, then accept a permissive run of `KEY = value` options (any of the
	// type-specific variants), plus paren-lists, COMMENT and TAG.
	anyOption := func() bool {
		return v.Choice(
			v.option2(v.parseIdentPath, func() bool {
				return v.Choice(
					func() bool { return v.parseParenList(v.parseString) },
					v.parseScalar,
				)
			}),
			v.commentOption(),
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(anyOption) },
	)
}

// ParseCreateNotificationIntegrationEmail validates the Snowflake `CREATE NOTIFICATION INTEGRATION (email)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-email
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = EMAIL
//	  ENABLED = { TRUE | FALSE }
//	  [ ALLOWED_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_SUBJECT = '<subject_line>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationEmail() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("TYPE", v.wordsValue("EMAIL")),
				v.option("ENABLED", v.parseBool),
				v.option("ALLOWED_RECIPIENTS", func() bool { return v.parseParenList(v.parseString) }),
				v.option("DEFAULT_RECIPIENTS", func() bool { return v.parseParenList(v.parseString) }),
				v.option("DEFAULT_SUBJECT", v.parseString),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationInboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-azure
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE
//	  AZURE_STORAGE_QUEUE_PRIMARY_URI = '<queue_url>'
//	  AZURE_TENANT_ID = '<ad_directory_id>';
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationInboundAzureEventGrid() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENABLED", v.parseBool),
				v.option("TYPE", v.wordsValue("QUEUE")),
				v.option("NOTIFICATION_PROVIDER", v.wordsValue("AZURE_STORAGE_QUEUE")),
				v.option("AZURE_STORAGE_QUEUE_PRIMARY_URI", v.parseString),
				v.option("AZURE_TENANT_ID", v.parseString),
				v.option("USE_PRIVATELINK_ENDPOINT", v.parseBool),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationInboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (inbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-inbound-gcp
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  NOTIFICATION_PROVIDER = GCP_PUBSUB
//	  GCP_PUBSUB_SUBSCRIPTION_NAME = '<subscription_id>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationInboundGooglePubSub() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENABLED", v.parseBool),
				v.option("TYPE", v.wordsValue("QUEUE")),
				v.option("NOTIFICATION_PROVIDER", v.wordsValue("GCP_PUBSUB")),
				v.option("GCP_PUBSUB_SUBSCRIPTION_NAME", v.parseString),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationOutboundAmazonSns validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Amazon SNS)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-aws
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = AWS_SNS
//	  AWS_SNS_TOPIC_ARN = '<topic_arn>'
//	  AWS_SNS_ROLE_ARN = '<iam_role_arn>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundAmazonSns() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENABLED", v.parseBool),
				v.option("TYPE", v.wordsValue("QUEUE")),
				v.option("DIRECTION", v.wordsValue("OUTBOUND")),
				v.option("NOTIFICATION_PROVIDER", v.wordsValue("AWS_SNS")),
				v.option("AWS_SNS_TOPIC_ARN", v.parseString),
				v.option("AWS_SNS_ROLE_ARN", v.parseString),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationOutboundAzureEventGrid validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-azure
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = AZURE_EVENT_GRID
//	  AZURE_EVENT_GRID_TOPIC_ENDPOINT = '<event_grid_topic_endpoint>'
//	  AZURE_TENANT_ID = '<ad_directory_id>';
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundAzureEventGrid() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENABLED", v.parseBool),
				v.option("TYPE", v.wordsValue("QUEUE")),
				v.option("DIRECTION", v.wordsValue("OUTBOUND")),
				v.option("NOTIFICATION_PROVIDER", v.wordsValue("AZURE_EVENT_GRID")),
				v.option("AZURE_EVENT_GRID_TOPIC_ENDPOINT", v.parseString),
				v.option("AZURE_TENANT_ID", v.parseString),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationOutboundGooglePubSub validates the Snowflake `CREATE NOTIFICATION INTEGRATION (outbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-queue-outbound-gcp
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  ENABLED = { TRUE | FALSE }
//	  TYPE = QUEUE
//	  DIRECTION = OUTBOUND
//	  NOTIFICATION_PROVIDER = GCP_PUBSUB
//	  GCP_PUBSUB_TOPIC_NAME = '<topic_id>'
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationOutboundGooglePubSub() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("ENABLED", v.parseBool),
				v.option("TYPE", v.wordsValue("QUEUE")),
				v.option("DIRECTION", v.wordsValue("OUTBOUND")),
				v.option("NOTIFICATION_PROVIDER", v.wordsValue("GCP_PUBSUB")),
				v.option("GCP_PUBSUB_TOPIC_NAME", v.parseString),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateNotificationIntegrationWebhooks validates the Snowflake `CREATE NOTIFICATION INTEGRATION (webhooks)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-notification-integration-webhooks
//
// Syntax:
//
//	CREATE [ OR REPLACE ] NOTIFICATION INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = WEBHOOK
//	  ENABLED = { TRUE | FALSE }
//	  WEBHOOK_URL = '<url>'
//	  [ WEBHOOK_SECRET = <secret_name> ]
//	  [ WEBHOOK_BODY_TEMPLATE = '<template_for_http_request_body>' ]
//	  [ WEBHOOK_HEADERS = ( '<header_1>'='<value_1>' [ , '<header_N>'='<value_N>', ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateNotificationIntegrationWebhooks() bool {
	headerItem := v.option2(v.parseString, v.parseString)
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("NOTIFICATION") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.unorderedOnce(
				v.option("TYPE", v.wordsValue("WEBHOOK")),
				v.option("ENABLED", v.parseBool),
				v.option("WEBHOOK_URL", v.parseString),
				v.option("WEBHOOK_SECRET", v.parseIdentPath),
				v.option("WEBHOOK_BODY_TEMPLATE", v.parseString),
				v.option("WEBHOOK_HEADERS", func() bool { return v.parseParenList(headerItem) }),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateOnlineFeatureTable validates the Snowflake `CREATE ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-online-feature-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ONLINE FEATURE TABLE <name>
//	  PRIMARY KEY ( <col_name> [ , <col_name> , ... ] )
//	  TARGET_LAG = '<num> { seconds | minutes | hours | days }'
//	  WAREHOUSE = <warehouse_name>
//	  [ REFRESH_MODE = { AUTO | FULL | INCREMENTAL } ]
//	  [ TIMESTAMP_COLUMN = <col_name> ]
//	  [ [ WITH ] COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	FROM <source>
func (v *Validator) ParseCreateOnlineFeatureTable() bool {
	primaryKey := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("PRIMARY") },
			func() bool { return v.MatchKeyword("KEY") },
			func() bool { return v.parseParenList(v.parseIdentPath) },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ONLINE") },
		func() bool { return v.MatchWord("FEATURE") },
		func() bool { return v.MatchKeyword("TABLE") },
		v.parseIdentPath,
		// Order-independent body: PRIMARY KEY (...), TARGET_LAG/WAREHOUSE/etc.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					primaryKey,
					v.option("TARGET_LAG", v.parseString),
					v.option("WAREHOUSE", v.parseIdentPath),
					v.option("REFRESH_MODE", v.wordsValue("AUTO", "FULL", "INCREMENTAL")),
					v.option("TIMESTAMP_COLUMN", v.parseIdentPath),
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
							v.commentOption(),
						)
					},
					v.tagClause,
				)
			})
		},
		// FROM <source>
		func() bool { return v.MatchKeyword("FROM") },
		v.parseIdentPath,
	)
}

// ParseCreateOrAlterObj validates the Snowflake `CREATE OR ALTER <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-or-alter
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseCreateOrAlterObj() bool {
	// Lenient dispatcher for `CREATE OR ALTER <object-type> [IF NOT EXISTS] <name> ...`.
	// Require the CREATE OR ALTER skeleton + an object-type word + name, then accept
	// a permissive run of tokens (balanced-paren aware) for the object-specific body.
	consumeOne := func() bool {
		return v.Choice(
			func() bool { return v.parseParenList(v.parseScalar) },
			func() bool { return v.Match(sqltok.LParen) },
			func() bool { return v.Match(sqltok.RParen) },
			func() bool { return v.Match(sqltok.Comma) },
			func() bool { return v.Match(sqltok.StringLit) },
			func() bool { return v.Match(sqltok.NumberLit) },
			func() bool { return v.MatchOp("=") },
			func() bool { return v.MatchOp("=>") },
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchKeyword("OR") },
		func() bool { return v.MatchWord("ALTER") },
		// object type, e.g. TABLE / TASK / WAREHOUSE / NETWORK RULE — accept one or two words
		v.parseIdentPath,
		func() bool { return v.Optional(func() bool { return v.MatchWord("RULE") }) },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(consumeOne) },
	)
}

// ParseCreateOrganizationAccount validates the Snowflake `CREATE ORGANIZATION ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-account
//
// Syntax:
//
//	CREATE ORGANIZATION ACCOUNT <name>
//	    ADMIN_NAME = <string>
//	    { ADMIN_PASSWORD = '<string_literal>' | ADMIN_RSA_PUBLIC_KEY = <string> }
//	    [ FIRST_NAME = <string> ]
//	    [ LAST_NAME = <string> ]
//	    EMAIL = '<string>'
//	    [ MUST_CHANGE_PASSWORD = { TRUE | FALSE } ]
//	    EDITION = { ENTERPRISE | BUSINESS_CRITICAL }
//	    [ REGION_GROUP = <region_group_id> ]
//	    [ REGION = <snowflake_region_id> ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateOrganizationAccount() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("ACCOUNT") },
		v.parseIdentPath,
		// Order-independent property list (ADMIN_NAME, EMAIL, EDITION required by doc;
		// kept lenient as an option set).
		func() bool {
			return v.unorderedOnce(
				v.option("ADMIN_NAME", v.parseScalar),
				v.option("ADMIN_PASSWORD", v.parseString),
				v.option("ADMIN_RSA_PUBLIC_KEY", v.parseScalar),
				v.option("FIRST_NAME", v.parseScalar),
				v.option("LAST_NAME", v.parseScalar),
				v.option("EMAIL", v.parseString),
				v.option("MUST_CHANGE_PASSWORD", v.parseBool),
				v.option("EDITION", v.wordsValue("ENTERPRISE", "BUSINESS_CRITICAL")),
				v.option("REGION_GROUP", v.parseScalar),
				v.option("REGION", v.parseScalar),
				v.commentOption(),
			)
		},
	)
}

// ParseCreateOrganizationListing validates the Snowflake `CREATE ORGANIZATION LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-listing
//
// Syntax:
//
//	CREATE ORGANIZATION LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  AS '<yaml_manifest_string>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//
//	CREATE ORGANIZATION LISTING [ IF NOT EXISTS ] <name>
//	  [ { SHARE <share_name>  |  APPLICATION PACKAGE <package_name> } ]
//	  FROM '<yaml_manifest_stage_location>'
//	  [ PUBLISH = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationListing() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("LISTING") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ SHARE <name> | APPLICATION PACKAGE <name> ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("SHARE") },
							v.parseIdentPath,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("APPLICATION") },
							func() bool { return v.MatchWord("PACKAGE") },
							v.parseIdentPath,
						)
					},
				)
			})
		},
		// AS '<manifest>' | FROM '<stage_location>'
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("AS") },
						v.parseString,
					)
				},
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("FROM") },
						v.parseString,
					)
				},
			)
		},
		func() bool { return v.Optional(v.option("PUBLISH", v.parseBool)) },
	)
}

// ParseCreateOrganizationProfile validates the Snowflake `CREATE ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-profile
//
// Syntax:
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//	  AS '<yaml_manifest_string>'
//	  [ VERSION <version_alias_name> ]
//	  [ PUBLISH = { TRUE | FALSE } ]
//
//	CREATE ORGANIZATION PROFILE [ IF NOT EXISTS ] <name>
//	  FROM @<yaml_manifest_stage_location>
//	  [ VERSION <version_alias_name> ]
//	  [ PUBLISH = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationProfile() bool {
	trailing := func() bool {
		v.Optional(func() bool {
			return v.Sequence(
				func() bool { return v.MatchWord("VERSION") },
				v.parseIdentPath,
			)
		})
		return v.Optional(v.option("PUBLISH", v.parseBool))
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("PROFILE") },
		v.ifNotExists,
		v.parseIdentPath,
		// Optional body: AS '<str>' ... | FROM @<stage> ... | (bare name)
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("AS") },
							v.parseString,
							trailing,
						)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchKeyword("FROM") },
							func() bool { return v.Choice(v.parseStageRef, v.parseIdentPath) },
							trailing,
						)
					},
				)
			})
		},
	)
}

// ParseCreateOrganizationUser validates the Snowflake `CREATE ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user
//
// Syntax:
//
//	CREATE ORGANIZATION USER [ IF NOT EXISTS ] <name>
//	  [ objectProperties ]
//
//	Where:
//
//	objectProperties ::=
//	  EMAIL = '<string>'
//	  LOGIN_NAME = '<string>'
//	  DISPLAY_NAME = '<string>'
//	  FIRST_NAME = '<string>'
//	  MIDDLE_NAME = '<string>'
//	  LAST_NAME = '<string>'
//	  COMMENT = '<string>'
func (v *Validator) ParseCreateOrganizationUser() bool {
	prop := func() bool {
		return v.Choice(
			v.option("EMAIL", v.parseString),
			v.option("LOGIN_NAME", v.parseString),
			v.option("DISPLAY_NAME", v.parseString),
			v.option("FIRST_NAME", v.parseString),
			v.option("MIDDLE_NAME", v.parseString),
			v.option("LAST_NAME", v.parseString),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("USER") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateOrganizationUserGroup validates the Snowflake `CREATE ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-organization-user-group
//
// Syntax:
//
//	CREATE ORGANIZATION USER GROUP [ IF NOT EXISTS ] <name>
//	  [ IS_GRANTABLE = { TRUE | FALSE } ]
func (v *Validator) ParseCreateOrganizationUserGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.MatchWord("GROUP") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(v.option("IS_GRANTABLE", v.parseBool)) },
	)
}

// ParseCreatePackagesPolicy validates the Snowflake `CREATE PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-packages-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PACKAGES POLICY [ IF NOT EXISTS ] <name>
//	  LANGUAGE PYTHON
//	  [ ALLOWLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ ADDITIONAL_CREATION_BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePackagesPolicy() bool {
	// A possibly-empty parenthesized list of string literals.
	strList := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						v.parseString,
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseString)
							})
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("ALLOWLIST", strList),
			v.option("BLOCKLIST", strList),
			v.option("ADDITIONAL_CREATION_BLOCKLIST", strList),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("PACKAGES") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("LANGUAGE") },
		func() bool { return v.MatchWord("PYTHON") },
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreatePasswordPolicy validates the Snowflake `CREATE PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-password-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PASSWORD POLICY [ IF NOT EXISTS ] <name>
//	  [ PASSWORD_MIN_LENGTH = <integer> ]
//	  [ PASSWORD_MAX_LENGTH = <integer> ]
//	  [ PASSWORD_MIN_UPPER_CASE_CHARS = <integer> ]
//	  [ PASSWORD_MIN_LOWER_CASE_CHARS = <integer> ]
//	  [ PASSWORD_MIN_NUMERIC_CHARS = <integer> ]
//	  [ PASSWORD_MIN_SPECIAL_CHARS = <integer> ]
//	  [ PASSWORD_MIN_AGE_DAYS = <integer> ]
//	  [ PASSWORD_MAX_AGE_DAYS = <integer> ]
//	  [ PASSWORD_MAX_RETRIES = <integer> ]
//	  [ PASSWORD_LOCKOUT_TIME_MINS = <integer> ]
//	  [ PASSWORD_HISTORY = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePasswordPolicy() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	prop := func() bool {
		return v.Choice(
			v.option("PASSWORD_MIN_LENGTH", num),
			v.option("PASSWORD_MAX_LENGTH", num),
			v.option("PASSWORD_MIN_UPPER_CASE_CHARS", num),
			v.option("PASSWORD_MIN_LOWER_CASE_CHARS", num),
			v.option("PASSWORD_MIN_NUMERIC_CHARS", num),
			v.option("PASSWORD_MIN_SPECIAL_CHARS", num),
			v.option("PASSWORD_MIN_AGE_DAYS", num),
			v.option("PASSWORD_MAX_AGE_DAYS", num),
			v.option("PASSWORD_MAX_RETRIES", num),
			v.option("PASSWORD_LOCKOUT_TIME_MINS", num),
			v.option("PASSWORD_HISTORY", num),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("PASSWORD") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreatePipe validates the Snowflake `CREATE PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-pipe
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PIPE [ IF NOT EXISTS ] <name>
//	  [ AUTO_INGEST = [ TRUE | FALSE ] ]
//	  [ ERROR_INTEGRATION = <integration_name> ]
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ INTEGRATION = '<string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS <copy_statement>
//
//	CREATE OR ALTER PIPE <name>
//	  [ AUTO_INGEST = [ TRUE | FALSE ] ]
//	  [ ERROR_INTEGRATION = <integration_name> ]
//	  [ AWS_SNS_TOPIC = '<string>' ]
//	  [ INTEGRATION = '<string>' ]
//	  [ PIPE_EXECUTION_PAUSED = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//	  AS <copy_statement>
func (v *Validator) ParseCreatePipe() bool {
	// consumeRest greedily consumes every remaining token (the free-form
	// <copy_statement> body following AS).
	consumeRest := func() bool {
		for !v.AtEnd() {
			v.advance()
		}
		return true
	}
	prop := func() bool {
		return v.Choice(
			v.option("AUTO_INGEST", v.parseBool),
			v.option("ERROR_INTEGRATION", v.parseIdentPath),
			v.option("AWS_SNS_TOPIC", v.parseString),
			v.option("INTEGRATION", v.parseString),
			v.option("PIPE_EXECUTION_PAUSED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchKeyword("PIPE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
		func() bool { return v.MatchKeyword("AS") },
		consumeRest,
	)
}

// ParseCreatePostgresInstance validates the Snowflake `CREATE POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-postgres-instance
//
// Syntax:
//
//	CREATE POSTGRES INSTANCE <name>
//	  COMPUTE_FAMILY = '<compute_family>'
//	  STORAGE_SIZE_GB = <storage_gb>
//	  AUTHENTICATION_AUTHORITY = { POSTGRES | POSTGRES_OR_SNOWFLAKE }
//	  [ POSTGRES_VERSION = { 16 | 17 | 18 } ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ HIGH_AVAILABILITY = { TRUE | FALSE } ]
//	  [ STORAGE_INTEGRATION = '<storage_integration_name>' ]
//	  [ POSTGRES_SETTINGS = '<json_string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//
//	CREATE POSTGRES INSTANCE <name>
//	  FORK <source_instance>
//	  [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> } ) ]
//	  [ COMPUTE_FAMILY = '<compute_family>' ]
//	  [ STORAGE_SIZE_GB = <storage_gb> ]
//	  [ HIGH_AVAILABILITY = { TRUE | FALSE } ]
//	  [ POSTGRES_SETTINGS = '<json_string>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
func (v *Validator) ParseCreatePostgresInstance() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	prop := func() bool {
		return v.Choice(
			v.option("COMPUTE_FAMILY", v.parseString),
			v.option("STORAGE_SIZE_GB", num),
			v.option("AUTHENTICATION_AUTHORITY", v.wordsValue("POSTGRES", "POSTGRES_OR_SNOWFLAKE")),
			v.option("POSTGRES_VERSION", num),
			v.option("NETWORK_POLICY", v.parseString),
			v.option("HIGH_AVAILABILITY", v.parseBool),
			v.option("STORAGE_INTEGRATION", v.parseString),
			v.option("POSTGRES_SETTINGS", v.parseString),
			v.commentOption(),
			v.tagClause,
		)
	}
	// FORK <source> [ { AT | BEFORE } ( { TIMESTAMP => ... | OFFSET => ... } ) ]
	forkClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("FORK") },
			v.parseIdentPath,
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						v.wordsValue("AT", "BEFORE"),
						func() bool { return v.Match(sqltok.LParen) },
						func() bool {
							return v.Choice(v.arrowOption("TIMESTAMP"), v.arrowOption("OFFSET"))
						},
						func() bool { return v.Match(sqltok.RParen) },
					)
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("POSTGRES") },
		func() bool { return v.MatchWord("INSTANCE") },
		v.parseIdentPath,
		func() bool { return v.Optional(forkClause) },
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreatePrivacyPolicy validates the Snowflake `CREATE PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-privacy-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PRIVACY POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS PRIVACY_BUDGET -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreatePrivacyPolicy() bool {
	// The policy <body> is an arbitrary SQL expression; consume the remaining
	// tokens permissively after the `->` arrow.
	consumeRest := func() bool {
		for !v.AtEnd() {
			v.advance()
		}
		return true
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("PRIVACY") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.Match(sqltok.RParen) },
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("PRIVACY_BUDGET") },
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		consumeRest,
	)
}

// ParseCreateProcedure validates the Snowflake `CREATE PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-procedure
//
// Syntax:
//
//	-- Java handler (in-line)
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] [ SECURE ] PROCEDURE <name> (
//	    [ <arg_name> <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> [ [ NOT ] NULL ] | TABLE ( [ <col_name> <col_data_type> [ , ... ] ] ) }
//	  LANGUAGE JAVA
//	  RUNTIME_VERSION = '<java_runtime_version>'
//	  PACKAGES = ( 'com.snowflake:snowpark:<version>' [, '<package_name_and_version>' ...] )
//	  [ IMPORTS = ( ... ) ]
//	  HANDLER = '<fully_qualified_method_name>'
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <name_of_integration> [ , ... ] ) ]
//	  [ SECRETS = ('<secret_variable_name>' = <secret_name> [ , ... ] ) ]
//	  [ TARGET_PATH = '<stage_path_and_file_name_to_write>' ]
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER } ]
//	  AS '<procedure_definition>'
//
//	-- Other languages: JAVASCRIPT, PYTHON, SCALA, and Snowflake Scripting (LANGUAGE SQL).
//
//	-- Snowflake Scripting handler
//	CREATE [ OR REPLACE ] PROCEDURE <name> (
//	    [ <arg_name> [ { IN | INPUT | OUT | OUTPUT } ] <arg_data_type> [ DEFAULT <default_value> ] ] [ , ... ] )
//	  [ COPY GRANTS ]
//	  RETURNS { <result_data_type> | TABLE ( [ <col_name> <col_data_type> [ , ... ] ] ) }
//	  [ NOT NULL ]
//	  LANGUAGE SQL
//	  [ { CALLED ON NULL INPUT | { RETURNS NULL ON NULL INPUT | STRICT } } ]
//	  [ { VOLATILE | IMMUTABLE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER } ]
//	  AS <procedure_definition>
func (v *Validator) ParseCreateProcedure() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	parenIdentList := func() bool { return v.parseParenList(v.parseIdentPath) }
	parenStringList := func() bool { return v.parseParenList(v.parseString) }
	// RETURNS { <result_data_type> | TABLE ( ... ) }
	returnsType := func() bool {
		return v.Choice(
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("TABLE") },
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
			func() bool {
				return v.Sequence(
					v.parseIdentPath,
					func() bool { return v.Optional(v.consumeBalancedParens) },
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(v.wordsValue("TEMP", "TEMPORARY")) },
		func() bool { return v.Optional(func() bool { return v.MatchKeyword("SECURE") }) },
		func() bool { return v.MatchKeyword("PROCEDURE") },
		v.parseIdentPath,
		// argument list ( ... ) — free-form arg defs.
		v.consumeBalancedParens,
		// [ COPY GRANTS ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("COPY") },
					func() bool { return v.MatchWord("GRANTS") },
				)
			})
		},
		// RETURNS { <type> [ [ NOT ] NULL ] | TABLE ( ... ) }
		func() bool { return v.MatchWord("RETURNS") },
		returnsType,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchWord("NOT") }) },
					func() bool { return v.MatchWord("NULL") },
				)
			})
		},
		// Order-independent body of property clauses (LANGUAGE, HANDLER, the
		// null-input / volatility flags, EXECUTE AS, …).
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("LANGUAGE") }, v.parseIdentPath)
					},
					func() bool { return v.phrase("CALLED", "ON", "NULL", "INPUT") },
					func() bool { return v.phrase("RETURNS", "NULL", "ON", "NULL", "INPUT") },
					func() bool { return v.MatchWord("STRICT") },
					v.wordsValue("VOLATILE", "IMMUTABLE"),
					v.option("RUNTIME_VERSION", v.parseScalar),
					v.option("PACKAGES", parenStringList),
					v.option("IMPORTS", parenStringList),
					v.option("HANDLER", v.parseString),
					v.option("EXTERNAL_ACCESS_INTEGRATIONS", parenIdentList),
					v.option("SECRETS", v.consumeBalancedParens),
					v.option("TARGET_PATH", v.parseString),
					v.option("MAX_BATCH_ROWS", num),
					v.commentOption(),
					// EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER }
					func() bool {
						return v.Sequence(
							func() bool { return v.phrase("EXECUTE", "AS") },
							func() bool {
								return v.Choice(
									func() bool { return v.MatchWord("OWNER") },
									func() bool { return v.phrase("RESTRICTED", "CALLER") },
									func() bool { return v.MatchWord("CALLER") },
								)
							},
						)
					},
					// Generic `<option> = <value>` fallback — keep LAST so the rule
					// tolerates language/option keys not enumerated above (this rule is
					// dispatched, so an unmodeled option must not false-reject a valid
					// procedure). The RETURNS skeleton and AS body stay required.
					v.option2(v.parseIdentPath, v.parseScalar),
				)
			})
		},
		// AS '<procedure_definition>' — a string literal, or a permissive
		// consume-to-EOF fallback for dollar-quoted-like bodies.
		func() bool {
			return v.Sequence(
				func() bool { return v.MatchKeyword("AS") },
				func() bool {
					return v.Choice(
						v.parseString,
						func() bool {
							if v.AtEnd() {
								return false
							}
							for !v.AtEnd() {
								v.advance()
							}
							return true
						},
					)
				},
			)
		},
	)
}

// ParseCreateProjectionPolicy validates the Snowflake `CREATE PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-projection-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PROJECTION POLICY [ IF NOT EXISTS ] <name>
//	  AS () RETURNS PROJECTION_CONSTRAINT -> <body>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateProjectionPolicy() bool {
	consumeRest := func() bool {
		for !v.AtEnd() {
			v.advance()
		}
		return true
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("PROJECTION") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.Match(sqltok.RParen) },
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("PROJECTION_CONSTRAINT") },
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		consumeRest,
	)
}

// ParseCreateProvisionedThroughput validates the Snowflake `CREATE PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-provisioned-throughput
//
// Syntax:
//
//	CREATE [ OR REPLACE ] PROVISIONED THROUGHPUT <name>
//	    CLOUD_PROVIDER = '<cloud_provider>'
//	    MODEL = '<model_name>'
//	    PTUS = <num_ptus>
//	    TERM_START = '<start_date>'
//	    TERM_END = '<end_date>';
func (v *Validator) ParseCreateProvisionedThroughput() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	prop := func() bool {
		return v.Choice(
			v.option("CLOUD_PROVIDER", v.parseString),
			v.option("MODEL", v.parseString),
			v.option("PTUS", num),
			v.option("TERM_START", v.parseString),
			v.option("TERM_END", v.parseString),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("PROVISIONED") },
		func() bool { return v.MatchWord("THROUGHPUT") },
		v.parseIdentPath,
		// At least one property required.
		prop,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateReplicationGroup validates the Snowflake `CREATE REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-replication-group
//
// Syntax:
//
//	CREATE REPLICATION GROUP [ IF NOT EXISTS ] <name>
//	    OBJECT_TYPES = <object_type> [ , <object_type> , ... ]
//	    [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	    [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	    [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//	    [ ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> , ... ] ]
//	    ALLOWED_ACCOUNTS = <org_name>.<target_account_name> [ , <org_name>.<target_account_name> , ... ]
//	    [ IGNORE EDITION CHECK ]
//	    [ REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }' ]
//	    [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//
//	CREATE REPLICATION GROUP [ IF NOT EXISTS ] <secondary_name>
//	    AS REPLICA OF <org_name>.<source_account_name>.<name>
func (v *Validator) ParseCreateReplicationGroup() bool {
	// A bare comma-separated list of identifier-path items (no parens).
	identList := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseIdentPath)
				})
			},
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("OBJECT_TYPES", identList),
			v.option("ALLOWED_DATABASES", identList),
			v.option("ALLOWED_EXTERNAL_VOLUMES", identList),
			v.option("ALLOWED_SHARES", identList),
			v.option("ALLOWED_INTEGRATION_TYPES", identList),
			v.option("ALLOWED_ACCOUNTS", identList),
			v.option("REPLICATION_SCHEDULE", v.parseString),
			v.option("ERROR_INTEGRATION", v.parseIdentPath),
			// IGNORE EDITION CHECK
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("IGNORE") },
					func() bool { return v.MatchWord("EDITION") },
					func() bool { return v.MatchWord("CHECK") },
				)
			},
			v.tagClause,
		)
	}
	// AS REPLICA OF <name>
	replicaClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("AS") },
			func() bool { return v.MatchWord("REPLICA") },
			func() bool { return v.MatchKeyword("OF") },
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("REPLICATION") },
		func() bool { return v.MatchWord("GROUP") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool {
			return v.Choice(replicaClause, func() bool {
				// At least one property in the primary form.
				return v.Sequence(prop, func() bool { return v.ZeroOrMore(prop) })
			})
		},
	)
}

// ParseCreateResourceMonitor validates the Snowflake `CREATE RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-resource-monitor
//
// Syntax:
//
//	CREATE [ OR REPLACE ] RESOURCE MONITOR [ IF NOT EXISTS ] <name> WITH
//	                      [ CREDIT_QUOTA = <number> ]
//	                      [ FREQUENCY = { MONTHLY | DAILY | WEEKLY | YEARLY | NEVER } ]
//	                      [ START_TIMESTAMP = { <timestamp> | IMMEDIATELY } ]
//	                      [ END_TIMESTAMP = <timestamp> ]
//	                      [ NOTIFY_USERS = ( <user_name> [ , <user_name> , ... ] ) ]
//	                      [ TRIGGERS triggerDefinition [ triggerDefinition ... ] ]
//
//	Where:
//
//	triggerDefinition ::=
//	    ON <threshold> PERCENT DO { SUSPEND | SUSPEND_IMMEDIATE | NOTIFY }
func (v *Validator) ParseCreateResourceMonitor() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// triggerDefinition: ON <threshold> PERCENT DO { SUSPEND | SUSPEND_IMMEDIATE | NOTIFY }
	triggerDef := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("ON") },
			num,
			func() bool { return v.MatchWord("PERCENT") },
			func() bool { return v.MatchWord("DO") },
			v.wordsValue("SUSPEND", "SUSPEND_IMMEDIATE", "NOTIFY"),
		)
	}
	prop := func() bool {
		return v.Choice(
			v.option("CREDIT_QUOTA", num),
			v.option("FREQUENCY", v.wordsValue("MONTHLY", "DAILY", "WEEKLY", "YEARLY", "NEVER")),
			v.option("START_TIMESTAMP", func() bool {
				return v.Choice(v.parseString, func() bool { return v.MatchWord("IMMEDIATELY") })
			}),
			v.option("END_TIMESTAMP", v.parseString),
			v.option("NOTIFY_USERS", func() bool { return v.parseParenList(v.parseIdentPath) }),
			// TRIGGERS triggerDefinition [ triggerDefinition ... ]
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("TRIGGERS") },
					triggerDef,
					func() bool { return v.ZeroOrMore(triggerDef) },
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("RESOURCE") },
		func() bool { return v.MatchWord("MONITOR") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("WITH") },
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateRole validates the Snowflake `CREATE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-role
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ROLE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//
//	CREATE OR ALTER ROLE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateRole() bool {
	prop := func() bool {
		return v.Choice(
			v.commentOption(),
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchKeyword("ROLE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateRowAccessPolicy validates the Snowflake `CREATE ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-row-access-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ROW ACCESS POLICY [ IF NOT EXISTS ] <name> AS
//	( <arg_name> <arg_type> [ , ... ] ) RETURNS BOOLEAN -> <body>
//	[ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateRowAccessPolicy() bool {
	// The signature ( <arg> <type> [, ...] ) is consumed as a balanced-paren
	// span; the trailing <body> is an arbitrary expression consumed permissively.
	consumeRest := func() bool {
		for !v.AtEnd() {
			v.advance()
		}
		return true
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchKeyword("ROW") },
		func() bool { return v.MatchWord("ACCESS") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		v.consumeBalancedParens,
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("BOOLEAN") },
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		consumeRest,
	)
}

// ParseCreateSchema validates the Snowflake `CREATE SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-schema
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ TRANSIENT ] SCHEMA [ IF NOT EXISTS ] <name>
//	  [ CLONE <source_schema>
//	      [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> } ) ]
//	      [ IGNORE TABLES WITH INSUFFICIENT DATA RETENTION ]
//	      [ IGNORE HYBRID TABLES ] ]
//	  [ WITH MANAGED ACCESS ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ EXTERNAL_VOLUME = <external_volume_name> ]
//	  [ CATALOG = <catalog_integration_name> ]
//	  [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	  [ CLASSIFICATION_PROFILE = '<classification_profile>' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	  [ OBJECT_VISIBILITY = PRIVILEGED ]
//	  [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//
//	CREATE SCHEMA <name> FROM BACKUP SET <backup_set> IDENTIFIER '<backup_id>'
//
//	CREATE OR ALTER [ TRANSIENT ] SCHEMA <name>
//	  [ ... schema properties ... ]
func (v *Validator) ParseCreateSchema() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	str := v.parseString
	name := v.parseIdentPath

	// CLONE <source> [ { AT | BEFORE } ( ... ) ] [ IGNORE ... ]
	cloneClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("CLONE") },
			name,
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						v.wordsValue("AT", "BEFORE"),
						func() bool { return v.Match(sqltok.LParen) },
						func() bool {
							return v.Choice(
								v.arrowOption("TIMESTAMP"),
								v.arrowOption("OFFSET"),
								v.arrowOption("STATEMENT"),
							)
						},
						func() bool { return v.Match(sqltok.RParen) },
					)
				})
			},
			func() bool {
				return v.Optional(func() bool {
					return v.phrase("IGNORE", "TABLES", "WITH", "INSUFFICIENT", "DATA", "RETENTION")
				})
			},
			func() bool {
				return v.Optional(func() bool { return v.phrase("IGNORE", "HYBRID", "TABLES") })
			},
		)
	}
	prop := func() bool {
		return v.Choice(
			// WITH MANAGED ACCESS
			func() bool { return v.phrase("WITH", "MANAGED", "ACCESS") },
			v.option("DATA_RETENTION_TIME_IN_DAYS", num),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
			v.option("EXTERNAL_VOLUME", name),
			v.option("ICEBERG_VERSION_DEFAULT", num),
			v.option("ICEBERG_MERGE_ON_READ_BEHAVIOR", str),
			v.option("ENABLE_ICEBERG_MERGE_ON_READ", v.parseBool),
			v.option("REPLACE_INVALID_CHARACTERS", v.parseBool),
			v.option("DEFAULT_DDL_COLLATION", str),
			v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
			v.option("CLASSIFICATION_PROFILE", str),
			v.option("CATALOG_SYNC", str),
			v.option("CATALOG", name),
			v.option("OBJECT_VISIBILITY", v.parseScalar),
			v.option("ENABLE_DATA_COMPACTION", v.parseBool),
			v.commentOption(),
			// WITH CONTACT ( <purpose> = <contact> [, ...] )
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("CONTACT") },
					func() bool { return v.parseParenList(v.option2(name, name)) },
				)
			},
			v.tagClause,
		)
	}
	// FROM BACKUP SET <name> IDENTIFIER '<id>'
	fromBackup := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("FROM") },
			func() bool { return v.MatchWord("BACKUP") },
			func() bool { return v.MatchKeyword("SET") },
			name,
			func() bool { return v.MatchWord("IDENTIFIER") },
			str,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.Optional(func() bool { return v.MatchWord("TRANSIENT") }) },
		func() bool { return v.MatchKeyword("SCHEMA") },
		v.ifNotExists,
		name,
		func() bool {
			return v.Choice(
				fromBackup,
				func() bool {
					v.Optional(cloneClause)
					return v.ZeroOrMore(prop)
				},
			)
		},
	)
}

// ParseCreateSecret validates the Snowflake `CREATE SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-secret
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = OAUTH2
//	  API_AUTHENTICATION = <security_integration_name>
//	  OAUTH_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] )
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = OAUTH2
//	  OAUTH_REFRESH_TOKEN = '<string_literal>'
//	  OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '<string_literal>'
//	  API_AUTHENTICATION = <security_integration_name>;
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = CLOUD_PROVIDER_TOKEN
//	  API_AUTHENTICATION = '<cloud_provider_security_integration>'
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = PASSWORD
//	  USERNAME = '<username>'
//	  PASSWORD = '<password>'
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = GENERIC_STRING
//	  SECRET_STRING = '<string_literal>'
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE [ OR REPLACE ] SECRET [ IF NOT EXISTS ] <name>
//	  TYPE = SYMMETRIC_KEY
//	  ALGORITHM = GENERIC
func (v *Validator) ParseCreateSecret() bool {
	// Many TYPE variants share an option pool; accept them order-independently
	// after the required TYPE = <type>.
	prop := func() bool {
		return v.Choice(
			v.option("API_AUTHENTICATION", func() bool {
				return v.Choice(v.parseString, v.parseIdentPath)
			}),
			v.option("OAUTH_SCOPES", func() bool { return v.parseParenList(v.parseString) }),
			v.option("OAUTH_REFRESH_TOKEN", v.parseString),
			v.option("OAUTH_REFRESH_TOKEN_EXPIRY_TIME", v.parseString),
			v.option("ENABLED", v.parseBool),
			v.option("USERNAME", v.parseString),
			v.option("PASSWORD", v.parseString),
			v.option("SECRET_STRING", v.parseString),
			v.option("ALGORITHM", v.wordsValue("GENERIC")),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchKeyword("SECRET") },
		v.ifNotExists,
		v.parseIdentPath,
		v.option("TYPE", v.wordsValue(
			"OAUTH2", "CLOUD_PROVIDER_TOKEN", "PASSWORD", "GENERIC_STRING", "SYMMETRIC_KEY",
		)),
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateSecurityIntegration validates the Snowflake `CREATE SECURITY INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	  <name>
//	  TYPE = { API_AUTHENTICATION | EXTERNAL_OAUTH | OAUTH | SAML2 | SCIM }
//	  ...
func (v *Validator) ParseCreateSecurityIntegration() bool {
	// Lenient skeleton: the documented body is type-dependent (see the
	// per-type variants). Require the CREATE … SECURITY INTEGRATION … <name>
	// TYPE = <kind> header, then accept any run of order-independent options.
	value := func() bool {
		return v.Choice(
			func() bool { return v.parseParenList(v.parseScalar) },
			v.parseScalar,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		v.option("TYPE", v.wordsValue(
			"API_AUTHENTICATION", "EXTERNAL_OAUTH", "OAUTH", "SAML2", "SCIM",
		)),
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.tagClause,
					v.commentOption(),
					func() bool { return v.option2(v.parseIdentPath, value)() },
				)
			})
		},
	)
}

// ParseCreateSecurityIntegrationExternalApiAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (External API Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-api-auth
//
// Syntax:
//
//	CREATE SECURITY INTEGRATION <name>
//	  TYPE = API_AUTHENTICATION
//	  AUTH_TYPE = OAUTH2
//	  ENABLED = { TRUE | FALSE }
//	  [ OAUTH_TOKEN_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_CLIENT_AUTH_METHOD = { CLIENT_SECRET_BASIC | CLIENT_SECRET_POST } ]
//	  [ OAUTH_CLIENT_ID = '<string_literal>' ]
//	  [ OAUTH_CLIENT_SECRET = '<string_literal>' ]
//	  [ OAUTH_GRANT = 'CLIENT_CREDENTIALS']
//	  [ OAUTH_ACCESS_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_ALLOWED_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- AUTHORIZATION_CODE and JWT_BEARER grant variants are also supported
//	-- (with OAUTH_AUTHORIZATION_ENDPOINT and OAUTH_REFRESH_TOKEN_VALIDITY).
func (v *Validator) ParseCreateSecurityIntegrationExternalApiAuthentication() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("API_AUTHENTICATION")),
			v.option("AUTH_TYPE", v.wordsValue("OAUTH2")),
			v.option("ENABLED", v.parseBool),
			v.option("OAUTH_TOKEN_ENDPOINT", v.parseString),
			v.option("OAUTH_CLIENT_AUTH_METHOD", v.wordsValue("CLIENT_SECRET_BASIC", "CLIENT_SECRET_POST")),
			v.option("OAUTH_CLIENT_ID", v.parseString),
			v.option("OAUTH_CLIENT_SECRET", v.parseString),
			v.option("OAUTH_GRANT", v.parseString),
			v.option("OAUTH_ACCESS_TOKEN_VALIDITY", num),
			v.option("OAUTH_REFRESH_TOKEN_VALIDITY", num),
			v.option("OAUTH_AUTHORIZATION_ENDPOINT", v.parseString),
			v.option("OAUTH_ALLOWED_SCOPES", func() bool { return v.parseParenList(v.parseString) }),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSecurityIntegrationAwsIamAuthentication validates the Snowflake `CREATE SECURITY INTEGRATION (AWS IAM Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-aws-iam
//
// Syntax:
//
//	CREATE SECURITY INTEGRATION <name>
//	  TYPE = API_AUTHENTICATION
//	  AUTH_TYPE = AWS_IAM
//	  AWS_ROLE_ARN = '<iam_role_arn>'
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationAwsIamAuthentication() bool {
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("API_AUTHENTICATION")),
			v.option("AUTH_TYPE", v.wordsValue("AWS_IAM")),
			v.option("AWS_ROLE_ARN", v.parseString),
			v.option("ENABLED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSecurityIntegrationExternalOauth validates the Snowflake `CREATE SECURITY INTEGRATION (External OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-external
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = EXTERNAL_OAUTH
//	  ENABLED = { TRUE | FALSE }
//	  EXTERNAL_OAUTH_TYPE = { OKTA | AZURE | PING_FEDERATE | CUSTOM }
//	  EXTERNAL_OAUTH_ISSUER = '<string_literal>'
//	  EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) }
//	  EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = { 'LOGIN_NAME' | 'EMAIL_ADDRESS' }
//	  [ EXTERNAL_OAUTH_JWS_KEYS_URL = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) } ]
//	  [ EXTERNAL_OAUTH_BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY = <public_key1> ]
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY_2 = <public_key2> ]
//	  [ EXTERNAL_OAUTH_AUDIENCE_LIST = { '<string_literal>' | ('<string_literal>' [ , '<string_literal>' , ... ] ) } ]
//	  [ EXTERNAL_OAUTH_ANY_ROLE_MODE = { DISABLE | ENABLE | ENABLE_FOR_PRIVILEGE } ]
//	  [ EXTERNAL_OAUTH_SCOPE_DELIMITER = '<string_literal>' ]
//	  [ EXTERNAL_OAUTH_SCOPE_MAPPING_ATTRIBUTE = '<string_literal>' ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationExternalOauth() bool {
	strOrList := func() bool {
		return v.Choice(
			func() bool { return v.parseParenList(v.parseString) },
			v.parseString,
		)
	}
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("EXTERNAL_OAUTH")),
			v.option("ENABLED", v.parseBool),
			v.option("EXTERNAL_OAUTH_TYPE", v.wordsValue("OKTA", "AZURE", "PING_FEDERATE", "CUSTOM")),
			v.option("EXTERNAL_OAUTH_ISSUER", v.parseString),
			v.option("EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM", strOrList),
			v.option("EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE", v.parseString),
			v.option("EXTERNAL_OAUTH_JWS_KEYS_URL", strOrList),
			v.option("EXTERNAL_OAUTH_BLOCKED_ROLES_LIST", func() bool { return v.parseParenList(v.parseString) }),
			v.option("EXTERNAL_OAUTH_ALLOWED_ROLES_LIST", func() bool { return v.parseParenList(v.parseString) }),
			v.option("EXTERNAL_OAUTH_RSA_PUBLIC_KEY", v.parseScalar),
			v.option("EXTERNAL_OAUTH_RSA_PUBLIC_KEY_2", v.parseScalar),
			v.option("EXTERNAL_OAUTH_AUDIENCE_LIST", strOrList),
			v.option("EXTERNAL_OAUTH_ANY_ROLE_MODE", v.wordsValue("DISABLE", "ENABLE", "ENABLE_FOR_PRIVILEGE")),
			v.option("EXTERNAL_OAUTH_SCOPE_DELIMITER", v.parseString),
			v.option("EXTERNAL_OAUTH_SCOPE_MAPPING_ATTRIBUTE", v.parseString),
			v.option("NETWORK_POLICY", v.parseScalar),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSecurityIntegrationSnowflakeOauth validates the Snowflake `CREATE SECURITY INTEGRATION (Snowflake OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-oauth-snowflake
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = OAUTH
//	  OAUTH_CLIENT = <partner_application>
//	  OAUTH_REDIRECT_URI = '<uri>'
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_ISSUE_REFRESH_TOKENS = { TRUE | FALSE } ]
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED = { TRUE | FALSE } ]
//	  [ OAUTH_USE_SECONDARY_ROLES = { IMPLICIT | NONE } ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- For custom clients, additionally:
//	--   OAUTH_CLIENT = CUSTOM
//	--   OAUTH_CLIENT_TYPE = 'CONFIDENTIAL' | 'PUBLIC'
func (v *Validator) ParseCreateSecurityIntegrationSnowflakeOauth() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("OAUTH")),
			v.option("OAUTH_CLIENT", v.parseScalar),
			v.option("OAUTH_CLIENT_TYPE", v.parseString),
			v.option("OAUTH_REDIRECT_URI", v.parseString),
			v.option("ENABLED", v.parseBool),
			v.option("OAUTH_ISSUE_REFRESH_TOKENS", v.parseBool),
			v.option("OAUTH_REFRESH_TOKEN_VALIDITY", num),
			v.option("OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED", v.parseBool),
			v.option("OAUTH_USE_SECONDARY_ROLES", v.wordsValue("IMPLICIT", "NONE")),
			v.option("NETWORK_POLICY", v.parseScalar),
			v.option("ALLOWED_ROLES_LIST", func() bool { return v.parseParenList(v.parseString) }),
			v.option("BLOCKED_ROLES_LIST", func() bool { return v.parseParenList(v.parseString) }),
			v.option("USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSecurityIntegrationSaml2 validates the Snowflake `CREATE SECURITY INTEGRATION (SAML2)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-saml2
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	    <name>
//	    TYPE = SAML2
//	    ENABLED = { TRUE | FALSE }
//	    { METADATA_URL = '<string_literal>' | <idp_parameters> }
//	    [ ALLOWED_USER_DOMAINS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	    [ ALLOWED_EMAIL_PATTERNS = ( '<string_literal>' [ , '<string_literal>' , ... ] ) ]
//	    [ SAML2_SP_INITIATED_LOGIN_PAGE_LABEL = '<string_literal>' ]
//	    [ SAML2_ENABLE_SP_INITIATED = TRUE | FALSE ]
//	    [ SAML2_SNOWFLAKE_X509_CERT = '<string_literal>' ]
//	    [ SAML2_SIGN_REQUEST = TRUE | FALSE ]
//	    [ SAML2_REQUESTED_NAMEID_FORMAT = '<string_literal>' ]
//	    [ SAML2_POST_LOGOUT_REDIRECT_URL = '<string_literal>' ]
//	    [ SAML2_FORCE_AUTHN = TRUE | FALSE ]
//	    [ SAML2_SNOWFLAKE_ISSUER_URL = '<string_literal>' ]
//	    [ SAML2_SNOWFLAKE_ACS_URL = '<string_literal>' ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationSaml2() bool {
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("SAML2")),
			v.option("ENABLED", v.parseBool),
			v.option("METADATA_URL", v.parseString),
			v.option("ALLOWED_USER_DOMAINS", func() bool { return v.parseParenList(v.parseString) }),
			v.option("ALLOWED_EMAIL_PATTERNS", func() bool { return v.parseParenList(v.parseString) }),
			v.option("SAML2_ISSUER", v.parseString),
			v.option("SAML2_SSO_URL", v.parseString),
			v.option("SAML2_PROVIDER", v.parseString),
			v.option("SAML2_X509_CERT", v.parseString),
			v.option("SAML2_SP_INITIATED_LOGIN_PAGE_LABEL", v.parseString),
			v.option("SAML2_ENABLE_SP_INITIATED", v.parseBool),
			v.option("SAML2_SNOWFLAKE_X509_CERT", v.parseString),
			v.option("SAML2_SIGN_REQUEST", v.parseBool),
			v.option("SAML2_REQUESTED_NAMEID_FORMAT", v.parseString),
			v.option("SAML2_POST_LOGOUT_REDIRECT_URL", v.parseString),
			v.option("SAML2_FORCE_AUTHN", v.parseBool),
			v.option("SAML2_SNOWFLAKE_ISSUER_URL", v.parseString),
			v.option("SAML2_SNOWFLAKE_ACS_URL", v.parseString),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSecurityIntegrationScim validates the Snowflake `CREATE SECURITY INTEGRATION (SCIM)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-security-integration-scim
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SECURITY INTEGRATION [ IF NOT EXISTS ]
//	    <name>
//	    TYPE = SCIM
//	    ENABLED = { TRUE | FALSE }
//	    SCIM_CLIENT = { 'OKTA' | 'AZURE' | 'GENERIC' }
//	    RUN_AS_ROLE = { 'OKTA_PROVISIONER' | 'AAD_PROVISIONER' | 'GENERIC_SCIM_PROVISIONER' | '<custom_role>' }
//	    [ NETWORK_POLICY = '<network_policy>' ]
//	    [ SYNC_PASSWORD = { TRUE | FALSE } ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSecurityIntegrationScim() bool {
	option := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("SCIM")),
			v.option("ENABLED", v.parseBool),
			v.option("SCIM_CLIENT", v.parseString),
			v.option("RUN_AS_ROLE", v.parseString),
			v.option("NETWORK_POLICY", v.parseScalar),
			v.option("SYNC_PASSWORD", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SECURITY") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		option,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSemanticView validates the Snowflake `CREATE SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-semantic-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SEMANTIC VIEW [ IF NOT EXISTS ] <name>
//	  TABLES ( logicalTable [ , ... ] )
//	  [ RELATIONSHIPS ( relationshipDef [ , ... ] ) ]
//	  [ FACTS ( factExpression [ , ... ] ) ]
//	  [ DIMENSIONS ( dimensionExpression [ , ... ] ) ]
//	  [ METRICS ( { metricExpression | windowFunctionMetricExpression } [ , ... ] ) ]
//	  [ COMMENT = '<comment_about_semantic_view>' ]
//	  [ AI_SQL_GENERATION '<instructions_for_sql_generation>' ]
//	  [ AI_QUESTION_CATEGORIZATION '<instructions_for_question_categorization>' ]
//	  [ AI_VERIFIED_QUERIES ( verifiedQuery [ , ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COPY GRANTS ]
//
//	-- Sub-definitions: logicalTable, relationshipDef, factExpression,
//	-- dimensionExpression, metricExpression, windowFunctionMetricExpression,
//	-- verifiedQuery (see Reference URL).
func (v *Validator) ParseCreateSemanticView() bool {
	// balanced consumes a single ( ... ) group, including nested parens. The
	// inner definitions (logicalTable, relationshipDef, expressions, …) are
	// free-form, so accept any balanced span rather than modeling them.
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							t := v.Peek()
							if t.Kind == sqltok.RParen || t.Kind == sqltok.EOF {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	parenClause := func(word string) Rule {
		return func() bool {
			return v.Sequence(func() bool { return v.MatchWord(word) }, balanced)
		}
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SEMANTIC") },
		func() bool { return v.MatchKeyword("VIEW") },
		v.ifNotExists,
		v.parseIdentPath,
		// TABLES ( ... ) is required.
		parenClause("TABLES"),
		// The remaining clauses may appear in any order.
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					parenClause("RELATIONSHIPS"),
					parenClause("FACTS"),
					parenClause("DIMENSIONS"),
					parenClause("METRICS"),
					parenClause("AI_VERIFIED_QUERIES"),
					v.commentOption(),
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("AI_SQL_GENERATION") }, v.parseString)
					},
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("AI_QUESTION_CATEGORIZATION") }, v.parseString)
					},
					v.tagClause,
					func() bool { return v.phrase("COPY", "GRANTS") },
				)
			})
		},
	)
}

// ParseCreateSequence validates the Snowflake `CREATE SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-sequence
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SEQUENCE [ IF NOT EXISTS ] <name>
//	  [ WITH ]
//	  [ START [ WITH ] [ = ] <initial_value> ]
//	  [ INCREMENT [ BY ] [ = ] <sequence_interval> ]
//	  [ { ORDER | NOORDER } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER SEQUENCE <name>
//	  [ WITH ]
//	  [ START [ WITH ] [ = ] <initial_value> ]
//	  [ INCREMENT [ BY ] [ = ] <sequence_interval> ]
//	  [ { ORDER | NOORDER } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSequence() bool {
	eqOpt := func() bool { return v.Optional(func() bool { return v.MatchOp("=") }) }
	// Each parameter may appear at most once, in any order, so unorderedOnce both
	// rejects duplicates and stops autocomplete re-offering a parameter already set.
	body := func() bool {
		return v.unorderedOnce(
			// START [ WITH ] [ = ] <initial_value>
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("START") },
					func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
					eqOpt,
					v.parseNumber,
				)
			},
			// INCREMENT [ BY ] [ = ] <sequence_interval>
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("INCREMENT") },
					func() bool { return v.Optional(func() bool { return v.MatchWord("BY") }) },
					eqOpt,
					v.parseNumber,
				)
			},
			v.wordsValue("ORDER", "NOORDER"),
			v.option("COMMENT", v.parseString),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchKeyword("SEQUENCE") },
		v.ifNotExists,
		v.parseIdentPath,
		// [ WITH ]
		func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
		body,
	)
}

// ParseCreateService validates the Snowflake `CREATE SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-service
//
// Syntax:
//
//	CREATE SERVICE [ IF NOT EXISTS ] <name>
//	  IN COMPUTE POOL <compute_pool_name>
//	  {
//	     fromSpecification
//	     | fromSpecificationTemplate
//	  }
//	  [ AUTO_SUSPEND_SECS = <num> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <EAI_name> [ , ... ] ) ]
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ MIN_INSTANCES = <num> ]
//	  [ MIN_READY_INSTANCES = <num> ]
//	  [ MAX_INSTANCES = <num> ]
//	  [ LOG_LEVEL = '<log_level>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COMMENT = '{string_literal}']
func (v *Validator) ParseCreateService() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// balanced consumes a single ( ... ) group, used for inline specs and lists.
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							t := v.Peek()
							if t.Kind == sqltok.RParen || t.Kind == sqltok.EOF {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// fromSpecification / fromSpecificationTemplate are free-form bodies:
	//   FROM { SPECIFICATION | SPECIFICATION_TEMPLATE } { $$...$$ | '<text>' }
	//   FROM { SPECIFICATION_FILE | SPECIFICATION_TEMPLATE_FILE } = '<path>'
	//   [ USING ( <key> => <value> [ , ... ] ) ]
	fromSpec := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("FROM") },
			func() bool {
				return v.Choice(
					// SPECIFICATION_FILE = '<path>' (and template variant)
					func() bool {
						return v.Sequence(
							v.wordsValue("SPECIFICATION_FILE", "SPECIFICATION_TEMPLATE_FILE"),
							func() bool { return v.MatchOp("=") },
							v.parseString,
						)
					},
					// SPECIFICATION '<inline_text>' (and template variant)
					func() bool {
						return v.Sequence(
							v.wordsValue("SPECIFICATION", "SPECIFICATION_TEMPLATE"),
							func() bool { return v.Optional(func() bool { return v.MatchOp("=") }) },
							v.parseString,
						)
					},
				)
			},
			// [ USING ( <key> => <value> [ , ... ] ) ]
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(func() bool { return v.MatchKeyword("USING") }, balanced)
				})
			},
		)
	}
	option := func() bool {
		return v.Choice(
			v.option("AUTO_SUSPEND_SECS", num),
			v.option("EXTERNAL_ACCESS_INTEGRATIONS", func() bool { return v.parseParenList(v.parseIdentPath) }),
			v.option("AUTO_RESUME", v.parseBool),
			v.option("MIN_INSTANCES", num),
			v.option("MIN_READY_INSTANCES", num),
			v.option("MAX_INSTANCES", num),
			v.option("LOG_LEVEL", v.parseString),
			v.option("QUERY_WAREHOUSE", v.parseIdentPath),
			v.tagClause,
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SERVICE") },
		v.ifNotExists,
		v.parseIdentPath,
		// IN COMPUTE POOL <compute_pool_name>
		func() bool { return v.MatchKeyword("IN") },
		func() bool { return v.MatchWord("COMPUTE") },
		func() bool { return v.MatchWord("POOL") },
		v.parseIdentPath,
		fromSpec,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSessionPolicy validates the Snowflake `CREATE SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-session-policy
//
// Syntax:
//
//	CREATE [OR REPLACE] SESSION POLICY [IF NOT EXISTS] <name>
//	  [ SESSION_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_UI_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_MAX_LIFESPAN_MINS = <integer> ]
//	  [ SESSION_UI_MAX_LIFESPAN_MINS = <integer> ]
//	  [ ALLOWED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ BLOCKED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateSessionPolicy() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	roleItem := func() bool {
		return v.Choice(v.parseString, v.parseIdentPath)
	}
	option := func() bool {
		return v.Choice(
			v.option("SESSION_IDLE_TIMEOUT_MINS", num),
			v.option("SESSION_UI_IDLE_TIMEOUT_MINS", num),
			v.option("SESSION_MAX_LIFESPAN_MINS", num),
			v.option("SESSION_UI_MAX_LIFESPAN_MINS", num),
			v.option("ALLOWED_SECONDARY_ROLES", func() bool { return v.parseParenList(roleItem) }),
			v.option("BLOCKED_SECONDARY_ROLES", func() bool { return v.parseParenList(roleItem) }),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SESSION") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateShare validates the Snowflake `CREATE SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-share
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SHARE [ IF NOT EXISTS ] <name>
//	  [ COMMENT = '<string_literal>' ]
//
//	CREATE OR ALTER SHARE <name>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateShare() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		// [ OR { REPLACE | ALTER } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("OR") },
					v.wordsValue("REPLACE", "ALTER"),
				)
			})
		},
		func() bool { return v.MatchWord("SHARE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateSnapshot validates the Snowflake `CREATE SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT [ IF NOT EXISTS ] <name>
//	  FROM SERVICE <service_name>
//	  VOLUME "<volume_name>"
//	  INSTANCE <instance_id>
//	  [ COMMENT = '<string_literal>']
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
func (v *Validator) ParseCreateSnapshot() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SNAPSHOT") },
		v.ifNotExists,
		v.parseIdentPath,
		// FROM SERVICE <service_name>
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.MatchWord("SERVICE") },
		v.parseIdentPath,
		// VOLUME "<volume_name>"
		func() bool { return v.MatchWord("VOLUME") },
		v.parseIdentPath,
		// INSTANCE <instance_id>
		func() bool { return v.MatchWord("INSTANCE") },
		v.parseScalar,
		// [ COMMENT = ... ] and [ TAG ( ... ) ] in any order.
		func() bool {
			return v.unorderedOnce(v.commentOption(), v.tagClause)
		},
	)
}

// ParseCreateSnapshotPolicy validates the Snowflake `CREATE SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT POLICY [ IF NOT EXISTS ] <name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH RETENTION LOCK ]
//	   [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	   [ EXPIRE_AFTER_DAYS = <days_integer> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateSnapshotPolicy() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	option := func() bool {
		return v.Choice(
			v.tagClause,
			func() bool { return v.phrase("WITH", "RETENTION", "LOCK") },
			v.option("SCHEDULE", v.parseString),
			v.option("EXPIRE_AFTER_DAYS", num),
			v.option("COMMENT", v.parseString),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateSnapshotSet validates the Snowflake `CREATE SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-snapshot-set
//
// Syntax:
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	   FOR [ DYNAMIC ] TABLE <table_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	  FOR SCHEMA <schema_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
//
//	CREATE [ OR REPLACE ] SNAPSHOT SET [ IF NOT EXISTS ] <name>
//	  FOR DATABASE <database_name>
//	   [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	   [ WITH SNAPSHOT POLICY <policy_name> ]
//	   [ COMMENT = <string> ]
func (v *Validator) ParseCreateSnapshotSet() bool {
	forClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("FOR") },
			func() bool {
				return v.Choice(
					// [ DYNAMIC ] TABLE <table_name>
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.MatchWord("DYNAMIC") }) },
							func() bool { return v.MatchWord("TABLE") },
							v.parseIdentPath,
						)
					},
					// SCHEMA <schema_name>
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("SCHEMA") },
							v.parseIdentPath,
						)
					},
					// DATABASE <database_name>
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("DATABASE") },
							v.parseIdentPath,
						)
					},
				)
			},
		)
	}
	option := func() bool {
		return v.Choice(
			v.tagClause,
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("WITH") },
					func() bool { return v.MatchWord("SNAPSHOT") },
					func() bool { return v.MatchWord("POLICY") },
					v.parseIdentPath,
				)
			},
			v.option("COMMENT", v.parseString),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.MatchWord("SET") },
		v.ifNotExists,
		v.parseIdentPath,
		forClause,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateStage validates the Snowflake `CREATE STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stage
//
// Syntax:
//
//	-- Internal stage
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] STAGE [ IF NOT EXISTS ] <internal_stage_name>
//	    internalStageParams
//	    directoryTableParams
//	  [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//
//	-- External stage
//	CREATE [ OR REPLACE ] [ { TEMP | TEMPORARY } ] STAGE [ IF NOT EXISTS ] <external_stage_name>
//	    externalStageParams
//	    directoryTableParams
//	  [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateStage() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// balanced consumes a single ( ... ) group, used for the nested option
	// groups (FILE_FORMAT, COPY_OPTIONS, CREDENTIALS, ENCRYPTION, DIRECTORY),
	// whose contents are themselves order-independent KEY = value lists.
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							t := v.Peek()
							if t.Kind == sqltok.RParen || t.Kind == sqltok.EOF {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	parenGroup := func(key string) Rule {
		return func() bool {
			return v.Sequence(
				func() bool { return v.MatchWord(key) },
				func() bool { return v.MatchOp("=") },
				balanced,
			)
		}
	}
	// Order-independent stage options spanning internal/external/directory
	// params plus the trailing FILE_FORMAT / COMMENT / TAG clauses.
	option := func() bool {
		return v.Choice(
			// Nested ( ... ) option groups.
			parenGroup("FILE_FORMAT"),
			parenGroup("COPY_OPTIONS"),
			parenGroup("CREDENTIALS"),
			parenGroup("ENCRYPTION"),
			parenGroup("DIRECTORY"),
			// Scalar options.
			v.option("URL", v.parseString),
			v.option("STORAGE_INTEGRATION", v.parseIdentPath),
			v.option("ENABLE", v.parseBool),
			v.option("REFRESH_ON_CREATE", v.parseBool),
			v.option("AUTO_REFRESH", v.parseBool),
			v.option("NOTIFICATION_INTEGRATION", v.parseScalar),
			v.option("SNOWFLAKE_FULL", v.parseScalar),
			v.option("SNOWFLAKE_SSE", v.parseScalar),
			v.option("FILE_FORMAT", v.parseScalar),
			v.option("MAX_FILE_DOWNLOAD_BYTES", num),
			v.tagClause,
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		// [ { TEMP | TEMPORARY } ]
		func() bool {
			return v.Optional(func() bool { return v.wordsValue("TEMP", "TEMPORARY")() })
		},
		func() bool { return v.MatchWord("STAGE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(option) },
	)
}

// ParseCreateStorageIntegration validates the Snowflake `CREATE STORAGE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-integration
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE INTEGRATION [IF NOT EXISTS]
//	  <name>
//	  TYPE = { EXTERNAL_STAGE | POSTGRES_EXTERNAL_STORAGE | POSTGRES_INTERNAL_STORAGE }
//	  cloudProviderParams
//	  ENABLED = { TRUE | FALSE }
//	  STORAGE_ALLOWED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] )
//	  [ STORAGE_BLOCKED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	cloudProviderParams (for Amazon S3) ::=
//	  STORAGE_PROVIDER = 'S3'
//	  STORAGE_AWS_ROLE_ARN = '<iam_role>'
//	  [ STORAGE_AWS_EXTERNAL_ID = '<external_id>' ]
//	  [ STORAGE_AWS_OBJECT_ACL = 'bucket-owner-full-control' ]
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	cloudProviderParams (for Google Cloud Storage) ::=
//	  STORAGE_PROVIDER = 'GCS'
//
//	cloudProviderParams (for Microsoft Azure) ::=
//	  STORAGE_PROVIDER = 'AZURE'
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
func (v *Validator) ParseCreateStorageIntegration() bool {
	str := v.parseString
	name := v.parseIdentPath
	prop := func() bool {
		return v.Choice(
			v.option("TYPE", v.parseScalar),
			v.option("ENABLED", v.parseBool),
			v.option("STORAGE_PROVIDER", str),
			v.option("STORAGE_AWS_ROLE_ARN", str),
			v.option("STORAGE_AWS_EXTERNAL_ID", str),
			v.option("STORAGE_AWS_OBJECT_ACL", str),
			v.option("AZURE_TENANT_ID", str),
			v.option("USE_PRIVATELINK_ENDPOINT", v.parseBool),
			v.option("STORAGE_ALLOWED_LOCATIONS", func() bool { return v.parseParenList(str) }),
			v.option("STORAGE_BLOCKED_LOCATIONS", func() bool { return v.parseParenList(str) }),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		name,
		func() bool { return v.ZeroOrMore(prop) },
	)
}

// ParseCreateStorageLifecyclePolicy validates the Snowflake `CREATE STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-lifecycle-policy
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE LIFECYCLE POLICY [ IF NOT EXISTS ] <name>
//	  AS ( <arg_name> <arg_type> [ , ... ] )
//	  RETURNS BOOLEAN -> <body>
//	  [ ARCHIVE_TIER = { COOL | COLD } ]
//	  [ ARCHIVE_FOR_DAYS = <number_of_days> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateStorageLifecyclePolicy() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// arg list: ( <arg_name> <arg_type> [ , ... ] ) — consume a balanced paren run.
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// peekWord reports (without consuming) whether the next token is one of words.
	peekWord := func(words ...string) bool {
		saved := v.save()
		for _, w := range words {
			if v.MatchWord(w) {
				v.restore(saved)
				return true
			}
			v.restore(saved)
		}
		return false
	}
	// RETURNS BOOLEAN -> <body>; consume body permissively until a known option
	// keyword or EOF.
	stop := func() bool {
		return peekWord("ARCHIVE_TIER", "ARCHIVE_FOR_DAYS", "COMMENT", "WITH", "TAG")
	}
	body := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() || stop() {
				return false
			}
			v.advance()
			return true
		})
	}
	opt := func() bool {
		return v.Choice(
			v.option("ARCHIVE_TIER", v.wordsValue("COOL", "COLD")),
			v.option("ARCHIVE_FOR_DAYS", num),
			v.commentOption(),
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("LIFECYCLE") },
		func() bool { return v.MatchWord("POLICY") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		balanced,
		func() bool { return v.MatchWord("RETURNS") },
		func() bool { return v.MatchWord("BOOLEAN") },
		func() bool { return v.MatchOp("-") },
		func() bool { return v.MatchOp(">") },
		func() bool { return !v.AtEnd() && !stop() },
		body,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateStream validates the Snowflake `CREATE STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-stream
//
// Syntax:
//
//	-- Table
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ COPY GRANTS ]
//	  ON TABLE <table_name>
//	  [ { AT | BEFORE } ( { TIMESTAMP => <timestamp> | OFFSET => <time_difference> | STATEMENT => <id> | STREAM => '<name>' } ) ]
//	  [ APPEND_ONLY = TRUE | FALSE ]
//	  [ SHOW_INITIAL_ROWS = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- External table
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ COPY GRANTS ]
//	  ON EXTERNAL TABLE <external_table_name>
//	  [ { AT | BEFORE } ( ... ) ]
//	  [ INSERT_ONLY = TRUE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- View
//	CREATE [ OR REPLACE ] STREAM [IF NOT EXISTS]
//	  <name>
//	  [ COPY GRANTS ]
//	  ON VIEW <view_name>
//	  [ { AT | BEFORE } ( ... ) ]
//	  [ APPEND_ONLY = TRUE | FALSE ]
//	  [ SHOW_INITIAL_ROWS = TRUE | FALSE ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- Also supported: ON EVENT TABLE, ON STAGE, ON DYNAMIC TABLE.
func (v *Validator) ParseCreateStream() bool {
	phraseRule := func(words ...string) Rule {
		return func() bool { return v.phrase(words...) }
	}
	// [ { AT | BEFORE } ( { TIMESTAMP => ... | OFFSET => ... | STATEMENT => ... | STREAM => ... } ) ]
	atBefore := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				v.wordsValue("AT", "BEFORE"),
				func() bool { return v.Match(sqltok.LParen) },
				func() bool {
					return v.Choice(
						v.arrowOption("TIMESTAMP"),
						v.arrowOption("OFFSET"),
						v.arrowOption("STATEMENT"),
						v.arrowOption("STREAM"),
					)
				},
				func() bool { return v.Match(sqltok.RParen) },
			)
		})
	}
	// ON { TABLE | EXTERNAL TABLE | VIEW | EVENT TABLE | STAGE | DYNAMIC TABLE } <name>
	onSource := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("ON") },
			func() bool {
				return v.Choice(
					phraseRule("EXTERNAL", "TABLE"),
					phraseRule("EVENT", "TABLE"),
					phraseRule("DYNAMIC", "TABLE"),
					func() bool { return v.MatchWord("TABLE") },
					func() bool { return v.MatchWord("VIEW") },
					func() bool { return v.MatchWord("STAGE") },
				)
			},
			v.parseIdentPath,
		)
	}
	opt := func() bool {
		return v.Choice(
			v.option("APPEND_ONLY", v.parseBool),
			v.option("SHOW_INITIAL_ROWS", v.parseBool),
			v.option("INSERT_ONLY", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("STREAM") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(v.tagClause) },
		func() bool { return v.Optional(func() bool { return v.phrase("COPY", "GRANTS") }) },
		onSource,
		atBefore,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateStreamlit validates the Snowflake `CREATE STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-streamlit
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STREAMLIT [ IF NOT EXISTS ] <name>
//	  [ FROM <source_location> ]
//	  [ MAIN_FILE = '<filename>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ RUNTIME_NAME = '<runtime_name>' ]
//	  [ COMPUTE_POOL = <compute_pool_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ TITLE = '<app_title>' ]
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [ , ... ] ) ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ SECRETS = ( '<snowflake_secret_name>' = <snowflake_secret> [ , ... ] ) ]
//
//	CREATE [ OR REPLACE ] STREAMLIT [ IF NOT EXISTS ] <name>
//	  ROOT_LOCATION = '<stage_path_and_root_directory>'
//	  MAIN_FILE = '<path_to_main_file_in_root_directory>'
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ TITLE = '<app_title>' ]
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [ , ... ] ) ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
func (v *Validator) ParseCreateStreamlit() bool {
	str := v.parseString
	name := v.parseIdentPath
	opt := func() bool {
		return v.Choice(
			v.option("ROOT_LOCATION", str),
			v.option("MAIN_FILE", str),
			v.option("QUERY_WAREHOUSE", name),
			v.option("RUNTIME_NAME", str),
			v.option("COMPUTE_POOL", name),
			v.option("TITLE", str),
			v.option("IMPORTS", func() bool { return v.parseParenList(str) }),
			v.option("EXTERNAL_ACCESS_INTEGRATIONS", func() bool { return v.parseParenList(name) }),
			v.option("SECRETS", func() bool { return v.parseParenList(v.option2(str, v.parseScalar)) }),
			v.commentOption(),
		)
	}
	// FROM <source_location>
	fromClause := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(
				func() bool { return v.MatchKeyword("FROM") },
				func() bool { return v.Choice(str, name) },
			)
		})
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("STREAMLIT") },
		v.ifNotExists,
		name,
		fromClause,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateTable validates the Snowflake `CREATE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table
//
// Syntax:
//
//	CREATE [ OR REPLACE ]
//	    [ { [ { LOCAL | GLOBAL } ] TEMP | TEMPORARY | VOLATILE | TRANSIENT } ]
//	  TABLE [ IF NOT EXISTS ] <table_name>
//	  (
//	    <col_name> <col_type> [ AS ( <expr> ) ]
//	      [ inlineConstraint ]
//	      [ NOT NULL ]
//	      [ COLLATE '<collation_specification>' ]
//	      [
//	        {
//	          DEFAULT <expr>
//	          | { AUTOINCREMENT | IDENTITY }
//	            [ { ( <start_num> , <step_num> ) | START <num> INCREMENT <num> } ]
//	            [ { ORDER | NOORDER } ]
//	        }
//	      ]
//	      [ [ WITH ] MASKING POLICY <policy_name> [ USING ( <col_name> , <cond_col1> , ... ) ] ]
//	      [ [ WITH ] PROJECTION POLICY <policy_name> ]
//	      [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	      [ COMMENT '<string_literal>' ]
//	    [ , <col_name> <col_type> [ AS ( <expr> ) ] [ ... ] ]
//	    [ , outoflineConstraint [ ... ] ]
//	  )
//	  [ CLUSTER BY ( <expr> [ , <expr> , ... ] ) ]
//	  [ ENABLE_SCHEMA_EVOLUTION = { TRUE | FALSE } ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COPY GRANTS ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ COPY TAGS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ [ WITH ] STORAGE LIFECYCLE POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//
//	-- Variant forms:
//	-- CREATE TABLE ... AS <query>                       (CTAS)
//	-- CREATE TABLE ... USING TEMPLATE <query>
//	-- CREATE TABLE <table_name> LIKE <source_table> [ CLUSTER BY (...) ] [ COPY GRANTS ] [ ... ]
//	-- CREATE TABLE <name> CLONE <source_table> [ { AT | BEFORE } ( ... ) ] [ COPY GRANTS ] [ ... ]
//	-- CREATE [ TRANSIENT ] TABLE <name> FROM ARCHIVE OF <source_table> WHERE <expression>
//
//	-- GET_DDL / Snowsight "Copy DDL" places CLUSTER BY before the column list
//	-- (`CREATE TABLE <name> CLUSTER BY (...) ( <col defs> )`); accepted in both
//	-- positions (issue #776).
func (v *Validator) ParseCreateTable() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	name := v.parseIdentPath
	// Balanced parenthesized run — used for the column/constraint definition list
	// and for CLUSTER BY / policy column lists. Validates only paren balance.
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// AS <query> | USING TEMPLATE <query> — consume the remaining tokens.
	consumeRest := func() bool {
		return v.ZeroOrMore(func() bool {
			if v.AtEnd() {
				return false
			}
			v.advance()
			return true
		})
	}
	asQuery := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("AS") },
			func() bool { return !v.AtEnd() },
			consumeRest,
		)
	}
	usingTemplate := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("USING") },
			func() bool { return v.MatchWord("TEMPLATE") },
			func() bool { return !v.AtEnd() },
			consumeRest,
		)
	}
	// opt (a single trailing table option) is forward-declared here so the LIKE
	// and CLONE branches — which the docs allow to be followed by CLUSTER BY /
	// COPY GRANTS / etc. — can reuse the same option loop that the column-list
	// form does. It is assigned below.
	var opt func() bool
	trailingOpts := func() bool { return v.ZeroOrMore(opt) }
	// LIKE <source_table> [ trailing options ]
	//   The docs allow `[ CLUSTER BY (…) ] [ COPY GRANTS ] [ … ]` after LIKE.
	likeClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("LIKE") },
			name,
			trailingOpts,
		)
	}
	// CLONE <source> [ { AT | BEFORE } ( ... ) ] [ trailing options ]
	//   The docs allow `[ COPY GRANTS ]` (and other options) after CLONE.
	cloneClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("CLONE") },
			name,
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(v.wordsValue("AT", "BEFORE"), balanced)
				})
			},
			trailingOpts,
		)
	}
	// FROM ARCHIVE OF <source> WHERE <expr>
	fromArchive := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("FROM") },
			func() bool { return v.MatchWord("ARCHIVE") },
			func() bool { return v.MatchKeyword("OF") },
			name,
			func() bool { return v.MatchKeyword("WHERE") },
			func() bool { return !v.AtEnd() },
			consumeRest,
		)
	}
	// A trailing table option (order-independent).
	opt = func() bool {
		return v.Choice(
			v.clusterByClause(balanced),
			v.option("ENABLE_SCHEMA_EVOLUTION", v.parseBool),
			v.option("DATA_RETENTION_TIME_IN_DAYS", num),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
			v.option("CHANGE_TRACKING", v.parseBool),
			v.option("DEFAULT_DDL_COLLATION", v.parseString),
			v.option("ICEBERG_DEFAULT_DDL_COLLATION", v.parseString),
			v.option("ERROR_LOGGING", v.parseBool),
			v.option("ROW_TIMESTAMP", v.parseBool),
			func() bool { return v.phrase("COPY", "GRANTS") },
			func() bool { return v.phrase("COPY", "TAGS") },
			v.commentOption(),
			// [ WITH ] ROW ACCESS POLICY <name> ON ( cols )
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("ROW") },
					func() bool { return v.MatchWord("ACCESS") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool { return v.MatchKeyword("ON") },
					balanced,
				)
			},
			// [ WITH ] AGGREGATION POLICY <name> [ ENTITY KEY ( cols ) ]
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("AGGREGATION") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(func() bool { return v.MatchWord("ENTITY") }, func() bool { return v.MatchKeyword("KEY") }, balanced)
						})
					},
				)
			},
			// [ WITH ] JOIN POLICY <name> [ ALLOWED JOIN KEYS ( cols ) ]
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("JOIN") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(func() bool { return v.MatchWord("ALLOWED") }, func() bool { return v.MatchWord("JOIN") }, func() bool { return v.MatchWord("KEYS") }, balanced)
						})
					},
				)
			},
			// [ WITH ] STORAGE LIFECYCLE POLICY <name> ON ( cols )
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("STORAGE") },
					func() bool { return v.MatchWord("LIFECYCLE") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool { return v.MatchKeyword("ON") },
					balanced,
				)
			},
			// WITH CONTACT ( purpose = contact [ , ... ] )
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("CONTACT") },
					func() bool { return v.parseParenList(v.option2(name, name)) },
				)
			},
			v.tagClause,
		)
	}
	// colItemEnd consumes the remainder of one column/constraint entry up to the
	// next top-level comma or the column list's closing paren (nested parens are
	// balanced). It always succeeds — it swallows DEFAULT / NOT NULL / COLLATE /
	// inline constraints / etc. that we don't model in detail.
	colItemEnd := func() bool {
		depth := 0
		for !v.AtEnd() {
			k := v.Peek().Kind
			if depth == 0 && (k == sqltok.Comma || k == sqltok.RParen) {
				return true
			}
			switch k {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
			}
			v.advance()
		}
		return true
	}
	// constraintClause matches an out-of-line table constraint entry.
	constraintClause := func() bool {
		return v.Sequence(
			func() bool {
				return v.Choice(
					func() bool { return v.MatchWord("CONSTRAINT") },
					func() bool { return v.phrase("PRIMARY", "KEY") },
					func() bool { return v.phrase("FOREIGN", "KEY") },
					func() bool { return v.MatchWord("UNIQUE") },
					func() bool { return v.MatchWord("CHECK") },
				)
			},
			colItemEnd,
		)
	}
	// columnDef matches a column definition: a name followed by a (required) data
	// type token — this is what rejects `(dsdfssf)`, a name with no type — then
	// optional type args and the rest of the column spec. The data-type *name* is
	// validated separately by sqleditor.ValidateDataTypes, so it is not re-checked
	// here (avoids double-reporting).
	columnDef := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool {
				if t := v.Peek(); t.Kind.IsIdentLike() {
					v.advance()
					return true
				}
				v.expect("data type")
				return false
			},
			func() bool { return v.Optional(v.consumeBalancedParens) },
			colItemEnd,
		)
	}
	colItem := func() bool { return v.Choice(constraintClause, columnDef) }
	// colList is the full `( <col-def> [ , <col-def> ]* )` definition list — at
	// least one well-formed entry is required.
	colList := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			colItem,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, colItem)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// nameList is a bare `( <name> [ , <name> ]* )` list — only valid as the
	// column-alias list of a CTAS, so it must be followed by AS <query>.
	nameList := func() bool { return v.parseParenList(v.parseIdentPath) }

	body := func() bool {
		return v.Choice(
			// CREATE TABLE ... ( col defs ) [ options ] [ AS <query> ]
			func() bool {
				return v.Sequence(
					colList,
					func() bool { return v.ZeroOrMore(opt) },
					func() bool { return v.Optional(asQuery) },
				)
			},
			// CREATE TABLE ... ( col names ) AS <query>   (CTAS column aliases)
			func() bool { return v.Sequence(nameList, asQuery) },
			// CREATE TABLE ... AS <query>
			asQuery,
			// CREATE TABLE ... USING TEMPLATE <query>
			usingTemplate,
			likeClause,
			cloneClause,
			fromArchive,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		// [ { [ { LOCAL | GLOBAL } ] TEMP | TEMPORARY | VOLATILE | TRANSIENT } ]
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool {
								return v.Optional(func() bool { return v.wordsValue("LOCAL", "GLOBAL")() })
							},
							v.wordsValue("TEMP", "TEMPORARY"),
						)
					},
					func() bool { return v.MatchWord("TEMPORARY") },
					func() bool { return v.MatchWord("VOLATILE") },
					func() bool { return v.MatchWord("TRANSIENT") },
				)
			})
		},
		func() bool { return v.MatchKeyword("TABLE") },
		v.ifNotExists,
		name,
		// Snowflake's GET_DDL / Snowsight "Copy DDL" emits CLUSTER BY between the
		// table name and the column list (`… t cluster by (c)( … )`), and accepts
		// it there even though the syntax diagram only shows CLUSTER BY after the
		// column list. Allow it in both positions. See issue #776.
		func() bool { return v.Optional(v.clusterByClause(balanced)) },
		body,
	)
}

// ParseCreateAlterTableConstraint validates the Snowflake `CREATE | ALTER TABLE CONSTRAINT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-table-constraint
//
// Syntax:
//
//	CREATE TABLE <name> (
//	  <col1_name> <col1_type>  [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH }
//	  [ , <col2_name> <col2_type> [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH } ]
//	  [ , ... ]
//	)
//
//	ALTER TABLE <name> ADD COLUMN
//	  <col_name> <col_type> [ NOT NULL ] { inlineUniquePK | inlineFK | inlineCH }
//
//	inlineUniquePK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY }
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//
//	inlineFK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  [ FOREIGN KEY ]
//	  REFERENCES <ref_table_name> [ ( <ref_col_name> ) ]
//	  [ MATCH { FULL | SIMPLE | PARTIAL } ]
//	  [ ON [ UPDATE { CASCADE | SET NULL | SET DEFAULT | RESTRICT | NO ACTION } ]
//	       [ DELETE { CASCADE | SET NULL | SET DEFAULT | RESTRICT | NO ACTION } ] ]
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//
//	inlineCH ::=
//	  [ CONSTRAINT <constraint_name> ] CHECK ( <expr> )
//	  [ ENABLE { VALIDATE | NOVALIDATE } ]
//
//	outoflineUniquePK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  { UNIQUE | PRIMARY KEY } ( <col_name> [ , <col_name> , ... ] )
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineFK ::=
//	  [ CONSTRAINT <constraint_name> ]
//	  FOREIGN KEY ( <col_name> [ , <col_name> , ... ] )
//	  REFERENCES <ref_table_name> [ ( <ref_col_name> [ , <ref_col_name> , ... ] ) ]
//	  [ MATCH { FULL | SIMPLE | PARTIAL } ]
//	  [ ON [ UPDATE { ... } ] [ DELETE { ... } ] ]
//	  [ [ NOT ] ENFORCED ] [ [ NOT ] DEFERRABLE ]
//	  [ INITIALLY { DEFERRED | IMMEDIATE } ]
//	  [ { ENABLE | DISABLE } ] [ { VALIDATE | NOVALIDATE } ] [ { RELY | NORELY } ]
//	  [ COMMENT '<string_literal>' ]
//
//	outoflineCH ::=
//	  [ CONSTRAINT <constraint_name> ] CHECK ( <expr> )
//	  [ ENABLE { VALIDATE | NOVALIDATE } ]
func (v *Validator) ParseCreateAlterTableConstraint() bool {
	name := v.parseIdentPath
	str := v.parseString
	// dataType matches a single data-type token (NUMBER, VARCHAR, …) plus optional
	// ( precision, scale ) args — the same skeleton ParseCreateTable validates.
	dataType := func() bool {
		return v.Sequence(
			func() bool {
				if t := v.Peek(); t.Kind.IsIdentLike() {
					v.advance()
					return true
				}
				v.expect("data type")
				return false
			},
			func() bool { return v.Optional(v.consumeBalancedParens) },
		)
	}
	// colItemEnd swallows the remainder of one entry up to the next top-level comma
	// or the closing paren, balancing nested parens. It absorbs the column props we
	// don't model in detail (DEFAULT, COLLATE, AUTOINCREMENT, masking policies, …)
	// so a well-formed entry with extra props is never false-rejected.
	colItemEnd := func() bool {
		depth := 0
		for !v.AtEnd() {
			k := v.Peek().Kind
			if depth == 0 && (k == sqltok.Comma || k == sqltok.RParen) {
				return true
			}
			switch k {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				depth--
			}
			v.advance()
		}
		return true
	}

	// [ CONSTRAINT <constraint_name> ]
	constraintName := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(func() bool { return v.MatchWord("CONSTRAINT") }, name)
		})
	}
	// notWord matches `[ NOT ] <word>`, the shape of [NOT] ENFORCED / [NOT] DEFERRABLE.
	notWord := func(word string) Rule {
		return func() bool {
			return v.Sequence(
				func() bool { return v.Optional(func() bool { return v.MatchWord("NOT") }) },
				func() bool { return v.MatchWord(word) },
			)
		}
	}
	// constraintProps matches the trailing property flags shared by UNIQUE / PRIMARY
	// KEY / FOREIGN KEY constraints, each optional. They are accepted in any order
	// (ZeroOrMore over a Choice) rather than the documented fixed order, to avoid
	// false rejections on Snowflake's lenient ordering:
	//   [ [NOT] ENFORCED ] [ [NOT] DEFERRABLE ] [ INITIALLY {DEFERRED|IMMEDIATE} ]
	//   [ {ENABLE|DISABLE} ] [ {VALIDATE|NOVALIDATE} ] [ {RELY|NORELY} ]
	constraintProps := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				notWord("ENFORCED"),
				notWord("DEFERRABLE"),
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("INITIALLY") }, v.wordsValue("DEFERRED", "IMMEDIATE"))
				},
				v.wordsValue("ENABLE", "DISABLE"),
				v.wordsValue("VALIDATE", "NOVALIDATE"),
				v.wordsValue("RELY", "NORELY"),
			)
		})
	}
	// ( <col_name> [ , <col_name> , ... ] )
	colNameList := func() bool { return v.parseParenList(name) }
	// uniqueOrPK matches `{ UNIQUE | PRIMARY KEY }`.
	uniqueOrPK := func() bool {
		return v.Choice(
			func() bool { return v.MatchWord("UNIQUE") },
			func() bool { return v.phrase("PRIMARY", "KEY") },
		)
	}
	// referential action: CASCADE | SET NULL | SET DEFAULT | RESTRICT | NO ACTION
	refAction := func() bool {
		return v.Choice(
			func() bool { return v.MatchWord("CASCADE") },
			func() bool { return v.phrase("SET", "NULL") },
			func() bool { return v.phrase("SET", "DEFAULT") },
			func() bool { return v.MatchWord("RESTRICT") },
			func() bool { return v.phrase("NO", "ACTION") },
		)
	}
	// references matches `REFERENCES <table> [ ( <cols> ) ] [ MATCH {FULL|SIMPLE|
	// PARTIAL} ] [ ON [ UPDATE <action> ] [ DELETE <action> ] ]`.
	references := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("REFERENCES") },
			name,
			func() bool { return v.Optional(colNameList) },
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(func() bool { return v.MatchWord("MATCH") }, v.wordsValue("FULL", "SIMPLE", "PARTIAL"))
				})
			},
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						func() bool { return v.MatchKeyword("ON") },
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(v.wordsValue("UPDATE", "DELETE"), refAction)
							})
						},
					)
				})
			},
		)
	}
	// CHECK ( <expr> ) [ ENABLE {VALIDATE|NOVALIDATE} ] — the expr is consumed as a
	// balanced paren group (it is an arbitrary boolean expression).
	checkConstraint := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("CHECK") },
			v.consumeBalancedParens,
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(func() bool { return v.MatchWord("ENABLE") }, v.wordsValue("VALIDATE", "NOVALIDATE"))
				})
			},
		)
	}
	// constraintComment matches the bare `COMMENT '<string>'` (no '=', unlike the
	// table-option COMMENT) permitted on out-of-line UNIQUE/PK/FK constraints.
	constraintComment := func() bool {
		return v.Optional(func() bool {
			return v.Sequence(func() bool { return v.MatchWord("COMMENT") }, str)
		})
	}

	// inlineConstraint ::= [ CONSTRAINT n ] { {UNIQUE|PRIMARY KEY} props
	//   | [FOREIGN KEY] REFERENCES … props | CHECK (…) } — attached to a column def.
	inlineConstraint := func() bool {
		return v.Sequence(
			constraintName,
			func() bool {
				return v.Choice(
					func() bool { return v.Sequence(uniqueOrPK, constraintProps) },
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.phrase("FOREIGN", "KEY") }) },
							references,
							constraintProps,
						)
					},
					checkConstraint,
				)
			},
		)
	}
	// outOfLineConstraint ::= [ CONSTRAINT n ] { {UNIQUE|PRIMARY KEY} (cols) props
	//   [COMMENT] | FOREIGN KEY (cols) REFERENCES … props [COMMENT] | CHECK (…) }.
	outOfLineConstraint := func() bool {
		return v.Sequence(
			constraintName,
			func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(uniqueOrPK, colNameList, constraintProps, constraintComment)
					},
					func() bool {
						return v.Sequence(
							func() bool { return v.phrase("FOREIGN", "KEY") },
							colNameList,
							references,
							constraintProps,
							constraintComment,
						)
					},
					checkConstraint,
				)
			},
		)
	}

	// One column-list entry: an out-of-line constraint, or a column definition
	// (<name> <type> [NOT NULL] [inline constraint]) with its unmodeled tail
	// swallowed to the entry boundary. Out-of-line is tried first (its leading
	// CONSTRAINT/UNIQUE/PRIMARY/FOREIGN/CHECK words can't begin a column name).
	colItem := func() bool {
		return v.Choice(
			func() bool { return v.Sequence(outOfLineConstraint, colItemEnd) },
			func() bool {
				return v.Sequence(
					name,
					dataType,
					func() bool { return v.Optional(func() bool { return v.phrase("NOT", "NULL") }) },
					func() bool { return v.Optional(inlineConstraint) },
					colItemEnd,
				)
			},
		)
	}
	// colList ::= ( <entry> [ , <entry> ]* ) — at least one well-formed entry.
	colList := func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			colItem,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, colItem)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}

	// CREATE TABLE <name> ( <col defs / table constraints> )
	createForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("CREATE") },
			func() bool { return v.MatchKeyword("TABLE") },
			name,
			colList,
		)
	}
	// ALTER TABLE <name> ADD COLUMN <col> <type> [ NOT NULL ] [ inline constraint ]
	alterForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("ALTER") },
			func() bool { return v.MatchKeyword("TABLE") },
			name,
			func() bool { return v.MatchWord("ADD") },
			func() bool { return v.MatchWord("COLUMN") },
			name,
			dataType,
			func() bool { return v.Optional(func() bool { return v.phrase("NOT", "NULL") }) },
			func() bool { return v.Optional(inlineConstraint) },
			// Swallow any remaining unmodeled column props (DEFAULT, policies, …).
			func() bool {
				return v.ZeroOrMore(func() bool {
					if v.AtEnd() {
						return false
					}
					v.advance()
					return true
				})
			},
		)
	}
	return v.Choice(createForm, alterForm)
}

// ParseCreateTag validates the Snowflake `CREATE TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-tag
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TAG [ IF NOT EXISTS ] <name>
//	    [ ALLOWED_VALUES '<val_1>' [ , '<val_2>' [ , ... ] ] ]
//	    [ PROPAGATE = { ON_DEPENDENCY_AND_DATA_MOVEMENT | ON_DEPENDENCY | ON_DATA_MOVEMENT }
//	      [ ON_CONFLICT = { '<string>' | ALLOWED_VALUES_SEQUENCE } ] ]
//	    [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateTag() bool {
	str := v.parseString
	// ALLOWED_VALUES '<v1>' [ , '<v2>' ... ]
	allowedValues := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("ALLOWED_VALUES") },
			str,
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, str)
				})
			},
		)
	}
	// PROPAGATE = { ... } [ ON_CONFLICT = { '<string>' | ALLOWED_VALUES_SEQUENCE } ]
	propagate := func() bool {
		return v.Sequence(
			v.option("PROPAGATE", v.wordsValue("ON_DEPENDENCY_AND_DATA_MOVEMENT", "ON_DEPENDENCY", "ON_DATA_MOVEMENT")),
			func() bool {
				return v.Optional(v.option("ON_CONFLICT", func() bool {
					return v.Choice(str, func() bool { return v.MatchWord("ALLOWED_VALUES_SEQUENCE") })
				}))
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("TAG") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.Optional(allowedValues) },
		func() bool { return v.Optional(propagate) },
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateTask validates the Snowflake `CREATE TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-task
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TASK [ IF NOT EXISTS ] <name>
//	    [ WITH TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	    [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	    [ { WAREHOUSE = <string> }
//	      | { USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = <string> } ]
//	    [ SCHEDULE = { '<num> { HOURS | MINUTES | SECONDS }'
//	      | 'USING CRON <expr> <time_zone>' } ]
//	    [ CONFIG = <configuration_string> ]
//	    [ OVERLAP_POLICY = { NO_OVERLAP | ALLOW_CHILD_OVERLAP | ALLOW_ALL_OVERLAP } ]
//	    [ <session_parameter> = <value> [ , <session_parameter> = <value> ... ] ]
//	    [ USER_TASK_TIMEOUT_MS = <num> ]
//	    [ SUSPEND_TASK_AFTER_NUM_FAILURES = <num> ]
//	    [ ERROR_INTEGRATION = <integration_name> ]
//	    [ SUCCESS_INTEGRATION = <integration_name> ]
//	    [ LOG_LEVEL = '<log_level>' ]
//	    [ COMMENT = '<string_literal>' ]
//	    [ FINALIZE = <string> ]
//	    [ TASK_AUTO_RETRY_ATTEMPTS = <num> ]
//	    [ USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = <num> ]
//	    [ TARGET_COMPLETION_INTERVAL = '<num> { HOURS | MINUTES | SECONDS }' ]
//	    [ SERVERLESS_TASK_MIN_STATEMENT_SIZE = '{ XSMALL | ... | XXLARGE }' ]
//	    [ SERVERLESS_TASK_MAX_STATEMENT_SIZE = '{ XSMALL | ... | XXLARGE }' ]
//	  [ AFTER <string> [ , <string> , ... ] ]
//	  [ EXECUTE AS USER <user_name> ]
//	  [ WHEN <boolean_expr> ]
//	  AS
//	    <sql>
func (v *Validator) ParseCreateTask() bool {
	str := v.parseString
	name := v.parseIdentPath
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// WITH CONTACT ( purpose = contact [ , ... ] )
	contact := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("WITH") },
			func() bool { return v.MatchWord("CONTACT") },
			func() bool { return v.parseParenList(v.option2(name, name)) },
		)
	}
	// WITH TAG ( ... ) — tagClause already accepts optional WITH.
	opt := func() bool {
		return v.Choice(
			v.tagClause,
			contact,
			v.option("WAREHOUSE", v.parseScalar),
			v.option("USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE", v.parseScalar),
			v.option("SCHEDULE", str),
			v.option("CONFIG", v.parseScalar),
			v.option("OVERLAP_POLICY", v.wordsValue("NO_OVERLAP", "ALLOW_CHILD_OVERLAP", "ALLOW_ALL_OVERLAP")),
			v.option("USER_TASK_TIMEOUT_MS", num),
			v.option("SUSPEND_TASK_AFTER_NUM_FAILURES", num),
			v.option("ERROR_INTEGRATION", name),
			v.option("SUCCESS_INTEGRATION", name),
			v.option("LOG_LEVEL", str),
			v.commentOption(),
			v.option("FINALIZE", v.parseScalar),
			v.option("TASK_AUTO_RETRY_ATTEMPTS", num),
			v.option("USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS", num),
			v.option("TARGET_COMPLETION_INTERVAL", str),
			v.option("SERVERLESS_TASK_MIN_STATEMENT_SIZE", str),
			v.option("SERVERLESS_TASK_MAX_STATEMENT_SIZE", str),
			// generic <session_parameter> = <value> — keep this LAST so named
			// options win first.
			v.option2(name, v.parseScalar),
		)
	}
	// AFTER <task> [ , <task> ... ]
	after := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("AFTER") },
			func() bool { return v.Choice(str, name) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, func() bool { return v.Choice(str, name) })
				})
			},
		)
	}
	// WHEN <boolean_expr> — consume permissively up to AS.
	whenClause := func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("WHEN") },
			func() bool {
				before := v.save()
				v.ZeroOrMore(func() bool {
					saved := v.save()
					if v.MatchKeyword("AS") {
						v.restore(saved)
						return false
					}
					if v.AtEnd() {
						return false
					}
					v.advance()
					return true
				})
				return v.pos > before
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("TASK") },
		v.ifNotExists,
		name,
		func() bool { return v.ZeroOrMore(opt) },
		func() bool { return v.Optional(after) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("EXECUTE") },
					func() bool { return v.MatchKeyword("AS") },
					func() bool { return v.MatchWord("USER") },
					name,
				)
			})
		},
		func() bool { return v.Optional(whenClause) },
		func() bool { return v.MatchKeyword("AS") },
		// <sql> body — consume the remaining tokens permissively.
		func() bool { return !v.AtEnd() },
		func() bool {
			return v.ZeroOrMore(func() bool {
				if v.AtEnd() {
					return false
				}
				v.advance()
				return true
			})
		},
	)
}

// ParseCreateType validates the Snowflake `CREATE TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-type
//
// Syntax:
//
//	CREATE [ OR REPLACE ] TYPE [ IF NOT EXISTS ] <name> AS <type>
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateType() bool {
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("TYPE") },
		v.ifNotExists,
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("AS") },
		// <type> — accept an identifier/path or a parenthesized spec; consume a
		// single type token (possibly dotted) plus an optional ( ... ) suffix.
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					func() bool { return v.Match(sqltok.LParen) },
					func() bool {
						return v.ZeroOrMore(func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						})
					},
					func() bool { return v.Match(sqltok.RParen) },
				)
			})
		},
		func() bool { return v.Optional(v.commentOption()) },
	)
}

// ParseCreateUser validates the Snowflake `CREATE USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-user
//
// Syntax:
//
//	CREATE [ OR REPLACE ] USER [ IF NOT EXISTS ] <name>
//	  [ objectProperties ]
//	  [ objectParams ]
//	  [ sessionParams ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
func (v *Validator) ParseCreateUser() bool {
	name := v.parseIdentPath
	// User properties / params / session params are an open-ended set of
	// `KEY = <value>` assignments; model them generically plus the TAG clause.
	opt := func() bool {
		return v.Choice(
			v.tagClause,
			v.option2(name, v.parseScalar),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("USER") },
		v.ifNotExists,
		name,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateOrAlterVersionedSchema validates the Snowflake `CREATE OR ALTER VERSIONED SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-versioned-schema
//
// Syntax:
//
//	CREATE OR ALTER VERSIONED SCHEMA <name>
//	  [ WITH MANAGED ACCESS ]
//	  [ DATA_RETENTION_TIME_IN_DAYS = ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateOrAlterVersionedSchema() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	opt := func() bool {
		return v.Choice(
			func() bool { return v.phrase("WITH", "MANAGED", "ACCESS") },
			v.option("DATA_RETENTION_TIME_IN_DAYS", num),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
			v.option("DEFAULT_DDL_COLLATION", v.parseString),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchKeyword("OR") },
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("VERSIONED") },
		func() bool { return v.MatchWord("SCHEMA") },
		v.parseIdentPath,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateView validates the Snowflake `CREATE VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-view
//
// Syntax:
//
//	CREATE [ OR REPLACE ] [ SECURE ] [ { [ { LOCAL | GLOBAL } ] TEMP | TEMPORARY | VOLATILE } ] [ RECURSIVE ] VIEW [ IF NOT EXISTS ] <name>
//	  [ ( <column_list> ) ]
//	  [ <col1> [ WITH ] MASKING POLICY <policy_name> [ USING ( <col1> , <cond_col1> , ... ) ]
//	           [ WITH ] PROJECTION POLICY <policy_name>
//	           [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ , <col2> [ ... ] ]
//	  [ [ WITH ] ROW ACCESS POLICY <policy_name> ON ( <col_name> [ , <col_name> ... ] ) ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ COPY GRANTS ]
//	  [ COPY TAGS ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ [ WITH ] AGGREGATION POLICY <policy_name> [ ENTITY KEY ( <col_name> [ , <col_name> ... ] ) ] ]
//	  [ [ WITH ] JOIN POLICY <policy_name> [ ALLOWED JOIN KEYS ( <col_name> [ , ... ] ) ] ]
//	  [ WITH CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  AS <select_statement>
func (v *Validator) ParseCreateView() bool {
	name := v.parseIdentPath
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// Trailing view options before AS — order-independent.
	opt := func() bool {
		return v.Choice(
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("ROW") },
					func() bool { return v.MatchWord("ACCESS") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool { return v.MatchKeyword("ON") },
					balanced,
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("AGGREGATION") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(func() bool { return v.MatchWord("ENTITY") }, func() bool { return v.MatchKeyword("KEY") }, balanced)
						})
					},
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
					func() bool { return v.MatchWord("JOIN") },
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool {
						return v.Optional(func() bool {
							return v.Sequence(func() bool { return v.MatchWord("ALLOWED") }, func() bool { return v.MatchWord("JOIN") }, func() bool { return v.MatchWord("KEYS") }, balanced)
						})
					},
				)
			},
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchKeyword("WITH") },
					func() bool { return v.MatchWord("CONTACT") },
					func() bool { return v.parseParenList(v.option2(name, name)) },
				)
			},
			v.option("CHANGE_TRACKING", v.parseBool),
			func() bool { return v.phrase("COPY", "GRANTS") },
			func() bool { return v.phrase("COPY", "TAGS") },
			v.commentOption(),
			v.tagClause,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURE") }) },
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool {
						return v.Sequence(
							func() bool { return v.Optional(func() bool { return v.wordsValue("LOCAL", "GLOBAL")() }) },
							v.wordsValue("TEMP", "TEMPORARY"),
						)
					},
					func() bool { return v.MatchWord("TEMPORARY") },
					func() bool { return v.MatchWord("VOLATILE") },
				)
			})
		},
		func() bool { return v.Optional(func() bool { return v.MatchWord("RECURSIVE") }) },
		func() bool { return v.MatchWord("VIEW") },
		v.ifNotExists,
		name,
		// Optional ( column_list ) — and any inline per-column policy/tag clauses
		// are subsumed by this balanced run.
		func() bool { return v.Optional(balanced) },
		func() bool { return v.ZeroOrMore(opt) },
		func() bool { return v.MatchKeyword("AS") },
		func() bool { return !v.AtEnd() },
		func() bool {
			return v.ZeroOrMore(func() bool {
				if v.AtEnd() {
					return false
				}
				v.advance()
				return true
			})
		},
	)
}

// ParseCreateWarehouse validates the Snowflake `CREATE WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-warehouse
//
// Syntax:
//
//	CREATE [ OR REPLACE ] WAREHOUSE [ IF NOT EXISTS ] <name>
//	       [ [ WITH ] objectProperties ]
//	       [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ) ]
//	       [ objectParams ]
//
//	Where:
//
//	objectProperties ::=
//	  WAREHOUSE_TYPE = { STANDARD | 'SNOWPARK-OPTIMIZED' | ADAPTIVE }
//	  WAREHOUSE_SIZE = { XSMALL | SMALL | MEDIUM | LARGE | XLARGE | XXLARGE | XXXLARGE | X4LARGE | X5LARGE | X6LARGE }
//	  GENERATION = { '1' | '2' }
//	  RESOURCE_CONSTRAINT = { STANDARD_GEN_1 | STANDARD_GEN_2 | MEMORY_1X | MEMORY_1X_x86 | MEMORY_16X | MEMORY_16X_x86 | MEMORY_64X | MEMORY_64X_x86 }
//	  MAX_CLUSTER_COUNT = <num>
//	  MIN_CLUSTER_COUNT = <num>
//	  SCALING_POLICY = { STANDARD | ECONOMY }
//	  AUTO_SUSPEND = { <num> | NULL }
//	  AUTO_RESUME = { TRUE | FALSE }
//	  INITIALLY_SUSPENDED = { TRUE | FALSE }
//	  RESOURCE_MONITOR = <monitor_name>
//	  COMMENT = '<string_literal>'
//	  ENABLE_QUERY_ACCELERATION = { TRUE | FALSE }
//	  QUERY_ACCELERATION_MAX_SCALE_FACTOR = <num>
//
//	objectParams ::=
//	  MAX_CONCURRENCY_LEVEL = <num>
//	  STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = <num>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
func (v *Validator) ParseCreateWarehouse() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	name := v.parseIdentPath
	opt := func() bool {
		return v.Choice(
			v.option("WAREHOUSE_TYPE", v.parseScalar),
			v.option("WAREHOUSE_SIZE", v.parseScalar),
			v.option("GENERATION", v.parseScalar),
			v.option("RESOURCE_CONSTRAINT", v.parseScalar),
			v.option("MAX_CLUSTER_COUNT", num),
			v.option("MIN_CLUSTER_COUNT", num),
			v.option("SCALING_POLICY", v.wordsValue("STANDARD", "ECONOMY")),
			v.option("AUTO_SUSPEND", v.parseScalar),
			v.option("AUTO_RESUME", v.parseBool),
			v.option("INITIALLY_SUSPENDED", v.parseBool),
			v.option("RESOURCE_MONITOR", name),
			v.commentOption(),
			v.option("ENABLE_QUERY_ACCELERATION", v.parseBool),
			v.option("QUERY_ACCELERATION_MAX_SCALE_FACTOR", num),
			v.option("MAX_CONCURRENCY_LEVEL", num),
			v.option("STATEMENT_QUEUED_TIMEOUT_IN_SECONDS", num),
			v.option("STATEMENT_TIMEOUT_IN_SECONDS", num),
			v.tagClause,
			// generic catch-all for any other session parameter assignment.
			v.option2(name, v.parseScalar),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("WAREHOUSE") },
		v.ifNotExists,
		name,
		// optional leading WITH before the property list.
		func() bool { return v.Optional(func() bool { return v.MatchKeyword("WITH") }) },
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateApplicationService validates the Snowflake `CREATE APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-application-service
//
// Syntax:
//
//	CREATE APPLICATION SERVICE [ IF NOT EXISTS ] <name>
//	  FROM ARTIFACT REPOSITORY <repository_name> PACKAGE <package_name>
//	  [ VERSION <version_alias> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND_SECS = <num> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateApplicationService() bool {
	num := func() bool { return v.Match(sqltok.NumberLit) }
	name := v.parseIdentPath
	opt := func() bool {
		return v.Choice(
			v.option("EXTERNAL_ACCESS_INTEGRATIONS", func() bool { return v.parseParenList(name) }),
			v.option("QUERY_WAREHOUSE", name),
			v.option("AUTO_RESUME", v.parseBool),
			v.option("AUTO_SUSPEND_SECS", num),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.MatchWord("SERVICE") },
		v.ifNotExists,
		name,
		func() bool { return v.MatchKeyword("FROM") },
		func() bool { return v.MatchWord("ARTIFACT") },
		func() bool { return v.MatchWord("REPOSITORY") },
		name,
		func() bool { return v.MatchWord("PACKAGE") },
		name,
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("VERSION") }, name)
			})
		},
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateArtifactRepository validates the Snowflake `CREATE ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-artifact-repository
//
// Syntax:
//
//	CREATE [ OR REPLACE ] ARTIFACT REPOSITORY [ IF NOT EXISTS ] <name>
//	  TYPE = { APPLICATION | PYPI }
//	  [ API_INTEGRATION = '<integration_name>' ]
//	  [ [ WITH ] TAG ( <tag_name> = '<tag_value>' [ , ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateArtifactRepository() bool {
	opt := func() bool {
		return v.Choice(
			v.option("TYPE", v.wordsValue("APPLICATION", "PYPI")),
			v.option("API_INTEGRATION", v.parseScalar),
			v.tagClause,
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("ARTIFACT") },
		func() bool { return v.MatchWord("REPOSITORY") },
		v.ifNotExists,
		v.parseIdentPath,
		// TYPE is required; require at least the option list to start with it but
		// keep ordering flexible.
		opt,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateCatalogIntegrationDeltaSharing validates the Snowflake `CREATE CATALOG INTEGRATION (Delta Sharing)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-delta-sharing
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = DELTA_SHARING
//	  TABLE_FORMAT = DELTA
//	  REST_CONFIG = (
//	    CATALOG_URI = '<delta_sharing_endpoint_url>'
//	    CATALOG_NAME = 'shares/<share_name>'
//	    ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS
//	  )
//	  REST_AUTHENTICATION = (
//	    restAuthenticationParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	Where restAuthenticationParams is one of:
//
//	restAuthenticationParams (for Bearer token) ::=
//	  TYPE = BEARER
//	  BEARER_TOKEN = '<bearer_token>'
//
//	restAuthenticationParams (for OIDC) ::=
//	  TYPE = OIDC
//	  OIDC_AUDIENCE = '<audience>'
//
//	restAuthenticationParams (for OAuth) ::=
//	  TYPE = OAUTH
//	  OAUTH_CLIENT_ID = '<oauth_client_id>'
//	  OAUTH_CLIENT_SECRET = '<oauth_client_secret>'
//	  OAUTH_TOKEN_URI = 'https://<token_server_uri>'
func (v *Validator) ParseCreateCatalogIntegrationDeltaSharing() bool {
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	opt := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.parseScalar),
			v.option("TABLE_FORMAT", v.parseScalar),
			v.option("REST_CONFIG", balanced),
			v.option("REST_AUTHENTICATION", balanced),
			v.option("ENABLED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		opt,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateCatalogIntegrationSnowflakePostgres validates the Snowflake `CREATE CATALOG INTEGRATION (Snowflake Postgres)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-catalog-integration-snowflake-postgres
//
// Syntax:
//
//	CREATE [ OR REPLACE ] CATALOG INTEGRATION [ IF NOT EXISTS ] <name>
//	  CATALOG_SOURCE = SNOWFLAKE_POSTGRES
//	  TABLE_FORMAT = ICEBERG
//	  [ CATALOG_NAMESPACE = '<namespace>' ]
//	  REST_CONFIG = (
//	    restConfigParams
//	  )
//	  ENABLED = { TRUE | FALSE }
//	  [ COMMENT = '<string_literal>' ]
//
//	Where:
//
//	restConfigParams ::=
//	  POSTGRES_INSTANCE = '<instance_name>'
//	  ACCESS_DELEGATION_MODE = VENDED_CREDENTIALS
//	  [ CATALOG_NAME = '<database_name>' ]
func (v *Validator) ParseCreateCatalogIntegrationSnowflakePostgres() bool {
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	opt := func() bool {
		return v.Choice(
			v.option("CATALOG_SOURCE", v.parseScalar),
			v.option("TABLE_FORMAT", v.parseScalar),
			v.option("CATALOG_NAMESPACE", v.parseString),
			v.option("REST_CONFIG", balanced),
			v.option("ENABLED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("CATALOG") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		opt,
		func() bool { return v.ZeroOrMore(opt) },
	)
}

// ParseCreateEventRoutingTable validates the Snowflake `CREATE EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-event-routing-table
//
// Syntax:
//
//	CREATE EVENT ROUTING TABLE <table_name>
//	   WITH RULES
//	    {rule name} = (REGION_GROUP={region group}, REGIONS=('{region1}', '{region2}', ...), DESTINATION_ACCOUNT = {organization}.{account_name})
//	    ...
func (v *Validator) ParseCreateEventRoutingTable() bool {
	var balanced func() bool
	balanced = func() bool {
		return v.Sequence(
			func() bool { return v.Match(sqltok.LParen) },
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Choice(
						balanced,
						func() bool {
							if v.AtEnd() || v.Peek().Kind == sqltok.RParen {
								return false
							}
							v.advance()
							return true
						},
					)
				})
			},
			func() bool { return v.Match(sqltok.RParen) },
		)
	}
	// <rule_name> = ( <params> )
	rule := func() bool {
		return v.Sequence(
			v.parseIdentPath,
			func() bool { return v.MatchOp("=") },
			balanced,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		func() bool { return v.MatchWord("EVENT") },
		func() bool { return v.MatchWord("ROUTING") },
		func() bool { return v.MatchKeyword("TABLE") },
		v.parseIdentPath,
		func() bool { return v.MatchKeyword("WITH") },
		func() bool { return v.MatchWord("RULES") },
		rule,
		func() bool { return v.ZeroOrMore(rule) },
	)
}

// ParseCreateStorageIntegrationPostgresInternal validates the Snowflake `CREATE STORAGE INTEGRATION (Postgres internal)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/create-storage-integration-postgres-internal
//
// Syntax:
//
//	CREATE [ OR REPLACE ] STORAGE INTEGRATION [ IF NOT EXISTS ] <name>
//	  TYPE = POSTGRES_INTERNAL_STORAGE
//	  POSTGRES_INSTANCE = '<instance_name>'
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseCreateStorageIntegrationPostgresInternal() bool {
	opt := func() bool {
		return v.Choice(
			v.option("TYPE", v.parseScalar),
			v.option("POSTGRES_INSTANCE", v.parseString),
			v.option("ENABLED", v.parseBool),
			v.commentOption(),
		)
	}
	return v.Sequence(
		func() bool { return v.MatchKeyword("CREATE") },
		v.orReplace,
		func() bool { return v.MatchWord("STORAGE") },
		func() bool { return v.MatchWord("INTEGRATION") },
		v.ifNotExists,
		v.parseIdentPath,
		opt,
		func() bool { return v.ZeroOrMore(opt) },
	)
}
