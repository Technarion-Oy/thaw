package sqlgrammar

import "thaw/internal/sqltok"

// ALTER commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseAlterObj validates the Snowflake `ALTER <object>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseAlterObj() bool {
	// Generic `ALTER <object-words> [IF EXISTS] <name> <action>`.
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		v.parseIdentPath, // object kind word(s) / name
		v.consumeRest,
	)
}

// ParseAlterAccount validates the Snowflake `ALTER ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-account
//
// Syntax:
//
//	ALTER ACCOUNT SET { [ accountProperties ] | [ accountParams ] | [ objectParams ] | [ sessionParams ] }
//
//	ALTER ACCOUNT UNSET <param_name> [ , ... ]
//
//	ALTER ACCOUNT SET RESOURCE_MONITOR = <monitor_name>
//
//	ALTER ACCOUNT ADD ORGANIZATION USER GROUP <group_name>
//	ALTER ACCOUNT REMOVE ORGANIZATION USER GROUP <group_name>
//
//	ALTER ACCOUNT SET { AUTHENTICATION | SESSION } POLICY <policy_name> [ { FOR ALL PERSON USERS | FOR ALL SERVICE USERS } ] [ FORCE ]
//
//	ALTER ACCOUNT UNSET { AUTHENTICATION | SESSION } POLICY [ { FOR ALL PERSON USERS | FOR ALL SERVICE USERS } ]
//
//	ALTER ACCOUNT SET FEATURE POLICY <policy_name> FOR ALL APPLICATIONS [ FORCE ]
//
//	ALTER ACCOUNT UNSET FEATURE POLICY FOR ALL APPLICATIONS
//
//	ALTER ACCOUNT SET MAINTENANCE POLICY <policy_name> [ FORCE ] FOR ALL APPLICATIONS
//
//	ALTER ACCOUNT UNSET MAINTENANCE POLICY FOR ALL APPLICATIONS
//
//	ALTER ACCOUNT SET { PACKAGES | PASSWORD } POLICY <policy_name> [ FORCE ]
//
//	ALTER ACCOUNT UNSET { PACKAGES | PASSWORD } POLICY
//
//	ALTER ACCOUNT SET CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ]
//
//	ALTER ACCOUNT UNSET CONTACT <purpose>
//
//	ALTER ACCOUNT SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ACCOUNT UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER ACCOUNT <name> SET IS_ORG_ADMIN = { TRUE | FALSE }
//
//	ALTER ACCOUNT <name> SET EDITION = { 'STANDARD' | 'ENTERPRISE' | 'BUSINESS_CRITICAL' }
//
//	ALTER ACCOUNT <name> RENAME TO <new_name> [ SAVE_OLD_URL = { TRUE | FALSE } ]
//
//	ALTER ACCOUNT <name> DROP OLD URL
//
//	ALTER ACCOUNT <name> DROP OLD ORGANIZATION URL
func (v *Validator) ParseAlterAccount() bool {
	name := v.parseIdentPath
	// FOR ALL { PERSON | SERVICE } USERS
	forAllUsers := func() bool {
		return v.Sequence(
			func() bool { return v.phrase("FOR", "ALL") },
			v.wordsValue("PERSON", "SERVICE"),
			func() bool { return v.MatchWord("USERS") },
		)
	}
	forAllApps := func() bool { return v.phrase("FOR", "ALL", "APPLICATIONS") }
	force := func() bool { return v.Optional(func() bool { return v.MatchWord("FORCE") }) }
	// A `<key> = <value>` assignment with a permissive value (the account/object/
	// session param bag and the CONTACT / TAG lists are all this shape).
	assign := func() bool {
		return v.Sequence(
			name,
			func() bool { return v.MatchOp("=") },
			func() bool { return v.Choice(v.parseScalar, v.consumeBalancedParens) },
		)
	}
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(item, func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
				})
			})
		}
	}
	assignList := commaList(assign)
	nameList := commaList(name)

	// SET <target>: the structured policy / CONTACT / TAG forms, then the generic
	// `<param> = <value> [ , ... ]` bag (RESOURCE_MONITOR, IS_ORG_ADMIN, EDITION,
	// and the account / object / session params).
	setTarget := func() bool {
		return v.Choice(
			// { AUTHENTICATION | SESSION } POLICY <name> [ FOR ALL {PERSON|SERVICE} USERS ] [ FORCE ]
			func() bool {
				return v.Sequence(
					v.wordsValue("AUTHENTICATION", "SESSION"),
					func() bool { return v.MatchWord("POLICY") },
					name,
					func() bool { return v.Optional(forAllUsers) },
					force,
				)
			},
			// FEATURE POLICY <name> FOR ALL APPLICATIONS [ FORCE ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("FEATURE", "POLICY") }, name, forAllApps, force)
			},
			// MAINTENANCE POLICY <name> [ FORCE ] FOR ALL APPLICATIONS
			func() bool {
				return v.Sequence(func() bool { return v.phrase("MAINTENANCE", "POLICY") }, name, force, forAllApps)
			},
			// { PACKAGES | PASSWORD } POLICY <name> [ FORCE ]
			func() bool {
				return v.Sequence(
					v.wordsValue("PACKAGES", "PASSWORD"),
					func() bool { return v.MatchWord("POLICY") },
					name, force,
				)
			},
			// CONTACT <purpose> = <contact_name> [ , ... ]  |  TAG <tag> = '<value>' [ , ... ]
			func() bool { return v.Sequence(v.wordsValue("CONTACT", "TAG"), assignList) },
			// RESOURCE_MONITOR / IS_ORG_ADMIN / EDITION / account / object / session params.
			assignList,
		)
	}
	// UNSET <target>: the structured policy / CONTACT / TAG forms, then the generic
	// `<param_name> [ , ... ]` list.
	unsetTarget := func() bool {
		return v.Choice(
			func() bool {
				return v.Sequence(
					v.wordsValue("AUTHENTICATION", "SESSION"),
					func() bool { return v.MatchWord("POLICY") },
					func() bool { return v.Optional(forAllUsers) },
				)
			},
			func() bool { return v.Sequence(func() bool { return v.phrase("FEATURE", "POLICY") }, forAllApps) },
			func() bool { return v.Sequence(func() bool { return v.phrase("MAINTENANCE", "POLICY") }, forAllApps) },
			func() bool {
				return v.Sequence(v.wordsValue("PACKAGES", "PASSWORD"), func() bool { return v.MatchWord("POLICY") })
			},
			func() bool { return v.Sequence(func() bool { return v.MatchWord("CONTACT") }, name) },
			func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, nameList) },
			nameList,
		)
	}

	action := func() bool {
		return v.Choice(
			// RENAME TO <new_name> [ SAVE_OLD_URL = { TRUE | FALSE } ]
			func() bool {
				return v.Sequence(
					func() bool { return v.phrase("RENAME", "TO") },
					name,
					func() bool { return v.Optional(v.option("SAVE_OLD_URL", v.parseBool)) },
				)
			},
			// DROP OLD [ ORGANIZATION ] URL
			func() bool {
				return v.Sequence(
					func() bool { return v.MatchWord("DROP") },
					func() bool { return v.MatchWord("OLD") },
					func() bool { return v.Optional(func() bool { return v.MatchWord("ORGANIZATION") }) },
					func() bool { return v.MatchWord("URL") },
				)
			},
			// { ADD | REMOVE } ORGANIZATION USER GROUP <group_name>
			func() bool {
				return v.Sequence(
					v.wordsValue("ADD", "REMOVE"),
					func() bool { return v.phrase("ORGANIZATION", "USER", "GROUP") },
					name,
				)
			},
			func() bool { return v.Sequence(func() bool { return v.MatchWord("SET") }, setTarget) },
			func() bool { return v.Sequence(func() bool { return v.MatchWord("UNSET") }, unsetTarget) },
		)
	}

	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("ACCOUNT") },
		// Account-level form (action directly) or per-account form (`<name> <action>`:
		// RENAME TO, DROP OLD URL, SET IS_ORG_ADMIN / EDITION, …).
		func() bool {
			return v.Choice(
				action,
				func() bool { return v.Sequence(name, action) },
			)
		},
	)
}

// ParseAlterAgent validates the Snowflake `ALTER AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-agent
//
// Syntax:
//
//	ALTER AGENT <name> SET
//	  [ COMMENT = '<string>' ]
//	  [ PROFILE = '<string>' ]
//
//	ALTER AGENT <name> MODIFY LIVE VERSION SET SPECIFICATION = <specification>
func (v *Validator) ParseAlterAgent() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("AGENT") },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// SET [ COMMENT = … ] [ PROFILE = … ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SET") },
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Choice(
									v.commentOption(),
									v.option("PROFILE", v.parseString),
								)
							})
						},
					)
				},
				// MODIFY LIVE VERSION SET SPECIFICATION = <spec>
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("MODIFY", "LIVE", "VERSION", "SET") },
						v.option("SPECIFICATION", v.parseScalar),
					)
				},
			)
		},
	)
}

// ParseAlterAggregationPolicy validates the Snowflake `ALTER AGGREGATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-aggregation-policy
//
// Syntax:
//
//	ALTER AGGREGATION POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER AGGREGATION POLICY [ IF EXISTS ] <name> SET BODY -> <expression>
//
//	ALTER AGGREGATION POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER AGGREGATION POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER AGGREGATION POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER AGGREGATION POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterAggregationPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("AGGREGATION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET / UNSET … (BODY -> expr, TAG, COMMENT) — free-form remainder.
				func() bool {
					return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest)
				},
			)
		},
	)
}

// ParseAlterAlert validates the Snowflake `ALTER ALERT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-alert
//
// Syntax:
//
//	ALTER ALERT [ IF EXISTS ] <name> { RESUME | SUSPEND };
//
//	ALTER ALERT [ IF EXISTS ] <name> SET
//	  [ WAREHOUSE = <string> ]
//	  [ SCHEDULE = '{ <number> MINUTE | USING CRON <expr> <time_zone> }' ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ CONFIG = '<configuration_string>' ]
//	  [ RUNBOOK = '<string_literal>' ]
//	  [ SUSPEND_ALERT_AFTER_NUM_FAILURES = <number> ]
//
//	ALTER ALERT [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ALERT [ IF EXISTS ] <name> UNSET
//	  [ WAREHOUSE ]
//	  [ COMMENT ]
//	  [ CONFIG ]
//	  [ RUNBOOK ]
//	  [ SUSPEND_ALERT_AFTER_NUM_FAILURES ]
//
//	ALTER ALERT <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER ALERT [ IF EXISTS ] <name> MODIFY CONDITION EXISTS (<condition>)
//
//	ALTER ALERT [ IF EXISTS ] <name> MODIFY ACTION <action>
func (v *Validator) ParseAlterAlert() bool {
	name := v.parseIdentPath
	num := func() bool { return v.Match(sqltok.NumberLit) }
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(item, func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
				})
			})
		}
	}
	// TAG <tag> = '<value>' assignment.
	tagAssign := func() bool {
		return v.Sequence(name, func() bool { return v.MatchOp("=") }, v.parseString)
	}
	// SET option (closed set).
	setOption := func() bool {
		return v.Choice(
			v.option("WAREHOUSE", v.parseScalar),
			v.option("SCHEDULE", v.parseString),
			v.commentOption(),
			v.option("CONFIG", v.parseString),
			v.option("RUNBOOK", v.parseString),
			v.option("SUSPEND_ALERT_AFTER_NUM_FAILURES", num),
		)
	}
	setForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("SET") },
			func() bool {
				return v.Choice(
					// SET TAG <tag> = '<value>' [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(tagAssign)) },
					// SET <option> [ <option> ... ]  (at least one)
					func() bool { return v.Sequence(setOption, func() bool { return v.ZeroOrMore(setOption) }) },
				)
			},
		)
	}
	unsetForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("UNSET") },
			func() bool {
				return v.Choice(
					// UNSET TAG <tag> [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(name)) },
					// UNSET { WAREHOUSE | COMMENT | CONFIG | RUNBOOK | SUSPEND_ALERT_AFTER_NUM_FAILURES } [ , ... ]
					commaList(v.wordsValue("WAREHOUSE", "COMMENT", "CONFIG", "RUNBOOK", "SUSPEND_ALERT_AFTER_NUM_FAILURES")),
				)
			},
		)
	}
	modifyForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("MODIFY") },
			func() bool {
				return v.Choice(
					// CONDITION EXISTS ( <condition> )
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("CONDITION") },
							func() bool { return v.MatchWord("EXISTS") },
							v.consumeBalancedParens,
						)
					},
					// ACTION <action> — the action body is free-form.
					func() bool {
						return v.Sequence(func() bool { return v.MatchWord("ACTION") }, v.consumeRest)
					},
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("ALERT") },
		func() bool { return v.ifExists() },
		name,
		func() bool {
			return v.Choice(
				v.wordsValue("RESUME", "SUSPEND"),
				setForm,
				unsetForm,
				modifyForm,
			)
		},
	)
}

// ParseAlterApiIntegration validates the Snowflake `ALTER API INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-api-integration
//
// Syntax:
//
//	ALTER [ API ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ API_AWS_ROLE_ARN = '<iam_role>' ]
//	  [ AZURE_AD_APPLICATION_ID = '<azure_application_id>' ]
//	  [ API_KEY = '<api_key>' ]
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ API_ALLOWED_PREFIXES = ('<...>') ]
//	  [ API_BLOCKED_PREFIXES = ('<...>') ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = ( { <secret_name> [, <secret_name>, ... ] } ) | all | none ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ API ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ API ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ API ] INTEGRATION [ IF EXISTS ] <name>  UNSET {
//	                                                      API_KEY              |
//	                                                      ENABLED              |
//	                                                      API_BLOCKED_PREFIXES |
//	                                                      COMMENT
//	                                                      }
//	                                                      [ , ... ]
func (v *Validator) ParseAlterApiIntegration() bool {
	name := v.parseIdentPath
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(item, func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
				})
			})
		}
	}
	tagAssign := func() bool {
		return v.Sequence(name, func() bool { return v.MatchOp("=") }, v.parseString)
	}
	// ALLOWED_AUTHENTICATION_SECRETS = ( <secret> [, ...] ) | ALL | NONE
	secretsVal := func() bool {
		return v.Choice(
			v.consumeBalancedParens,
			func() bool { return v.MatchWord("ALL") },
			func() bool { return v.MatchWord("NONE") },
		)
	}
	setOption := func() bool {
		return v.Choice(
			v.option("API_AWS_ROLE_ARN", v.parseString),
			v.option("AZURE_AD_APPLICATION_ID", v.parseString),
			v.option("API_KEY", v.parseString),
			v.option("ENABLED", v.parseBool),
			v.option("API_ALLOWED_PREFIXES", v.consumeBalancedParens),
			v.option("API_BLOCKED_PREFIXES", v.consumeBalancedParens),
			v.option("ALLOWED_AUTHENTICATION_SECRETS", secretsVal),
			v.commentOption(),
		)
	}
	setForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("SET") },
			func() bool {
				return v.Choice(
					// SET TAG <tag> = '<value>' [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(tagAssign)) },
					// SET <option> [ <option> ... ]  (at least one)
					func() bool { return v.Sequence(setOption, func() bool { return v.ZeroOrMore(setOption) }) },
				)
			},
		)
	}
	unsetForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("UNSET") },
			func() bool {
				return v.Choice(
					// UNSET TAG <tag> [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(name)) },
					// UNSET { API_KEY | ENABLED | API_BLOCKED_PREFIXES | COMMENT } [ , ... ]
					commaList(v.wordsValue("API_KEY", "ENABLED", "API_BLOCKED_PREFIXES", "COMMENT")),
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		// [ API ] INTEGRATION
		func() bool { return v.Optional(func() bool { return v.MatchWord("API") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		name,
		func() bool { return v.Choice(setForm, unsetForm) },
	)
}

// ParseAlterApplication validates the Snowflake `ALTER APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application
//
// Syntax:
//
//	ALTER APPLICATION [ IF EXISTS ] <name> SET
//	  [ COMMENT = '<string-literal>' ]
//	  [ SHARE_EVENTS_WITH_PROVIDER = { TRUE | FALSE } ]
//	  [ DEBUG_MODE = { TRUE | FALSE } ]
//
//	ALTER APPLICATION [ IF EXISTS ] <name> UNSET
//	  [ COMMENT ]
//	  [ SHARE_EVENTS_WITH_PROVIDER ]
//	  [ DEBUG_MODE ]
//
//	ALTER APPLICATION [ IF EXISTS ] <name> RENAME TO <new_app_name>
//
//	ALTER APPLICATION <name> SET FEATURE POLICY <policy_name> [ FORCE ]
//
//	ALTER APPLICATION <name> UNSET FEATURE POLICY;
//
//	ALTER APPLICATION <name> SET MAINTENANCE POLICY <policy_name> [ FORCE ]
//
//	ALTER APPLICATION <name> UNSET MAINTENANCE POLICY
//
//	ALTER APPLICATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER APPLICATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER APPLICATION <name> SET SHARED TELEMETRY EVENTS ('<event_definition' [ , <event_definition>, ...])
//
//	ALTER APPLICATION <name> SET AUTHORIZE_TELEMETRY_EVENT_SHARING = { TRUE | FALSE }
//
//	ALTER APPLICATION <name> UNSET REFERENCES [ ( '<reference_name>' [ , '<reference_alias>' ] ) ]
//
//	ALTER APPLICATION <name> UPGRADE
//
//	ALTER APPLICATION <name> UPGRADE USING VERSION <version_name> [ PATCH <patch_num> ]
//
//	ALTER APPLICATION <name> UPGRADE USING <path_to_stage>
func (v *Validator) ParseAlterApplication() bool {
	name := v.parseIdentPath
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(item, func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
				})
			})
		}
	}
	force := func() bool { return v.Optional(func() bool { return v.MatchWord("FORCE") }) }
	tagAssign := func() bool {
		return v.Sequence(name, func() bool { return v.MatchOp("=") }, v.parseString)
	}
	setOption := func() bool {
		return v.Choice(
			v.commentOption(),
			v.option("SHARE_EVENTS_WITH_PROVIDER", v.parseBool),
			v.option("DEBUG_MODE", v.parseBool),
			v.option("AUTHORIZE_TELEMETRY_EVENT_SHARING", v.parseBool),
		)
	}
	setForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("SET") },
			func() bool {
				return v.Choice(
					// FEATURE POLICY <name> [ FORCE ]
					func() bool { return v.Sequence(func() bool { return v.phrase("FEATURE", "POLICY") }, name, force) },
					// MAINTENANCE POLICY <name> [ FORCE ]
					func() bool { return v.Sequence(func() bool { return v.phrase("MAINTENANCE", "POLICY") }, name, force) },
					// SHARED TELEMETRY EVENTS ( <event> [ , ... ] )
					func() bool {
						return v.Sequence(func() bool { return v.phrase("SHARED", "TELEMETRY", "EVENTS") }, v.consumeBalancedParens)
					},
					// TAG <tag> = '<value>' [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(tagAssign)) },
					// COMMENT / SHARE_EVENTS_WITH_PROVIDER / DEBUG_MODE /
					// AUTHORIZE_TELEMETRY_EVENT_SHARING (at least one).
					func() bool { return v.Sequence(setOption, func() bool { return v.ZeroOrMore(setOption) }) },
				)
			},
		)
	}
	unsetForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("UNSET") },
			func() bool {
				return v.Choice(
					func() bool { return v.phrase("FEATURE", "POLICY") },
					func() bool { return v.phrase("MAINTENANCE", "POLICY") },
					// UNSET TAG <tag> [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(name)) },
					// UNSET REFERENCES [ ( '<reference_name>' [ , '<reference_alias>' ] ) ]
					func() bool {
						return v.Sequence(
							func() bool { return v.MatchWord("REFERENCES") },
							func() bool { return v.Optional(v.consumeBalancedParens) },
						)
					},
					// UNSET { COMMENT | SHARE_EVENTS_WITH_PROVIDER | DEBUG_MODE } [ , ... ]
					commaList(v.wordsValue("COMMENT", "SHARE_EVENTS_WITH_PROVIDER", "DEBUG_MODE")),
				)
			},
		)
	}
	// UPGRADE [ USING VERSION <version> [ PATCH <patch_num> ] | USING <path_to_stage> ]
	upgradeForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("UPGRADE") },
			func() bool {
				return v.Optional(func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("USING") },
						func() bool {
							return v.Choice(
								func() bool {
									return v.Sequence(
										func() bool { return v.MatchWord("VERSION") },
										name,
										func() bool {
											return v.Optional(func() bool {
												return v.Sequence(func() bool { return v.MatchWord("PATCH") }, v.parseScalar)
											})
										},
									)
								},
								// <path_to_stage> — free-form.
								v.consumeRest,
							)
						},
					)
				})
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.ifExists() },
		name,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				upgradeForm,
				setForm,
				unsetForm,
			)
		},
	)
}

// ParseAlterApplicationDropSpecification validates the Snowflake `ALTER APPLICATION DROP SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-drop-app-spec
//
// Syntax:
//
//	ALTER APPLICATION DROP SPECIFICATION <app_spec_name>;
func (v *Validator) ParseAlterApplicationDropSpecification() bool {
	// ALTER APPLICATION DROP SPECIFICATION <app_spec_name>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "DROP", "SPECIFICATION") },
		v.parseIdentPath,
	)
}

// ParseAlterApplicationDropConfigurationDefinition validates the Snowflake `ALTER APPLICATION DROP CONFIGURATION DEFINITION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-drop-configuration-definition
//
// Syntax:
//
//	ALTER APPLICATION DROP CONFIGURATION DEFINITION {config};
func (v *Validator) ParseAlterApplicationDropConfigurationDefinition() bool {
	// ALTER APPLICATION DROP CONFIGURATION DEFINITION <config>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "DROP", "CONFIGURATION", "DEFINITION") },
		v.parseIdentPath,
	)
}

// ParseAlterApplicationPackage validates the Snowflake `ALTER APPLICATION PACKAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package
//
// Syntax:
//
//	ALTER APPLICATION PACKAGE [ IF EXISTS ] <name> SET
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	  [ COMMENT = <string-literal> ]
//	  [ DISTRIBUTION = { INTERNAL | EXTERNAL } ]
//	  [ MULTIPLE_INSTANCES = TRUE ]
//	  [ ENABLE_RELEASE_CHANNELS = TRUE ]
//	  [ LISTING_AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ AUTOMATIC_APPLICATION_MAINTENANCE = { TRUE | FALSE } ]
//
//	ALTER APPLICATION PACKAGE [ IF EXISTS ] <name> UNSET
//	  [ DATA_RETENTION_TIME_IN_DAYS ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS ]
//	  [ DEFAULT_DDL_COLLATION ]
//	  [ COMMENT = <string-literal> ]
//	  [ DISTRIBUTION = { INTERNAL | EXTERNAL } ]
//
//	ALTER APPLICATION PACKAGE <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER APPLICATION PACKAGE <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterApplicationPackage() bool {
	name := v.parseIdentPath
	num := func() bool { return v.Match(sqltok.NumberLit) }
	commaList := func(item Rule) Rule {
		return func() bool {
			return v.Sequence(item, func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, item)
				})
			})
		}
	}
	tagAssign := func() bool {
		return v.Sequence(name, func() bool { return v.MatchOp("=") }, v.parseString)
	}
	setOption := func() bool {
		return v.Choice(
			v.option("DATA_RETENTION_TIME_IN_DAYS", num),
			v.option("MAX_DATA_EXTENSION_TIME_IN_DAYS", num),
			v.option("DEFAULT_DDL_COLLATION", v.parseString),
			v.commentOption(),
			v.option("DISTRIBUTION", v.wordsValue("INTERNAL", "EXTERNAL")),
			v.option("MULTIPLE_INSTANCES", v.parseBool),
			v.option("ENABLE_RELEASE_CHANNELS", v.parseBool),
			v.option("LISTING_AUTO_REFRESH", v.parseBool),
			v.option("AUTOMATIC_APPLICATION_MAINTENANCE", v.parseBool),
		)
	}
	setForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("SET") },
			func() bool {
				return v.Choice(
					// SET TAG <tag> = '<value>' [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(tagAssign)) },
					// SET <option> [ <option> ... ]  (at least one)
					func() bool { return v.Sequence(setOption, func() bool { return v.ZeroOrMore(setOption) }) },
				)
			},
		)
	}
	unsetForm := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("UNSET") },
			func() bool {
				return v.Choice(
					// UNSET TAG <tag> [ , ... ]
					func() bool { return v.Sequence(func() bool { return v.MatchWord("TAG") }, commaList(name)) },
					// UNSET { DATA_RETENTION_TIME_IN_DAYS | MAX_DATA_EXTENSION_TIME_IN_DAYS |
					//         DEFAULT_DDL_COLLATION | COMMENT | DISTRIBUTION } [ , ... ]
					commaList(v.wordsValue(
						"DATA_RETENTION_TIME_IN_DAYS", "MAX_DATA_EXTENSION_TIME_IN_DAYS",
						"DEFAULT_DDL_COLLATION", "COMMENT", "DISTRIBUTION")),
				)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		func() bool { return v.ifExists() },
		name,
		func() bool { return v.Choice(setForm, unsetForm) },
	)
}

// ParseAlterApplicationPackageModifyReleaseChannel validates the Snowflake `ALTER APPLICATION PACKAGE MODIFY RELEASE CHANNEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-release-channel
//
// Syntax:
//
//	ALTER APPLICATION PACKAGE <name>
//	  MODIFY RELEASE CHANNEL <release_channel>
//	  SET DEFAULT RELEASE DIRECTIVE
//	  VERSION = <version_identifier>
//	  PATCH = <patch_num>
//	  [ UPGRADE_AFTER = '<timestamp>' ]
//	  [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	  [ UPGRADE_DEADLINE = '<timestamp>' ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  MODIFY RELEASE CHANNEL <release_channel>
//	  SET RELEASE DIRECTIVE <release_directive>
//	  ACCOUNTS = ( <organization_name>.<account_name> [ , <organization_name>.<account_name> , ... ] )
//	  VERSION = <version_identifier>
//	  PATCH = <patch_num>
//	  [ UPGRADE_AFTER = '<timestamp>' ]
//	  [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	  [ UPGRADE_DEADLINE = '<timestamp>' ]
//
//	ALTER APPLICATION PACKAGE <name>
//	 MODIFY RELEASE CHANNEL <release_channel>
//	 MODIFY RELEASE DIRECTIVE <release_directive>
//	 VERSION = <version_identifier>
//	 PATCH = <patch_num>
//	 [ UPGRADE_AFTER = '<timestamp>' ]
//	 [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	 [ UPGRADE_DEADLINE = '<timestamp>' ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  MODIFY RELEASE CHANNEL <release_channel>
//	  UNSET RELEASE DIRECTIVE <release_directive>
func (v *Validator) ParseAlterApplicationPackageModifyReleaseChannel() bool {
	name := v.parseIdentPath
	// Order-independent trailing options shared by the release-directive forms:
	// VERSION / PATCH / ACCOUNTS = ( ... ) / UPGRADE_AFTER / UPGRADE_IN_MAINTENANCE_WINDOW
	// / UPGRADE_DEADLINE.
	trailers := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("VERSION", v.parseScalar),
				v.option("PATCH", v.parseScalar),
				v.option("ACCOUNTS", v.consumeBalancedParens),
				v.option("UPGRADE_AFTER", v.parseString),
				v.option("UPGRADE_IN_MAINTENANCE_WINDOW", v.parseBool),
				v.option("UPGRADE_DEADLINE", v.parseString),
			)
		})
	}
	directive := func() bool {
		return v.Choice(
			// SET DEFAULT RELEASE DIRECTIVE [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("SET", "DEFAULT", "RELEASE", "DIRECTIVE") }, trailers)
			},
			// SET RELEASE DIRECTIVE <directive> [ ACCOUNTS = ( ... ) ] [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("SET", "RELEASE", "DIRECTIVE") }, name, trailers)
			},
			// MODIFY RELEASE DIRECTIVE <directive> [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("MODIFY", "RELEASE", "DIRECTIVE") }, name, trailers)
			},
			// UNSET RELEASE DIRECTIVE <directive>
			func() bool {
				return v.Sequence(func() bool { return v.phrase("UNSET", "RELEASE", "DIRECTIVE") }, name)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		name,
		func() bool { return v.phrase("MODIFY", "RELEASE", "CHANNEL") },
		name,
		directive,
	)
}

// ParseAlterApplicationPackageReleaseDirective validates the Snowflake `ALTER APPLICATION PACKAGE RELEASE DIRECTIVE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-release-directive
//
// Syntax:
//
//	ALTER APPLICATION PACKAGE <name>
//	  MODIFY RELEASE DIRECTIVE <release_directive>
//	  VERSION = <version_identifier>
//	  PATCH = <patch_num>
//	  [ UPGRADE_AFTER = '<timestamp>' ]
//	  [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	  [ UPGRADE_DEADLINE = '<timestamp>' ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  [ MODIFY RELEASE CHANNEL <release_channel_name> ]
//	  MODIFY RELEASE DIRECTIVE <release_directive>
//	  ADD ACCOUNTS = ( <organization_name>.<account_name> [ , <organization_name>.<account_name> , ... ] )
//	  [ VERSION = <version_identifier> PATCH = <patch_num> ]
//	  [ FORCE ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  [ MODIFY RELEASE CHANNEL <release_channel_name> ]
//	  MODIFY RELEASE DIRECTIVE <release_directive>
//	  REMOVE ACCOUNTS = ( <organization_name>.<account_name> [ , <organization_name>.<account_name> , ... ] )
//	  [ VERSION = <version_identifier> PATCH = <patch_num> ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  SET DEFAULT RELEASE DIRECTIVE
//	  VERSION = <version_identifier>
//	  PATCH = <patch_num>
//	  [ UPGRADE_AFTER = '<timestamp>' ]
//	  [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	  [ UPGRADE_DEADLINE = '<timestamp>' ]
//
//	ALTER APPLICATION PACKAGE <name>
//	  SET RELEASE DIRECTIVE <release_directive>
//	  ACCOUNTS = ( <organization_name>.<account_name> [ , <organization_name>.<account_name> , ... ] )
//	  VERSION = <version_identifier>
//	  PATCH = <patch_num>
//	  [ UPGRADE_AFTER = '<timestamp>' ]
//	  [ UPGRADE_IN_MAINTENANCE_WINDOW = { TRUE | FALSE } ]
//	  [ UPGRADE_DEADLINE = '<timestamp>' ]
//	  [ FORCE ]
//
//	ALTER APPLICATION PACKAGE <name> UNSET RELEASE DIRECTIVE <release_directive>
func (v *Validator) ParseAlterApplicationPackageReleaseDirective() bool {
	name := v.parseIdentPath
	// Order-independent trailing options shared by the release-directive forms:
	// VERSION / PATCH / [ ADD | REMOVE ] ACCOUNTS = ( ... ) / UPGRADE_AFTER /
	// UPGRADE_IN_MAINTENANCE_WINDOW / UPGRADE_DEADLINE / FORCE.
	trailers := func() bool {
		return v.ZeroOrMore(func() bool {
			return v.Choice(
				v.option("VERSION", v.parseScalar),
				v.option("PATCH", v.parseScalar),
				func() bool {
					return v.Sequence(v.wordsValue("ADD", "REMOVE"), v.option("ACCOUNTS", v.consumeBalancedParens))
				},
				v.option("ACCOUNTS", v.consumeBalancedParens),
				v.option("UPGRADE_AFTER", v.parseString),
				v.option("UPGRADE_IN_MAINTENANCE_WINDOW", v.parseBool),
				v.option("UPGRADE_DEADLINE", v.parseString),
				func() bool { return v.MatchWord("FORCE") },
			)
		})
	}
	directive := func() bool {
		return v.Choice(
			// SET DEFAULT RELEASE DIRECTIVE [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("SET", "DEFAULT", "RELEASE", "DIRECTIVE") }, trailers)
			},
			// SET RELEASE DIRECTIVE <directive> [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("SET", "RELEASE", "DIRECTIVE") }, name, trailers)
			},
			// MODIFY RELEASE DIRECTIVE <directive> [ <trailers> ]
			func() bool {
				return v.Sequence(func() bool { return v.phrase("MODIFY", "RELEASE", "DIRECTIVE") }, name, trailers)
			},
			// UNSET RELEASE DIRECTIVE <directive>
			func() bool {
				return v.Sequence(func() bool { return v.phrase("UNSET", "RELEASE", "DIRECTIVE") }, name)
			},
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		name,
		// optional MODIFY RELEASE CHANNEL <name>
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.phrase("MODIFY", "RELEASE", "CHANNEL") }, name)
			})
		},
		directive,
	)
}

// ParseAlterApplicationPackageVersion validates the Snowflake `ALTER APPLICATION PACKAGE VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-package-version
//
// Syntax:
//
//	ALTER APPLICATION PACKAGE <name> ADD VERSION [ <version_identifier> ]
//	  USING <path_to_version_directory> [ LABEL = '<display_label>' ]
//
//	ALTER APPLICATION PACKAGE <name> DROP VERSION <version_identifier>
//
//	ALTER APPLICATION PACKAGE <name> ADD PATCH [<patch_number>] FOR VERSION [<version_identifier>]
//	  USING <path_to_version_directory> [ LABEL = '<display_label>' ]
func (v *Validator) ParseAlterApplicationPackageVersion() bool {
	name := v.parseIdentPath
	num := func() bool { return v.Match(sqltok.NumberLit) }
	// `[ <version_identifier> ] USING` — the version is optional, so try
	// `<version> USING` first and fall back to a bare `USING` (a greedy identifier
	// match would otherwise swallow the USING keyword).
	optVersionUsing := func() bool {
		return v.Choice(
			func() bool { return v.Sequence(name, func() bool { return v.MatchWord("USING") }) },
			func() bool { return v.MatchWord("USING") },
		)
	}
	// USING <path> [ LABEL = '<display_label>' ]
	usingPath := func() bool {
		return v.Sequence(
			optVersionUsing,
			v.parseScalar, // <path_to_version_directory>
			func() bool { return v.Optional(v.option("LABEL", v.parseString)) },
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "PACKAGE") },
		name,
		func() bool {
			return v.Choice(
				// ADD VERSION [ <version> ] USING <path> [ LABEL = '<label>' ]
				func() bool { return v.Sequence(func() bool { return v.phrase("ADD", "VERSION") }, usingPath) },
				// DROP VERSION <version_identifier>
				func() bool { return v.Sequence(func() bool { return v.phrase("DROP", "VERSION") }, name) },
				// ADD PATCH [ <patch_number> ] FOR VERSION [ <version> ] USING <path> [ LABEL = '<label>' ]
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("ADD", "PATCH") },
						func() bool { return v.Optional(num) },
						func() bool { return v.phrase("FOR", "VERSION") },
						usingPath,
					)
				},
			)
		},
	)
}

// ParseAlterApplicationRole validates the Snowflake `ALTER APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-role
//
// Syntax:
//
//	ALTER APPLICATION ROLE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER APPLICATION ROLE [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER APPLICATION ROLE [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterApplicationRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(func() bool { return v.MatchWord("SET") }, v.commentOption()) },
				func() bool { return v.phrase("UNSET", "COMMENT") },
			)
		},
	)
}

// ParseAlterApplicationApproveDeclineSpecification validates the Snowflake `ALTER APPLICATION APPROVE DECLINE SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-sequence-number
//
// Syntax:
//
//	ALTER APPLICATION <app_name>
//	  { APPROVE | DECLINE } SPECIFICATION <spec_name>
//	  SEQUENCE_NUMBER = <sequence_num>;
func (v *Validator) ParseAlterApplicationApproveDeclineSpecification() bool {
	// ALTER APPLICATION <app> { APPROVE | DECLINE } SPECIFICATION <spec> SEQUENCE_NUMBER = <num>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
		v.wordsValue("APPROVE", "DECLINE"),
		func() bool { return v.MatchWord("SPECIFICATION") },
		v.parseIdentPath,
		v.option("SEQUENCE_NUMBER", v.parseNumber),
	)
}

// ParseAlterApplicationSetSpecification validates the Snowflake `ALTER APPLICATION SET SPECIFICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-app-spec
//
// Syntax:
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = EXTERNAL_ACCESS
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  { HOST_PORTS | PRIVATE_HOST_PORTS } = ( '<value>' [, '<value>', ... ] )
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	    TYPE = SECURITY_INTEGRATION
//	    LABEL = '<string_literal>'
//	    DESCRIPTION = '<string_literal>'
//	    OAUTH_TYPE = 'CLIENT_CREDENTIALS'
//	    OAUTH_TOKEN_ENDPOINT = '<string_literal>'
//	    OAUTH_ALLOWED_SCOPES = ( '<scope>' [ , '<scope>' ... ] );
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = SECURITY_INTEGRATION
//	  LABEL = '<string_literal>'
//	  DESCRIPTION = '<string_literal>'
//	  OAUTH_TYPE = 'AUTHORIZATION_CODE'
//	  OAUTH_TOKEN_ENDPOINT = '<string_literal>'
//	  [ OAUTH_AUTHORIZATION_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_ALLOWED_SCOPES = ( '<scope>' [ , '<scope>' ... ] ) ];
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = SECURITY_INTEGRATION
//	  LABEL = '<string_literal>'
//	  DESCRIPTION = '<string_literal>'
//	  OAUTH_TYPE = 'JWT_BEARER'
//	  OAUTH_TOKEN_ENDPOINT = '<string_literal>'
//	  [ OAUTH_AUTHORIZATION_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_ALLOWED_SCOPES = ( '<scope>' [ , '<scope>' ... ] ) ];
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = LISTING
//	  LABEL = '<string_literal>'
//	  DESCRIPTION = '<string_literal>'
//	  TARGET_ACCOUNTS = '<account_list>'
//	  LISTING = <listing_name>
//	  [ AUTO_FULFILLMENT_REFRESH_SCHEDULE = '<schedule>' ]
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = CONNECTION
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  SERVER_APPLICATION = <server_app>
//	  SERVER_APPLICATION_ROLES = ( <app_role1> [ , <app_role2> ... ] );
//
//	ALTER APPLICATION SET SPECIFICATION <app_spec_name>
//	  TYPE = SETTING
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  SETTING = <setting_name>
//	  [ VALUE = '<value>' ]
func (v *Validator) ParseAlterApplicationSetSpecification() bool {
	// ALTER APPLICATION SET SPECIFICATION <app_spec_name> <option list…>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "SET", "SPECIFICATION") },
		v.parseIdentPath,
		// At least one option follows (TYPE = …, LABEL = …, etc.) — free-form.
		func() bool { return v.MatchWord("TYPE") },
		func() bool { return v.MatchOp("=") },
		v.consumeRest,
	)
}

// ParseAlterApplicationSetConfigurationDefinition validates the Snowflake `ALTER APPLICATION SET CONFIGURATION DEFINITION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-configuration-definition
//
// Syntax:
//
//	For `APPLICATION_NAME`:
//
//	ALTER APPLICATION SET CONFIGURATION DEFINITION <config>
//	  TYPE = APPLICATION_NAME
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  APPLICATION_ROLES = ( <app_role1> [ , <app_role2> ... ] );
//
//	For `STRING`:
//
//	ALTER APPLICATION SET CONFIGURATION DEFINITION <config>
//	  TYPE = STRING
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  APPLICATION_ROLES = ( <app_role1> [ , <app_role2> ... ] )
//	  SENSITIVE = { TRUE | FALSE };
//
//	For `SECRET_AUTHORIZATION`:
//
//	ALTER APPLICATION SET CONFIGURATION DEFINITION <config>
//	  TYPE = SECRET_AUTHORIZATION
//	  SECRET = <schema>.<secret>
//	  LABEL = '<label>'
//	  DESCRIPTION = '<description>'
//	  APPLICATION_ROLES = ( <app_role1> [ , <app_role2> ... ] );
func (v *Validator) ParseAlterApplicationSetConfigurationDefinition() bool {
	// ALTER APPLICATION SET CONFIGURATION DEFINITION <config> <option list…>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "SET", "CONFIGURATION", "DEFINITION") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TYPE") },
		func() bool { return v.MatchOp("=") },
		v.consumeRest,
	)
}

// ParseAlterApplicationSetConfigurationValue validates the Snowflake `ALTER APPLICATION SET CONFIGURATION VALUE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-set-configuration-value
//
// Syntax:
//
//	ALTER APPLICATION <app> SET CONFIGURATION <config> VALUE = '<value>';
func (v *Validator) ParseAlterApplicationSetConfigurationValue() bool {
	// ALTER APPLICATION <app> SET CONFIGURATION <config> VALUE = '<value>'
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
		func() bool { return v.phrase("SET", "CONFIGURATION") },
		v.parseIdentPath,
		v.option("VALUE", v.parseString),
	)
}

// ParseAlterApplicationUnsetConfiguration validates the Snowflake `ALTER APPLICATION UNSET CONFIGURATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-unset-configuration
//
// Syntax:
//
//	ALTER APPLICATION <app> UNSET CONFIGURATION <config>;
func (v *Validator) ParseAlterApplicationUnsetConfiguration() bool {
	// ALTER APPLICATION <app> UNSET CONFIGURATION <config>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("APPLICATION") },
		v.parseIdentPath,
		func() bool { return v.phrase("UNSET", "CONFIGURATION") },
		v.parseIdentPath,
	)
}

// ParseAlterAuthenticationPolicy validates the Snowflake `ALTER AUTHENTICATION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-authentication-policy
//
// Syntax:
//
//	ALTER AUTHENTICATION POLICY <name> RENAME TO <new_name>
//
//	ALTER AUTHENTICATION POLICY [ IF EXISTS ] <name> SET
//	  [ AUTHENTICATION_METHODS = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_TYPES = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ CLIENT_POLICY = ( <client_type> = ( MINIMUM_VERSION = '<version>' ) [ , ... ] ) ]
//	  [ SECURITY_INTEGRATIONS = ( '<string_literal>' [ , '<string_literal>' , ...  ] ) ]
//	  [ MFA_ENROLLMENT = { 'REQUIRED' | 'REQUIRED_PASSWORD_ONLY' | 'OPTIONAL' } ]
//	  [ MFA_POLICY= ( <list_of_properties> ) ]
//	  [ PAT_POLICY = ( <list_of_properties> ) ]
//	  [ WORKLOAD_IDENTITY_POLICY = ( <list_of_properties> ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER AUTHENTICATION POLICY [ IF EXISTS ] <name> UNSET
//	  [ AUTHENTICATION_METHODS ]
//	  [ CLIENT_TYPES ]
//	  [ CLIENT_POLICY ]
//	  [ SECURITY_INTEGRATIONS ]
//	  [ MFA_ENROLLMENT ]
//	  [ MFA_POLICY ]
//	  [ PAT_POLICY ]
//	  [ WORKLOAD_IDENTITY_POLICY ]
//	  [ COMMENT ]
//	  [ DCM PROJECT ]
func (v *Validator) ParseAlterAuthenticationPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("AUTHENTICATION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterBackupPolicy validates the Snowflake `ALTER BACKUP POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-backup-policy
//
// Syntax:
//
//	ALTER BACKUP POLICY <name> RENAME TO <new_name>
//
//	ALTER BACKUP POLICY <name> SET
//	  [ COMMENT = '<string_literal>' ]
//	  [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	  [ EXPIRE_AFTER_DAYS = <days_integer> ]
//
//	ALTER BACKUP POLICY <name> UNSET { COMMENT | SCHEDULE | EXPIRE_AFTER_DAYS }
//
//	ALTER BACKUP POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER BACKUP POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterBackupPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("BACKUP", "POLICY") },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterBackupSet validates the Snowflake `ALTER BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-backup-set
//
// Syntax:
//
//	ALTER BACKUP SET <name> ADD BACKUP
//
//	ALTER BACKUP SET <name> APPLY BACKUP POLICY <policy_name> [ FORCE ]
//
//	ALTER BACKUP SET <name> SUSPEND BACKUP [ { CREATION | EXPIRATION } ] POLICY
//
//	ALTER BACKUP SET <name> RESUME BACKUP [ { CREATION | EXPIRATION } ] POLICY
//
//	ALTER BACKUP SET <name> DELETE BACKUP IDENTIFIER '<backup_id>'
//
//	ALTER BACKUP SET <name> MODIFY BACKUP IDENTIFIER '<backup_id>' { ADD | REMOVE } LEGAL HOLD
//
//	ALTER BACKUP SET <name> MODIFY BACKUP IDENTIFIER '<backup_id>' SET COMMENT = '<string_literal>'
//
//	ALTER BACKUP SET <name> MODIFY BACKUP IDENTIFIER '<backup_id>' UNSET COMMENT
//
//	ALTER BACKUP SET <name> RENAME TO <new_name>
//
//	ALTER BACKUP SET <name> SET COMMENT = '<string_literal>'
//
//	ALTER BACKUP SET <name> UNSET COMMENT
//
//	ALTER BACKUP SET <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER BACKUP SET <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterBackupSet() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("BACKUP", "SET") },
		v.parseIdentPath,
		// One of many actions (ADD/APPLY/SUSPEND/RESUME/DELETE/MODIFY/RENAME/SET/UNSET);
		// require at least the action verb then consume the free-form remainder.
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "APPLY", "SUSPEND", "RESUME", "DELETE", "MODIFY", "SET", "UNSET"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterCatalogIntegration validates the Snowflake `ALTER CATALOG INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-catalog-integration
//
// Syntax:
//
//	ALTER CATALOG INTEGRATION [ IF EXISTS ] <name> SET
//	  REST_AUTHENTICATION = (
//	    restAuthenticationParams
//	  )
//	  [ REFRESH_INTERVAL_SECONDS = <value> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	restAuthenticationParams (for OAuth) ::=
//
//	  OAUTH_CLIENT_SECRET = '<oauth_client_secret>'
//
//	restAuthenticationParams (for Bearer token) ::=
//
//	  BEARER_TOKEN = '<bearer_token>'
func (v *Validator) ParseAlterCatalogIntegration() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("CATALOG", "INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// SET REST_AUTHENTICATION = ( … ) [ … ] — free-form remainder.
		func() bool { return v.MatchWord("SET") },
		v.consumeRest,
	)
}

// ParseAlterComputePool validates the Snowflake `ALTER COMPUTE POOL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-compute-pool
//
// Syntax:
//
//	ALTER COMPUTE POOL [ IF EXISTS ] <name> { SUSPEND | RESUME }
//
//	ALTER COMPUTE POOL [ IF EXISTS ] <name> STOP ALL  [ OF TYPE <workload_type> [ , ... ] ]
//
//	ALTER COMPUTE POOL [ IF EXISTS ] <name> SET [ MIN_NODES = <num> ]
//	                                            [ MAX_NODES = <num> ]
//	                                            [ AUTO_RESUME = { TRUE | FALSE } ]
//	                                            [ AUTO_SUSPEND_SECS = <num> ]
//	                                            [ PLACEMENT_GROUP = '<placement_group_name>' ]
//	                                            [ INSTANCE_FAMILY = <instance_family_name> ]
//	                                            [ TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' , ... ] ]
//	                                            [ COMMENT = '<string_literal>' ]
//
//	ALTER COMPUTE POOL [ IF EXISTS ] <name> UNSET { AUTO_SUSPEND_SECS |
//	                                                AUTO_RESUME       |
//	                                                PLACEMENT_GROUP   |
//	                                                COMMENT
//	                                              }
//	                                              [ , ... ]
func (v *Validator) ParseAlterComputePool() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("COMPUTE", "POOL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("SUSPEND", "RESUME"),
				// STOP ALL [ OF TYPE … ]
				func() bool { return v.phrase("STOP", "ALL") && v.consumeRest() },
				// SET … | UNSET …
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterConnection validates the Snowflake `ALTER CONNECTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-connection
//
// Syntax:
//
//	ALTER CONNECTION [ IF EXISTS ] <name> ENABLE FAILOVER TO ACCOUNTS <organization_name>.<account_name> [ , <organization_name>.<account_name> ... ]
//	                        [ IGNORE EDITION CHECK ]
//
//	ALTER CONNECTION [ IF EXISTS ] <name> DISABLE FAILOVER [ TO ACCOUNTS <organization_name>.<account_name> [ , <organization_name>.<account_name> ... ] ]
//
//	ALTER CONNECTION [ IF EXISTS ] <name> PRIMARY
//
//	ALTER CONNECTION [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER CONNECTION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterConnection() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("CONNECTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// ENABLE FAILOVER TO ACCOUNTS … [ IGNORE EDITION CHECK ]
				func() bool { return v.phrase("ENABLE", "FAILOVER") && v.consumeRest() },
				// DISABLE FAILOVER [ TO ACCOUNTS … ]
				func() bool { return v.phrase("DISABLE", "FAILOVER") && v.consumeRest() },
				func() bool { return v.MatchWord("PRIMARY") },
				// SET COMMENT = … | UNSET COMMENT
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterContact validates the Snowflake `ALTER CONTACT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-contact
//
// Syntax:
//
//	ALTER CONTACT [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER CONTACT [ IF EXISTS ] <name> SET
//	  [ {
//	    USERS = ( '<user_name>' [ , '<user_name>' ... ] )
//	    | EMAIL_DISTRIBUTION_LIST = '<email>'
//	    | URL = '<url>'
//	    } ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseAlterContact() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("CONTACT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(func() bool { return v.MatchWord("SET") }, v.consumeRest) },
			)
		},
	)
}

// ParseAlterCortexSearchService validates the Snowflake `ALTER CORTEX SEARCH SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-cortex-search
//
// Syntax:
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name>
//	  { SUSPEND | RESUME } [ { INDEXING | SERVING } ]
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> REFRESH
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> SET
//	  [ TARGET_LAG = { '<num> { seconds | minutes | hours | days }' } ]
//	  [ WAREHOUSE = <warehouse_name> ]
//	  [ PRIMARY KEY = ( <col_name> [, ... ] ) ]
//	  [ FULL_INDEX_BUILD_INTERVAL_DAYS = <num> ]
//	  [ REQUEST_LOGGING = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND = { <num_seconds> | NULL } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> UNSET
//	  [ PRIMARY KEY ]
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> SET ATTRIBUTES ( <col_name> [, ... ] )
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> UNSET ATTRIBUTES
//
//	ALTER CORTEX SEARCH SERVICE <name>
//	  ADD SCORING PROFILE [ IF NOT EXISTS ] <profile_name>
//	  <scoring_profile>
//
//	ALTER CORTEX SEARCH SERVICE <name>
//	  DROP SCORING PROFILE [ IF EXISTS ] <profile_name>
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER CORTEX SEARCH SERVICE [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterCortexSearchService() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("CORTEX", "SEARCH", "SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// { SUSPEND | RESUME } [ { INDEXING | SERVING } ]
				func() bool {
					return v.Sequence(
						v.wordsValue("SUSPEND", "RESUME"),
						func() bool { return v.Optional(v.wordsValue("INDEXING", "SERVING")) },
					)
				},
				func() bool { return v.MatchWord("REFRESH") },
				// ADD/DROP SCORING PROFILE … ; SET/UNSET [ATTRIBUTES|TAG|options]
				func() bool {
					return v.Sequence(v.wordsValue("ADD", "DROP", "SET", "UNSET"), v.consumeRest)
				},
			)
		},
	)
}

// ParseAlterDatabase validates the Snowflake `ALTER DATABASE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database
//
// Syntax:
//
//	ALTER DATABASE [ IF EXISTS ] <name> RENAME TO <new_db_name>
//
//	ALTER DATABASE [ IF EXISTS ] <name> SWAP WITH <target_db_name>
//
//	ALTER DATABASE [ IF EXISTS ] <name> SET [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	                                        [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	                                        [ EXTERNAL_VOLUME = <external_volume_name> ]
//	                                        [ CATALOG = <catalog_integration_name> ]
//	                                        [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	                                        [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	                                        [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	                                        [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	                                        [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	                                        [ DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU = '<compute_pool_name>' ]
//	                                        [ DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU = '<compute_pool_name>' ]
//	                                        [ OBJECT_VISIBILITY = { <object_visibility_spec> | PRIVILEGED } ]
//	                                        [ LOG_LEVEL = '<log_level>' ]
//	                                        [ METRIC_LEVEL = '<metric_level>' ]
//	                                        [ TRACE_LEVEL = '<trace_level>' ]
//	                                        [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	                                        [ EVENT_TABLE = <event_table_name> ]
//	                                        [ COMMENT = '<string_literal>' ]
//	                                        [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	                                        [ REPLICABLE_WITH_FAILOVER_GROUPS = { 'YES' | 'NO' } ]
//	                                        [ BASE_LOCATION_PREFIX = '<string>' ]
//	                                        [ DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE = <warehouse_name> ]
//	                                        [ CLASSIFICATION_PROFILE = '<profile_name>' ]
//	                                        [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	                                        [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//	                                        [ DATA_QUALITY_MONITORING_SETTINGS = <yaml_spec> ]
//
//	ALTER DATABASE <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER DATABASE <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER DATABASE [ IF EXISTS ] <name> UNSET { DATA_RETENTION_TIME_IN_DAYS         |
//	                                            MAX_DATA_EXTENSION_TIME_IN_DAYS     |
//	                                            EXTERNAL_VOLUME                     |
//	                                            CATALOG                             |
//	                                            ICEBERG_VERSION_DEFAULT             |
//	                                            ICEBERG_MERGE_ON_READ_BEHAVIOR      |
//	                                            ENABLE_ICEBERG_MERGE_ON_READ        |
//	                                            DEFAULT_DDL_COLLATION               |
//	                                            DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU   |
//	                                            DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU   |
//	                                            OBJECT_VISIBILITY                   |
//	                                            STORAGE_SERIALIZATION_POLICY        |
//	                                            EVENT_TABLE = <event_table_name>    |
//	                                            COMMENT                             |
//	                                            CATALOG_SYNC                        |
//	                                            REPLICABLE_WITH_FAILOVER_GROUPS     |
//	                                            BASE_LOCATION_PREFIX                |
//	                                            DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE|
//	                                            CLASSIFICATION_PROFILE              |
//	                                            CONTACT <purpose>                   |
//	                                            ENABLE_DATA_COMPACTION              |
//	                                            DCM PROJECT
//	                                          }
//	                                          [ , ... ]
//
//	ALTER DATABASE <name> ENABLE REPLICATION TO ACCOUNTS <account_identifier> [ , <account_identifier> ... ] [ IGNORE EDITION CHECK ]
//
//	ALTER DATABASE <name> DISABLE REPLICATION [ TO ACCOUNTS <account_identifier> [ , <account_identifier> ... ] ]
//
//	ALTER DATABASE <name> REFRESH
//
//	ALTER DATABASE <name> ENABLE FAILOVER TO ACCOUNTS <account_identifier> [ , <account_identifier> ... ]
//
//	ALTER DATABASE <name> DISABLE FAILOVER [ TO ACCOUNTS <account_identifier> [ , <account_identifier> ... ] ]
//
//	ALTER DATABASE <name> PRIMARY
func (v *Validator) ParseAlterDatabase() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("DATABASE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("SWAP", "WITH") && v.parseIdentPath() },
				func() bool { return v.MatchWord("REFRESH") },
				func() bool { return v.MatchWord("PRIMARY") },
				// ENABLE/DISABLE { REPLICATION | FAILOVER } … TO ACCOUNTS …
				func() bool { return v.Sequence(v.wordsValue("ENABLE", "DISABLE"), v.consumeRest) },
				// SET <options/TAG> | UNSET <options/TAG>
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterDatabaseCatalogLinked validates the Snowflake `ALTER DATABASE (catalog-linked)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database-catalog-linked
//
// Syntax:
//
//	ALTER DATABASE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER DATABASE [ IF EXISTS ] <name> SUSPEND DISCOVERY
//
//	ALTER DATABASE [ IF EXISTS ] <name> RESUME DISCOVERY
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  ADD ( '<namespace>' [ , ... ] ) TO ALLOWED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  REMOVE ( '<namespace>' [ , ... ] ) FROM ALLOWED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  UNSET ALLOWED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  ADD ( '<namespace>' [ , ... ] ) TO BLOCKED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  REMOVE ( '<namespace>' [ , ... ] ) FROM BLOCKED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  UNSET BLOCKED_NAMESPACES
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  SET SYNC_INTERVAL_SECONDS = <value>
//
//	ALTER DATABASE [ IF EXISTS ] <name> UPDATE LINKED_CATALOG
//	  SET ALLOWED_WRITE_OPERATIONS = { NONE | ALL }
//
//	ALTER DATABASE [ IF EXISTS ] <name> SET [ BASE_LOCATION_PREFIX = '<string>' ]
//	                                        [ COMMENT = '<string_literal>' ]
//	                                        [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	                                        [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	                                        [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	                                        [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//
//	ALTER DATABASE [ IF EXISTS ] <name> UNSET { BASE_LOCATION_PREFIX         |
//	                                            COMMENT                      |
//	                                            CONTACT                      |
//	                                            ICEBERG_VERSION_DEFAULT      |
//	                                            ICEBERG_MERGE_ON_READ_BEHAVIOR |
//	                                            ENABLE_ICEBERG_MERGE_ON_READ
//	                                          }
func (v *Validator) ParseAlterDatabaseCatalogLinked() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("DATABASE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SUSPEND/RESUME DISCOVERY
				func() bool {
					return v.Sequence(v.wordsValue("SUSPEND", "RESUME"), func() bool { return v.MatchWord("DISCOVERY") })
				},
				// UPDATE LINKED_CATALOG <add/remove/unset/set …>
				func() bool { return v.phrase("UPDATE", "LINKED_CATALOG") && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterDatabaseRole validates the Snowflake `ALTER DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-database-role
//
// Syntax:
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> UNSET COMMENT
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER DATABASE ROLE [ IF EXISTS ] <name> UNSET DCM PROJECT
func (v *Validator) ParseAlterDatabaseRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterDataset validates the Snowflake `ALTER DATASET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseAlterDataset() bool {
	// Syntax unavailable: permissive `ALTER DATASET [IF EXISTS] <name> <action…>`.
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("DATASET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// Require at least one action token, then consume the remainder.
		func() bool { return !v.AtEnd() },
		v.consumeRest,
	)
}

// ParseAlterDatasetAddVersion validates the Snowflake `ALTER DATASET ADD VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset-add-version
//
// Syntax:
//
//	ALTER DATASET <name> ADD VERSION <version_name>
//	  FROM <select_statement>
//	  [ PARTITION BY <string_expr> ]
//	  [ COMMENT = <string_literal> ]
//	  [ METADATA = <json_string_literal> ]
func (v *Validator) ParseAlterDatasetAddVersion() bool {
	// ALTER DATASET <name> ADD VERSION <version_name> FROM <select_statement> [ … ]
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("DATASET") },
		v.parseIdentPath,
		func() bool { return v.phrase("ADD", "VERSION") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		v.consumeRest,
	)
}

// ParseAlterDatasetDropVersion validates the Snowflake `ALTER DATASET DROP VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dataset-drop-version
//
// Syntax:
//
//	ALTER DATASET [ IF EXISTS ] <name> DROP VERSION <version_name>
func (v *Validator) ParseAlterDatasetDropVersion() bool {
	// ALTER DATASET [ IF EXISTS ] <name> DROP VERSION <version_name>
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("DATASET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("DROP", "VERSION") },
		v.parseIdentPath,
	)
}

// ParseAlterDbtProject validates the Snowflake `ALTER DBT PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dbt-project
//
// Syntax:
//
//	ALTER DBT PROJECT [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER DBT PROJECT <name> ADD VERSION [ <version_name_alias> ]
//	  FROM '<source_location>'
//
//	ALTER DBT PROJECT [ IF EXISTS ] <name> SET
//	  [ DBT_VERSION = '<version_number>' ]
//	  [ DEFAULT_TARGET = '<default_target>' ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [, ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER DBT PROJECT [ IF EXISTS ] <name> UNSET
//	  [ DBT_VERSION ]
//	  [ DEFAULT_TARGET ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS ]
//	  [ COMMENT ]
func (v *Validator) ParseAlterDbtProject() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("DBT", "PROJECT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// ADD VERSION [ <alias> ] FROM '<source>'
				func() bool { return v.phrase("ADD", "VERSION") && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterDcmProject validates the Snowflake `ALTER DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dcm-project
//
// Syntax:
//
//	ALTER DCM PROJECT [ IF EXISTS ] <name> SET
//	  [ LOG_LEVEL = <log_level> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER DCM PROJECT [ IF EXISTS ] <name> UNSET
//	  [ LOG_LEVEL ]
//	  [ COMMENT ]
func (v *Validator) ParseAlterDcmProject() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("DCM", "PROJECT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterDynamicTable validates the Snowflake `ALTER DYNAMIC TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-dynamic-table
//
// Syntax:
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> { SUSPEND | RESUME }
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> SWAP WITH <target_dynamic_table_name>
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> REFRESH [ COPY SESSION ]
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> { clusteringAction }
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> { tableColumnCommentAction }
//
//	ALTER DYNAMIC TABLE <name> { SET | UNSET } COMMENT = '<string_literal>'
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> dataGovnPolicyTagAction
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> searchOptimizationAction
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> storageLifecyclePolicyAction
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> SET
//	  [ TARGET_LAG = { '<num> { seconds | minutes | hours | days }' | DOWNSTREAM } ],
//	  [ SCHEDULER = DISABLE | ENABLE ],
//	  [ WAREHOUSE = <warehouse_name> ],
//	  [ INITIALIZATION_WAREHOUSE = <warehouse_name> ],
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ],
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ DEFAULT_DDL_COLLATION = '<collation_specification>' ],
//	  [ LOG_LEVEL = '<log_level>' ],
//	  [ CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ],
//	  [ FROZEN WHERE ( <expr> ) ],
//	  [ EXECUTE AS USER <user_name>
//	    [ USE SECONDARY ROLES { ALL | NONE | <role> [ , ... ] } ]
//	  ]
//	  [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//
//	ALTER DYNAMIC TABLE [ IF EXISTS ] <name> UNSET
//	  [ INITIALIZATION_WAREHOUSE ],
//	  [ DATA_RETENTION_TIME_IN_DAYS ],
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS ],
//	  [ DEFAULT_DDL_COLLATION ],
//	  [ LOG_LEVEL ],
//	  [ CONTACT <purpose> ],
//	  [ FROZEN WHERE ],
//	  [ EXECUTE AS USER ],
//	  [ ROW_TIMESTAMP ],
//	  [ DCM PROJECT ]
func (v *Validator) ParseAlterDynamicTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("DYNAMIC", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("SUSPEND", "RESUME"),
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("SWAP", "WITH") && v.parseIdentPath() },
				func() bool {
					return v.MatchWord("REFRESH") &&
						v.Optional(func() bool { return v.phrase("COPY", "SESSION") })
				},
				// SET / UNSET and the object-specific clustering / governance /
				// search-optimization actions — require a documented action verb (so a
				// garbage action is flagged) before the free-form remainder.
				func() bool {
					return v.Sequence(
						v.wordsValue("SET", "UNSET", "ADD", "DROP", "CLUSTER"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterExperiment validates the Snowflake `ALTER EXPERIMENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-experiment
//
// Syntax:
//
//	ALTER EXPERIMENT <experiment_name> ADD RUN <run_name>
//
//	ALTER EXPERIMENT <experiment_name> COMMIT RUN <run_name>
//
//	ALTER EXPERIMENT <experiment_name> DROP RUN <run_name>
func (v *Validator) ParseAlterExperiment() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("EXPERIMENT") },
		v.parseIdentPath,
		v.wordsValue("ADD", "COMMIT", "DROP"),
		func() bool { return v.MatchWord("RUN") },
		v.parseIdentPath,
	)
}

// ParseAlterExternalAgent validates the Snowflake `ALTER EXTERNAL AGENT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-agent
//
// Syntax:
//
//	ALTER EXTERNAL AGENT [ IF EXISTS ] <name> SET
//	  [ COMMENT = '<comment>' ]
//
//	ALTER EXTERNAL AGENT [ IF EXISTS ] <name> ADD VERSION <version_name>
func (v *Validator) ParseAlterExternalAgent() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("EXTERNAL", "AGENT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("SET") && v.consumeRest() },
				func() bool { return v.phrase("ADD", "VERSION") && v.parseIdentPath() },
			)
		},
	)
}

// ParseAlterExternalAccessIntegration validates the Snowflake `ALTER EXTERNAL ACCESS INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-access-integration
//
// Syntax:
//
//	ALTER EXTERNAL ACCESS INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ALLOWED_NETWORK_RULES = (<rule_name> [ , <rule_name> ... ]) ]
//	  [ ALLOWED_API_AUTHENTICATION_INTEGRATIONS = { ( <integration_name_1> [, <integration_name_2>, ... ] ) | none } ]
//	  [ ALLOWED_AUTHENTICATION_SECRETS = { ( <secret_name> [ , <secret_name> ... ] ) | all | none } ]
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ] ]
//
//	ALTER EXTERNAL ACCESS INTEGRATION [ IF EXISTS ] <name> UNSET {
//	  ALLOWED_NETWORK_RULES |
//	  ALLOWED_API_AUTHENTICATION_INTEGRATIONS |
//	  ALLOWED_AUTHENTICATION_SECRETS |
//	  COMMENT |
//	  TAG <tag_name> }
//	  [ , ... ]
func (v *Validator) ParseAlterExternalAccessIntegration() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("EXTERNAL", "ACCESS", "INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterExternalTable validates the Snowflake `ALTER EXTERNAL TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-table
//
// Syntax:
//
//	ALTER EXTERNAL TABLE [ IF EXISTS ] <name> REFRESH [ '<relative-path>' ]
//
//	ALTER EXTERNAL TABLE [ IF EXISTS ] <name> ADD FILES ( '<path>/[<filename>]' [ , '<path>/[<filename>'] ] )
//
//	ALTER EXTERNAL TABLE [ IF EXISTS ] <name> REMOVE FILES ( '<path>/[<filename>]' [ , '<path>/[<filename>]' ] )
//
//	ALTER EXTERNAL TABLE [ IF EXISTS ] <name> SET
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//
//	ALTER EXTERNAL TABLE <name> [ IF EXISTS ] ADD PARTITION ( <part_col_name> = '<string>' [ , <part_col_name> = '<string>' ] ) LOCATION '<path>'
//
//	ALTER EXTERNAL TABLE <name> [ IF EXISTS ] DROP PARTITION LOCATION '<path>'
func (v *Validator) ParseAlterExternalTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("EXTERNAL", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// The IF EXISTS can also appear after the name in some forms.
		func() bool { return v.Optional(func() bool { return v.ifExists() }) },
		func() bool {
			return v.Choice(
				// REFRESH [ '<path>' ]
				func() bool {
					return v.MatchWord("REFRESH") &&
						v.Optional(func() bool { return v.parseString() })
				},
				// ADD FILES (...) / REMOVE FILES (...)
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "REMOVE"),
						func() bool { return v.MatchWord("FILES") },
						v.consumeBalancedParens,
					)
				},
				// SET [ AUTO_REFRESH = ... ]
				func() bool { return v.MatchWord("SET") && v.consumeRest() },
				// ADD PARTITION (...) LOCATION '<path>'
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("ADD", "PARTITION") },
						v.consumeBalancedParens,
						func() bool { return v.MatchWord("LOCATION") },
						func() bool { return v.parseString() },
					)
				},
				// DROP PARTITION LOCATION '<path>'
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("DROP", "PARTITION", "LOCATION") },
						func() bool { return v.parseString() },
					)
				},
			)
		},
	)
}

// ParseAlterExternalVolume validates the Snowflake `ALTER EXTERNAL VOLUME` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-external-volume
//
// Syntax:
//
//	ALTER EXTERNAL VOLUME [ IF EXISTS ] <name> ADD STORAGE_LOCATION =
//	  (
//	    NAME = '<storage_location_name>'
//	    cloudProviderParams
//	  )
//
//	ALTER EXTERNAL VOLUME [ IF EXISTS ] <name> REMOVE STORAGE_LOCATION '<storage_location_name>'
//
//	ALTER EXTERNAL VOLUME [ IF EXISTS ] <name> UPDATE
//	  STORAGE_LOCATION = '<s3_compatible_storage_location_name>'
//	  CREDENTIALS = (
//	    AWS_KEY_ID = '<string>'
//	    AWS_SECRET_KEY = '<string>'
//	  )
//
//	ALTER EXTERNAL VOLUME [ IF EXISTS ] <name> SET ALLOW_WRITES = { TRUE | FALSE }
//
//	ALTER EXTERNAL VOLUME [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
func (v *Validator) ParseAlterExternalVolume() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("EXTERNAL", "VOLUME") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// REMOVE STORAGE_LOCATION '<name>'
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("REMOVE") },
						func() bool { return v.MatchWord("STORAGE_LOCATION") },
						func() bool { return v.parseString() },
					)
				},
				// ADD / UPDATE / SET — free-form trailing options.
				func() bool {
					return v.Sequence(v.wordsValue("ADD", "UPDATE", "SET"), v.consumeRest)
				},
			)
		},
	)
}

// ParseAlterFailoverGroup validates the Snowflake `ALTER FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-failover-group
//
// Syntax:
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  [ OBJECT_TYPES = <object_type> [ , <object_type> , ... ] ]
//	  [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	  [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	  [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  OBJECT_TYPES = INTEGRATIONS [ , <object_type> , ... ]
//	  ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> ... ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  COMMENT = '<string_literal>'
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }'
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  OPTIMIZED_REFRESH = { TRUE | FALSE }
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  ERROR_INTEGRATION = <integration_name>
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SET
//	  TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> UNSET
//	  { COMMENT | REPLICATION_SCHEDULE | OPTIMIZED_REFRESH | ERROR_INTEGRATION } [ , ... ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> UNSET
//	  TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  ADD <db_name> [ , <db_name> ,  ... ] TO ALLOWED_DATABASES
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  MOVE DATABASES <db_name> [ , <db_name> ,  ... ] TO FAILOVER GROUP <move_to_fg_name>
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  REMOVE <db_name> [ , <db_name> ,  ... ] FROM ALLOWED_DATABASES
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  ADD <share_name> [ , <share_name> ,  ... ] TO ALLOWED_SHARES
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  MOVE SHARES <share_name> [ , <share_name> ,  ... ] TO FAILOVER GROUP <move_to_fg_name>
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  REMOVE <share_name> [ , <share_name> ,  ... ] FROM ALLOWED_SHARES
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  ADD <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ] TO ALLOWED_ACCOUNTS
//	  [ IGNORE EDITION CHECK ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name>
//	  REMOVE <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ] FROM ALLOWED_ACCOUNTS
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> REFRESH
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> PRIMARY
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> SUSPEND [ IMMEDIATE ]
//
//	ALTER FAILOVER GROUP [ IF EXISTS ] <name> RESUME
func (v *Validator) ParseAlterFailoverGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("FAILOVER", "GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.MatchWord("REFRESH") },
				func() bool { return v.MatchWord("PRIMARY") },
				func() bool { return v.MatchWord("RESUME") },
				func() bool {
					return v.MatchWord("SUSPEND") &&
						v.Optional(func() bool { return v.MatchWord("IMMEDIATE") })
				},
				// SET / UNSET / ADD / MOVE / REMOVE — require a documented action verb
				// (so a garbage action is flagged) before the free-form target list.
				func() bool {
					return v.Sequence(
						v.wordsValue("SET", "UNSET", "ADD", "MOVE", "REMOVE"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterFeaturePolicy validates the Snowflake `ALTER FEATURE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-feature-policy
//
// Syntax:
//
//	ALTER FEATURE POLICY [ IF EXISTS ] <name> SET
//	  [ BLOCKED_OBJECT_TYPES_FOR_CREATION = ( [ <type> [ , <type>  ... ] ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER FEATURE POLICY [ IF EXISTS ] <name> UNSET
//	  [ BLOCKED_OBJECT_TYPES_FOR_CREATION ]
//	  [ COMMENT ]
//
//	ALTER FEATURE POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER FEATURE POLICY [ IF EXISTS ] <name> SET  TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER FEATURE POLICY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , ... ]
func (v *Validator) ParseAlterFeaturePolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("FEATURE", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterFileFormat validates the Snowflake `ALTER FILE FORMAT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-file-format
//
// Syntax:
//
//	ALTER FILE FORMAT [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER FILE FORMAT [ IF EXISTS ] <name> SET { [ formatTypeOptions ] [ COMMENT = '<string_literal>' ] }
func (v *Validator) ParseAlterFileFormat() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("FILE", "FORMAT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.MatchWord("SET") && v.consumeRest() },
			)
		},
	)
}

// ParseAlterFunction validates the Snowflake `ALTER FUNCTION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function
//
// Syntax:
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) RENAME TO <new_name>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET SECURE
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) UNSET { SECURE | LOG_LEVEL | TRACE_LEVEL | COMMENT }
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET
//	  [ LOG_LEVEL = '<log_level>' ]
//	  [ TRACE_LEVEL = '<trace_level>' ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , <integration_name> ... ] ) ]
//	  [ SECRETS = ( '<secret_variable_name>' = <secret_name> [ , '<secret_variable_name>' = <secret_name> ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	-- External functions:
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET API_INTEGRATION = <api_integration_name>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET HEADERS = ( [ '<header_1>' = '<value>' [ , '<header_2>' = '<value>' ... ] ] )
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET CONTEXT_HEADERS = ( [ <context_function_1> [ , <context_function_2> ...] ] )
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET MAX_BATCH_ROWS = <integer>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET COMPRESSION = <compression_type>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET { REQUEST_TRANSLATOR | RESPONSE_TRANSLATOR } = <udf_name>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) UNSET
//	              { COMMENT | HEADERS | CONTEXT_HEADERS | MAX_BATCH_ROWS | COMPRESSION | SECURE | REQUEST_TRANSLATOR | RESPONSE_TRANSLATOR }
func (v *Validator) ParseAlterFunction() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// ( [ <arg_data_type> , ... ] )
		v.consumeBalancedParens,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterFunctionDmf validates the Snowflake `ALTER FUNCTION (DMF)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function-dmf
//
// Syntax:
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  RENAME TO <new_name>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  SET SECURE
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  UNSET SECURE
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  SET COMMENT = '<string_literal>'
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  UNSET COMMENT
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( TABLE(  <arg_data_type> [ , ... ] ) [ , TABLE( <arg_data_type> [ , ... ] ) ] )
//	  UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterFunctionDmf() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// ( TABLE( <arg_data_type> [ , ... ] ) [ , TABLE(...) ] )
		v.consumeBalancedParens,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterFunctionSnowparkContainerServices validates the Snowflake `ALTER FUNCTION (Snowpark Container Services)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-function-spcs
//
// Syntax:
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  RENAME TO <new_name>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET CONTEXT_HEADERS = ( <context_function_1> [ , <context_function_2> ...] )
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET MAX_BATCH_ROWS = <integer>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET MAX_BATCH_RETRIES = <integer>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET ON_BATCH_FAILURE = { ABORT | RETURN_NULL }
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET BATCH_TIMEOUT_SECS = <integer>
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET COMMENT = '<string_literal>'
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  SET SERVICE = '<service_name>' ENDPOINT = '<endpoint_name>'
//
//	ALTER FUNCTION [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] )
//	  UNSET { CONTEXT_HEADERS | MAX_BATCH_ROWS | MAX_BATCH_RETRIES | ON_BATCH_FAILURE | BATCH_TIMEOUT_SECS | COMMENT }
func (v *Validator) ParseAlterFunctionSnowparkContainerServices() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("FUNCTION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// ( [ <arg_data_type> , ... ] )
		v.consumeBalancedParens,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterGateway validates the Snowflake `ALTER GATEWAY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-gateway
//
// Syntax:
//
//	ALTER GATEWAY [ IF EXISTS ] <name>
//	  FROM SPECIFICATION <specification_text>
func (v *Validator) ParseAlterGateway() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("GATEWAY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("FROM", "SPECIFICATION") },
		// <specification_text> — free-form.
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterGitRepository validates the Snowflake `ALTER GIT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-git-repository
//
// Syntax:
//
//	ALTER GIT REPOSITORY [ IF EXISTS ] <name> SET
//	  [ GIT_CREDENTIALS = <secret_name> ]
//	  [ API_INTEGRATION = <integration_name> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER GIT REPOSITORY [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER GIT REPOSITORY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER GIT REPOSITORY [ IF EXISTS ] <name> UNSET {
//	  GIT_CREDENTIALS |
//	  COMMENT }
//	  [ , ... ]
//
//	ALTER GIT REPOSITORY [ IF EXISTS ] <name> FETCH
func (v *Validator) ParseAlterGitRepository() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("GIT", "REPOSITORY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("FETCH") },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterIcebergTable validates the Snowflake `ALTER ICEBERG TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table
//
// Syntax:
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> { clusteringAction | tableColumnAction }
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> SET
//	  [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	  [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>']
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ AUTO_REFRESH = { TRUE | FALSE } ]
//	  [ TARGET_FILE_SIZE = { AUTO | 16MB | 32MB | 64MB | 128MB } ]
//	  [ CONTACT ( <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ) ]
//	  [ LOG_EVENT_LEVEL = { ERROR | WARN | DEBUG } ]
//	  [ ERROR_LOGGING = { TRUE | FALSE } ]
//	  [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> UNSET
//	  [ REPLACE_INVALID_CHARACTERS ]
//	  [ LOG_EVENT_LEVEL ]
//	  [ ERROR_LOGGING ]
//	  [ ENABLE_DATA_COMPACTION ]
//	  [ ICEBERG_MERGE_ON_READ_BEHAVIOR ]
//	  [ ENABLE_ICEBERG_MERGE_ON_READ ]
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] dataGovnPolicyTagAction
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> searchOptimizationAction
//
//	clusteringAction ::=
//	  {
//	     CLUSTER BY ( <expr> [ , <expr> , ... ] )
//	    | { SUSPEND | RESUME } RECLUSTER
//	    | DROP CLUSTERING KEY
//	  }
func (v *Validator) ParseAlterIcebergTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ICEBERG", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// clusteringAction
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("CLUSTER", "BY") },
						v.consumeBalancedParens,
					)
				},
				func() bool {
					return v.Sequence(v.wordsValue("SUSPEND", "RESUME"), func() bool { return v.MatchWord("RECLUSTER") })
				},
				func() bool { return v.phrase("DROP", "CLUSTERING", "KEY") },
				// SET / UNSET option lists, tableColumnAction, governance, and
				// search-optimization actions — require a documented action verb (so a
				// garbage action is flagged) before the free-form remainder.
				func() bool {
					return v.Sequence(
						v.wordsValue("SET", "UNSET", "ADD", "DROP", "ALTER", "MODIFY", "RENAME"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterIcebergTableAlterColumnSetDataType validates the Snowflake `ALTER ICEBERG TABLE ALTER COLUMN SET DATA TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-alter-column-set-data-type
//
// Syntax:
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> ALTER COLUMN <structured_column>
//	  SET DATA TYPE <new_structured_type>
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> ALTER COLUMN <structured_column>
//	  SET DATA TYPE <new_structured_type>
//	  RENAME FIELDS
func (v *Validator) ParseAlterIcebergTableAlterColumnSetDataType() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ICEBERG", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("ALTER", "COLUMN") },
		v.parseIdentPath,
		func() bool { return v.phrase("SET", "DATA", "TYPE") },
		// <new_structured_type>
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterIcebergTableConvertToManaged validates the Snowflake `ALTER ICEBERG TABLE CONVERT TO MANAGED` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-convert-to-managed
//
// Syntax:
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> CONVERT TO MANAGED
//	  [ BASE_LOCATION = '<directory_for_table_files>' ]
//	  [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
func (v *Validator) ParseAlterIcebergTableConvertToManaged() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ICEBERG", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("CONVERT", "TO", "MANAGED") },
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("BASE_LOCATION", v.parseString),
					v.option("STORAGE_SERIALIZATION_POLICY", v.wordsValue("COMPATIBLE", "OPTIMIZED")),
				)
			})
		},
	)
}

// ParseAlterIcebergTableRefresh validates the Snowflake `ALTER ICEBERG TABLE REFRESH` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-iceberg-table-refresh
//
// Syntax:
//
//	ALTER ICEBERG TABLE [ IF EXISTS ] <table_name> REFRESH [ '<metadata_file_relative_path>' ]
func (v *Validator) ParseAlterIcebergTableRefresh() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ICEBERG", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.MatchWord("REFRESH") },
		func() bool { return v.Optional(func() bool { return v.parseString() }) },
	)
}

// ParseAlterIntegration validates the Snowflake `ALTER INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-integration
//
// Syntax:
//
//	ALTER <integration_type> INTEGRATION <object_name> <actions>
//
//	Where <actions> are specific to the object type.
func (v *Validator) ParseAlterIntegration() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		// [ <integration_type> ] INTEGRATION
		func() bool {
			return v.Choice(
				// <integration_type> INTEGRATION (the type is a single leading word)
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchWord("INTEGRATION") },
					)
				},
				// INTEGRATION
				func() bool { return v.MatchWord("INTEGRATION") },
			)
		},
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// <actions> — object-type-specific, free-form.
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterJoinPolicy validates the Snowflake `ALTER JOIN POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-join-policy
//
// Syntax:
//
//	ALTER JOIN POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER JOIN POLICY [ IF EXISTS ] <name> SET BODY -> <expression>
//
//	ALTER JOIN POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER JOIN POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER JOIN POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER JOIN POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterJoinPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("JOIN", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET BODY -> <expression>
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("SET", "BODY") },
						func() bool { return v.MatchOp("-") },
						func() bool { return v.MatchOp(">") },
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterListing validates the Snowflake `ALTER LISTING` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-listing
//
// Syntax:
//
//	ALTER LISTING [ IF EXISTS ] <name> [ { PUBLISH | UNPUBLISH | REVIEW } ]
//
//	ALTER LISTING [ IF EXISTS ] <name> AS '<yaml_manifest_string>'
//	  [ PUBLISH = { TRUE | FALSE } ]
//	  [ REVIEW = { TRUE | FALSE } ]
//	  [ COMMENT = '<string>' ]
//
//	ALTER LISTING <name> ADD VERSION [ [ IF NOT EXISTS ] <version_name> ]
//	  FROM <yaml_manifest_stage_location>
//	  [ COMMENT = '<string>' ]
//
//	ALTER LISTING [ IF EXISTS ] <name> { ADD | REMOVE } TARGETS <manifest>
//
//	ALTER LISTING [ IF EXISTS ] <name> RENAME TO <new_name>;
//
//	ALTER LISTING [ IF EXISTS ] <name> SET COMMENT = '<string>'
func (v *Validator) ParseAlterListing() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("LISTING") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// [ { PUBLISH | UNPUBLISH | REVIEW } ]
				v.wordsValue("PUBLISH", "UNPUBLISH", "REVIEW"),
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// AS '<manifest>' [ options ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("AS") },
						func() bool { return v.parseString() },
						v.consumeRest,
					)
				},
				// ADD VERSION ... FROM ...
				func() bool { return v.phrase("ADD", "VERSION") && v.consumeRest() },
				// { ADD | REMOVE } TARGETS <manifest>
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "REMOVE"),
						func() bool { return v.MatchWord("TARGETS") },
						v.consumeRest,
					)
				},
				// SET COMMENT = ...
				func() bool { return v.MatchWord("SET") && v.consumeRest() },
				// Bare listing (no action).
				func() bool { return v.AtEnd() },
			)
		},
	)
}

// ParseAlterMaintenancePolicy validates the Snowflake `ALTER MAINTENANCE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-maintenance-policy
//
// Syntax:
//
//	ALTER MAINTENANCE POLICY [ IF EXISTS ] <name> SET
//	   [ SCHEDULE = '<schedule>' ]
//	   [ COMMENT = '<comment>' ]
//
//	ALTER MAINTENANCE POLICY [ IF EXISTS ] <name> UNSET
//	   [ COMMENT ]
//
//	ALTER MAINTENANCE POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
func (v *Validator) ParseAlterMaintenancePolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("MAINTENANCE", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterMaskingPolicy validates the Snowflake `ALTER MASKING POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-masking-policy
//
// Syntax:
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> SET BODY -> <expression_on_arg_name_to_mask>
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER MASKING POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterMaskingPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("MASKING", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET BODY -> <expression>
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("SET", "BODY") },
						func() bool { return v.MatchOp("-") },
						func() bool { return v.MatchOp(">") },
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterMaterializedView validates the Snowflake `ALTER MATERIALIZED VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-materialized-view
//
// Syntax:
//
//	ALTER MATERIALIZED VIEW <name>
//	  {
//	  RENAME TO <new_name>                     |
//	  CLUSTER BY ( <expr1> [, <expr2> ... ] )  |
//	  DROP CLUSTERING KEY                      |
//	  SUSPEND RECLUSTER                        |
//	  RESUME RECLUSTER                         |
//	  SUSPEND                                  |
//	  RESUME                                   |
//	  SET {
//	    [ SECURE ]
//	    [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	    [ COMMENT = '<comment>' ]
//	    }                                      |
//	  UNSET {
//	    SECURE
//	    CONTACT <purpose>                                 |
//	    COMMENT
//	    }
//	  }
//
//	ALTER MATERIALIZED VIEW
//	  SET DATA_METRIC_SCHEDULE = {
//	      '<num> MINUTE'
//	    | 'USING CRON <expr> <time_zone>'
//	  }
//
//	ALTER MATERIALIZED VIEW UNSET DATA_METRIC_SCHEDULE
func (v *Validator) ParseAlterMaterializedView() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("MATERIALIZED", "VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("CLUSTER", "BY") },
						v.consumeBalancedParens,
					)
				},
				func() bool { return v.phrase("DROP", "CLUSTERING", "KEY") },
				func() bool {
					return v.Sequence(v.wordsValue("SUSPEND", "RESUME"), func() bool { return v.MatchWord("RECLUSTER") })
				},
				v.wordsValue("SUSPEND", "RESUME"),
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterModel validates the Snowflake `ALTER MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model
//
// Syntax:
//
//	ALTER MODEL [ IF EXISTS ] <name> SET
//	  [ COMMENT = '<string_literal>' ]
//	  [ DEFAULT_VERSION = '<version_name>']
//
//	ALTER MODEL [ IF EXISTS ] <model_name> SET TAG <tag_name> = '<tag_value>'
//
//	ALTER MODEL [ IF EXISTS ] <model_name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER MODEL [ IF EXISTS ] <model_name> VERSION <version_name> SET ALIAS = <alias_name>
//
//	ALTER MODEL [ IF EXISTS ] <model_name> VERSION <version_or_alias_name> UNSET ALIAS
//
//	ALTER MODEL <model_name> RENAME TO <new_name>
func (v *Validator) ParseAlterModel() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// VERSION <name> { SET ALIAS = <name> | UNSET ALIAS }
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("VERSION") },
						v.parseIdentPath,
						func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterModelAddVersion validates the Snowflake `ALTER MODEL ADD VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-add-version
//
// Syntax:
//
//	ALTER MODEL [ IF EXISTS ] <name> ADD VERSION <version_name>
//	  FROM MODEL <source_model_name> [ VERSION <source_version_name> ]
//
//	ALTER MODEL [ IF EXISTS ] <name> ADD VERSION <version_name> FROM internalStage
//
//	Where:
//
//	internalStage ::=
//	    @[<namespace>.]<int_stage_name>[/<path>]
//	| @[<namespace>.]%<table_name>[/<path>]
//	| @~[/<path>]
func (v *Validator) ParseAlterModelAddVersion() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("ADD", "VERSION") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		// FROM MODEL <name> [ VERSION <name> ] | internalStage (@...)
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterModelDropVersion validates the Snowflake `ALTER MODEL DROP VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-drop-version
//
// Syntax:
//
//	ALTER MODEL [ IF EXISTS ] <name> DROP VERSION <version_name>
func (v *Validator) ParseAlterModelDropVersion() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("DROP", "VERSION") },
		v.parseIdentPath,
	)
}

// ParseAlterModelModifyVersion validates the Snowflake `ALTER MODEL MODIFY VERSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-modify-version
//
// Syntax:
//
//	ALTER MODEL [ IF EXISTS ] <name> MODIFY VERSION <version_or_alias_name> SET
//	  [ COMMENT = '<string_literal>' ]
//	  [ METADATA = '<json_metadata>']
func (v *Validator) ParseAlterModelModifyVersion() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("MODEL") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.phrase("MODIFY", "VERSION") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("SET") },
		// [ COMMENT = ... ] [ METADATA = ... ]
		func() bool {
			return v.ZeroOrMore(func() bool {
				return v.Choice(
					v.option("COMMENT", v.parseString),
					v.option("METADATA", v.parseString),
				)
			})
		},
	)
}

// ParseAlterModelMonitor validates the Snowflake `ALTER MODEL MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-model-monitor
//
// Syntax:
//
//	ALTER MODEL MONITOR [ IF EXISTS ] <monitor_name> { SUSPEND | RESUME }
//
//	ALTER MODEL MONITOR [ IF EXISTS ] <monitor_name> SET
//	   [ BASELINE='<baseline_table_name>' ]
//	   [ REFRESH_INTERVAL='<refresh_interval>' ]
//	   [ WAREHOUSE=<warehouse_name> ]
//
//	ALTER MODEL MONITOR [ IF EXISTS ] <monitor_name> ADD segment_column = '<segment_column_name>'
//
//	ALTER MODEL MONITOR [ IF EXISTS ] <monitor_name> DROP segment_column = '<segment_column_name>'
func (v *Validator) ParseAlterModelMonitor() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("MODEL", "MONITOR") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("SUSPEND", "RESUME"),
				func() bool { return v.MatchWord("SET") && v.consumeRest() },
				// ADD / DROP segment_column = '<name>'
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "DROP"),
						func() bool { return v.MatchWord("SEGMENT_COLUMN") },
						func() bool { return v.MatchOp("=") },
						func() bool { return v.parseString() },
					)
				},
			)
		},
	)
}

// ParseAlterNetworkPolicy validates the Snowflake `ALTER NETWORK POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-network-policy
//
// Syntax:
//
//	ALTER NETWORK POLICY [ IF EXISTS ] <name> SET {
//	    [ ALLOWED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	    [ BLOCKED_NETWORK_RULE_LIST = ( '<network_rule>' [ , '<network_rule>' , ... ] ) ]
//	    [ ALLOWED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' ... ] ) ]
//	    [ BLOCKED_IP_LIST = ( [ '<ip_address>' ] [ , '<ip_address>' ... ] ) ]
//	    [ COMMENT = '<string_literal>' ] }
//
//	ALTER NETWORK POLICY [ IF EXISTS ] <name> UNSET COMMENT
//
//	ALTER NETWORK POLICY <name> ADD { ALLOWED_NETWORK_RULE_LIST = '<network_rule>' | BLOCKED_NETWORK_RULE_LIST = '<network_rule>' }
//
//	ALTER NETWORK POLICY <name> REMOVE { ALLOWED_NETWORK_RULE_LIST = '<network_rule>' | BLOCKED_NETWORK_RULE_LIST = '<network_rule>' }
//
//	ALTER NETWORK POLICY <name> RENAME TO <new_name>
//
//	ALTER NETWORK POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER NETWORK POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterNetworkPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("NETWORK", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// ADD / REMOVE { ALLOWED_NETWORK_RULE_LIST | BLOCKED_NETWORK_RULE_LIST } = '<rule>'
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "REMOVE"),
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterNetworkRule validates the Snowflake `ALTER NETWORK RULE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-network-rule
//
// Syntax:
//
//	ALTER NETWORK RULE [ IF EXISTS ] <name> SET
//	  VALUE_LIST = ( '<value>'  [ , '<value>', ... ] )
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER NETWORK RULE [ IF EXISTS ] <name> UNSET { VALUE_LIST | COMMENT }
func (v *Validator) ParseAlterNetworkRule() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("NETWORK", "RULE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotebook validates the Snowflake `ALTER NOTEBOOK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notebook
//
// Syntax:
//
//	ALTER NOTEBOOK [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER NOTEBOOK [ IF EXISTS ] <name> SET
//	  [ COMMENT = '<string_literal>' ]
//	  [ QUERY_WAREHOUSE = <warehouse_to_run_nb_and_sql_queries_in> ]
//	  [ IDLE_AUTO_SHUTDOWN_TIME_SECONDS = <number_of_seconds> ]
//	  [ SECRETS = ('<secret_variable_name>' = <secret_name>) [ , ... ] ]
func (v *Validator) ParseAlterNotebook() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("NOTEBOOK") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterNotificationIntegration validates the Snowflake `ALTER NOTIFICATION INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration
//
// Syntax: (unavailable — see Reference URL)
func (v *Validator) ParseAlterNotificationIntegration() bool {
	// Syntax unavailable upstream: require the leading skeleton, accept the rest.
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterNotificationIntegrationEmail validates the Snowflake `ALTER NOTIFICATION INTEGRATION (email)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-email
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ ALLOWED_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_RECIPIENTS = ( '<email_address>' [ , ... '<email_address>' ] ) ]
//	  [ DEFAULT_SUBJECT = '<subject_line>' ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET
//	  ENABLED            |
//	  ALLOWED_RECIPIENTS |
//	  DEFAULT_RECIPIENTS |
//	  DEFAULT_SUBJECT    |
//	  COMMENT
func (v *Validator) ParseAlterNotificationIntegrationEmail() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationInboundAzureEventGrid validates the Snowflake `ALTER NOTIFICATION INTEGRATION (inbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-inbound-azure
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterNotificationIntegrationInboundAzureEventGrid() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationInboundGooglePubSub validates the Snowflake `ALTER NOTIFICATION INTEGRATION (inbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-inbound-gcp
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterNotificationIntegrationInboundGooglePubSub() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationOutboundAmazonSns validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Amazon SNS)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-aws
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  AWS_SNS_TOPIC_ARN = '<topic_arn>'
//	  AWS_SNS_ROLE_ARN = '<iam_role_arn>'
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterNotificationIntegrationOutboundAmazonSns() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationOutboundAzureEventGrid validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Azure Event Grid)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-azure
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  AZURE_STORAGE_QUEUE_PRIMARY_URI = '<queue_URL>'
//	  AZURE_TENANT_ID = '<directory_ID>';
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterNotificationIntegrationOutboundAzureEventGrid() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationOutboundGooglePubSub validates the Snowflake `ALTER NOTIFICATION INTEGRATION (outbound Google Pub/Sub)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-queue-outbound-gcp
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  GCP_PUBSUB_SUBSCRIPTION_NAME = '<subscription_id>'
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterNotificationIntegrationOutboundGooglePubSub() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterNotificationIntegrationWebhooks validates the Snowflake `ALTER NOTIFICATION INTEGRATION (webhooks)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-notification-integration-webhooks
//
// Syntax:
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ WEBHOOK_URL = '<url>' ]
//	  [ WEBHOOK_SECRET = <secret_name> ]
//	  [ WEBHOOK_BODY_TEMPLATE = '<template_for_http_request_body>' ]
//	  [ WEBHOOK_HEADERS = ( '<header_1>'='<value_1>' [ , '<header_N>'='<value_N>', ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ NOTIFICATION ] INTEGRATION [ IF EXISTS ] <name> UNSET {
//	  ENABLED               |
//	  WEBHOOK_SECRET        |
//	  WEBHOOK_BODY_TEMPLATE |
//	  WEBHOOK_HEADERS       |
//	  COMMENT
//	}
func (v *Validator) ParseAlterNotificationIntegrationWebhooks() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("NOTIFICATION") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterOpenflowDataPlane validates the Snowflake `ALTER OPENFLOW DATA PLANE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-oflow-data-plane
//
// Syntax:
//
//	ALTER OPENFLOW DATA PLANE INTEGRATION <name>
//	    SET EVENT_TABLE = '<database>.<schema>.<tablename>';
func (v *Validator) ParseAlterOpenflowDataPlane() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("OPENFLOW", "DATA", "PLANE", "INTEGRATION") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("SET") },
		v.option("EVENT_TABLE", v.parseScalar),
	)
}

// ParseAlterOnlineFeatureTable validates the Snowflake `ALTER ONLINE FEATURE TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-online-feature-table
//
// Syntax:
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> { SUSPEND | RESUME }
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> REFRESH
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> SET
//	  [ TARGET_LAG = '<num> { seconds | minutes | hours | days }' ]
//	  [ WAREHOUSE = <warehouse_name> ]
//
//	ALTER ONLINE FEATURE TABLE [ IF EXISTS ] <name> <tagAction>
func (v *Validator) ParseAlterOnlineFeatureTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ONLINE", "FEATURE", "TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("SUSPEND", "RESUME", "REFRESH"),
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// <tagAction> (SET/UNSET TAG) and SET COMMENT/TARGET_LAG/WAREHOUSE are
				// all covered by this SET/UNSET branch — no ungated catch-all, so a
				// garbage action is flagged.
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterOrganizationAccount validates the Snowflake `ALTER ORGANIZATION ACCOUNT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-account
//
// Syntax:
//
//	ALTER ORGANIZATION ACCOUNT SET { [ accountParams ] | [ objectParams ] | [ sessionParams ] }
//
//	ALTER ORGANIZATION ACCOUNT UNSET <param_name> [ , ... ]
//
//	ALTER ORGANIZATION ACCOUNT SET RESOURCE_MONITOR = <monitor_name>
//
//	ALTER ORGANIZATION ACCOUNT SET { PASSWORD | SESSION } POLICY <policy_name>
//
//	ALTER ORGANIZATION ACCOUNT UNSET { PASSWORD | SESSION } POLICY
//
//	ALTER ORGANIZATION ACCOUNT SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ORGANIZATION ACCOUNT UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterOrganizationAccount() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ORGANIZATION", "ACCOUNT") },
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterOrganizationProfile validates the Snowflake `ALTER ORGANIZATION PROFILE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-profile
//
// Syntax:
//
//	ALTER ORGANIZATION PROFILE [ IF EXISTS ] <name> AS '<yaml_manifest_string>'
//
//	ALTER ORGANIZATION PROFILE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER ORGANIZATION PROFILE [ IF EXISTS ] <name> PUBLISH
//
//	ALTER ORGANIZATION PROFILE <name> ADD VERSION [ [ IF NOT EXISTS ] <version_alias_name> ]
//	  FROM @<yaml_manifest_stage_location>
//
//	ALTER ORGANIZATION PROFILE <name> ADD LIVE VERSION [ [ IF NOT EXISTS ] <version_alias_name> ]
//	  FROM LAST
//
//	ALTER ORGANIZATION PROFILE <name> COMMIT
//
//	ALTER ORGANIZATION PROFILE <name> ABORT
func (v *Validator) ParseAlterOrganizationProfile() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ORGANIZATION", "PROFILE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("AS") && v.parseString() },
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				v.wordsValue("PUBLISH", "COMMIT", "ABORT"),
				// ADD [ LIVE ] VERSION ... FROM ...
				func() bool { return v.MatchWord("ADD") && v.consumeRest() },
			)
		},
	)
}

// ParseAlterOrganizationUser validates the Snowflake `ALTER ORGANIZATION USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-user
//
// Syntax:
//
//	ALTER ORGANIZATION USER [ IF EXISTS ] <name> SET [ objectProperties ]
//
//	ALTER ORGANIZATION USER <name> UNSET [ objectProperties ]
//
//	Where:
//
//	objectProperties ::=
//	  EMAIL = '<string>'
//	  DISPLAY_NAME = '<string>'
//	  FIRST_NAME = '<string>'
//	  MIDDLE_NAME = '<string>'
//	  LAST_NAME = '<string>'
//	  COMMENT = '<string>'
func (v *Validator) ParseAlterOrganizationUser() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ORGANIZATION", "USER") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterOrganizationUserGroup validates the Snowflake `ALTER ORGANIZATION USER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-user-group
//
// Syntax:
//
//	ALTER ORGANIZATION USER GROUP [ IF EXISTS ] <name> ADD ORGANIZATION USERS <org_user> [ , <org_user> ... ]
//
//	ALTER ORGANIZATION USER GROUP [ IF EXISTS ] <name> REMOVE ORGANIZATION USERS <org_user> [ , <org_user> ... ]
//
//	ALTER ORGANIZATION USER GROUP [ IF EXISTS ] <name> SET VISIBILITY =
//	  { ALL
//	  | ACCOUNTS <account> [ , <account> ... ]
//	  | REGION GROUPS '<region_group>' [ , '<region_group>' ... ]
//	  }
//
//	ALTER ORGANIZATION USER GROUP [ IF EXISTS ] <name> SET IS_GRANTABLE = { TRUE | FALSE }
func (v *Validator) ParseAlterOrganizationUserGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ORGANIZATION", "USER", "GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.wordsValue("ADD", "REMOVE")() && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterPackagesPolicy validates the Snowflake `ALTER PACKAGES POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-packages-policy
//
// Syntax:
//
//	ALTER PACKAGES POLICY [ IF EXISTS ] <name> SET
//	  [ ALLOWLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ ADDITIONAL_CREATION_BLOCKLIST = ( [ '<packageSpec>' ] [ , '<packageSpec>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER PACKAGES POLICY [ IF EXISTS ] <name> UNSET
//	  [ ALLOWLIST ]
//	  [ BLOCKLIST ]
//	  [ ADDITIONAL_CREATION_BLOCKLIST ]
//	  [ COMMENT ]
func (v *Validator) ParseAlterPackagesPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("PACKAGES", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterPasswordPolicy validates the Snowflake `ALTER PASSWORD POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-password-policy
//
// Syntax:
//
//	ALTER PASSWORD POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER PASSWORD POLICY [ IF EXISTS ] <name> SET [ PASSWORD_MIN_LENGTH = <integer> ]
//	                                               [ PASSWORD_MAX_LENGTH = <integer> ]
//	                                               [ PASSWORD_MIN_UPPER_CASE_CHARS = <integer> ]
//	                                               [ PASSWORD_MIN_LOWER_CASE_CHARS = <integer> ]
//	                                               [ PASSWORD_MIN_NUMERIC_CHARS = <integer> ]
//	                                               [ PASSWORD_MIN_SPECIAL_CHARS = <integer> ]
//	                                               [ PASSWORD_MIN_AGE_DAYS = <integer> ]
//	                                               [ PASSWORD_MAX_AGE_DAYS = <integer> ]
//	                                               [ PASSWORD_MAX_RETRIES = <integer> ]
//	                                               [ PASSWORD_LOCKOUT_TIME_MINS = <integer> ]
//	                                               [ PASSWORD_HISTORY = <integer> ]
//	                                               [ COMMENT = '<string_literal>' ]
//
//	ALTER PASSWORD POLICY [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER PASSWORD POLICY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER PASSWORD POLICY [ IF EXISTS ] <name> UNSET [ PASSWORD_MIN_LENGTH ]
//	                                                 [ PASSWORD_MAX_LENGTH ]
//	                                                 [ PASSWORD_MIN_UPPER_CASE_CHARS ]
//	                                                 [ PASSWORD_MIN_LOWER_CASE_CHARS ]
//	                                                 [ PASSWORD_MIN_NUMERIC_CHARS ]
//	                                                 [ PASSWORD_MIN_SPECIAL_CHARS ]
//	                                                 [ PASSWORD_MIN_AGE_DAYS ]
//	                                                 [ PASSWORD_MAX_AGE_DAYS ]
//	                                                 [ PASSWORD_MAX_RETRIES ]
//	                                                 [ PASSWORD_LOCKOUT_TIME_MINS ]
//	                                                 [ PASSWORD_HISTORY ]
//	                                                 [ COMMENT ]
func (v *Validator) ParseAlterPasswordPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("PASSWORD", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterPipe validates the Snowflake `ALTER PIPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-pipe
//
// Syntax:
//
//	ALTER PIPE [ IF EXISTS ] <name> SET { [ objectProperties ]
//	                                      [ COMMENT = '<string_literal>' ] }
//
//	ALTER PIPE <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER PIPE <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER PIPE [ IF EXISTS ] <name> UNSET { <property_name> | COMMENT } [ , ... ]
//
//	ALTER PIPE [ IF EXISTS ] <name> REFRESH { [ PREFIX = '<path>' ] [ MODIFIED_AFTER = <start_time> ] }
//
//	Where:
//
//	objectProperties ::=
//	  PIPE_EXECUTION_PAUSED = TRUE | FALSE
func (v *Validator) ParseAlterPipe() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("PIPE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("REFRESH") && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterPostgresInstance validates the Snowflake `ALTER POSTGRES INSTANCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-postgres-instance
//
// Syntax:
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name>
//	  RENAME TO <new_name>
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> SET
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ AUTHENTICATION_AUTHORITY = { POSTGRES | POSTGRES_OR_SNOWFLAKE } ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ HIGH_AVAILABILITY = { TRUE | FALSE } ]
//	  [ COMPUTE_FAMILY = '<compute_family>' ]
//	  [ STORAGE_SIZE_GB = <storage_gb> ]
//	  [ STORAGE_INTEGRATION = '<storage_integration_name>' ]
//	  [ POSTGRES_VERSION = { 16 | 17 | 18 } ]
//	  [ MAINTENANCE_WINDOW_START = <hour_of_day> ]
//	  [ POSTGRES_SETTINGS = '<json_string>' ]
//	  [ APPLY { IMMEDIATELY | ON '<timestamp>' } ]
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name>
//	  UNSET { COMMENT | POSTGRES_SETTINGS | NETWORK_POLICY
//	    | MAINTENANCE_WINDOW_START | STORAGE_INTEGRATION } [ , ... ]
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> SUSPEND
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> RESUME
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> RESET ACCESS
//	  FOR { 'snowflake_admin' | 'application' }
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> SET TAG <tag_name> =
//	  '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER POSTGRES INSTANCE [ IF EXISTS ] <name> UNSET TAG <tag_name>
//	  [ , <tag_name> ... ]
func (v *Validator) ParseAlterPostgresInstance() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("POSTGRES", "INSTANCE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				v.wordsValue("SUSPEND", "RESUME"),
				func() bool { return v.phrase("RESET", "ACCESS") && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterPrivacyPolicy validates the Snowflake `ALTER PRIVACY POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-privacy-policy
//
// Syntax:
//
//	ALTER PRIVACY POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER PRIVACY POLICY [ IF EXISTS ] <name> SET BODY -> <expression>
//
//	ALTER PRIVACY POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER PRIVACY POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER PRIVACY POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER PRIVACY POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterPrivacyPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("PRIVACY", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET BODY -> <expression>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SET") },
						func() bool { return v.MatchWord("BODY") },
						func() bool { return v.MatchOp("-") },
						func() bool { return v.MatchOp(">") },
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterProcedure validates the Snowflake `ALTER PROCEDURE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-procedure
//
// Syntax:
//
//	-- Java / Python / Scala handlers:
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) RENAME TO <new_name>
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET
//	  [ LOG_LEVEL = '<log_level>' ]
//	  [ TRACE_LEVEL = '<trace_level>' ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = '<integration_name>' [ , '<integration_name>' ... ] ]
//	  [ SECRETS = '<secret_variable_name>' = <secret_name> [ , '<secret_variable_name>' = <secret_name> ... ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) UNSET COMMENT
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER PROCEDURE [ IF EXISTS ] <name> ( [ <arg_data_type> , ... ] ) EXECUTE AS { OWNER | CALLER | RESTRICTED CALLER }
//
//	-- JavaScript / Snowflake Scripting handlers omit EXTERNAL_ACCESS_INTEGRATIONS and SECRETS in the SET block
//	-- (Snowflake Scripting additionally supports AUTO_EVENT_LOGGING = '<option>').
func (v *Validator) ParseAlterProcedure() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("PROCEDURE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// ( [ <arg_data_type> , ... ] )
		v.consumeBalancedParens,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("EXECUTE", "AS") && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterProjectionPolicy validates the Snowflake `ALTER PROJECTION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-projection-policy
//
// Syntax:
//
//	ALTER PROJECTION POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER PROJECTION POLICY [ IF EXISTS ] <name> SET BODY -> <expression>
//
//	ALTER PROJECTION POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER PROJECTION POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER PROJECTION POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER PROJECTION POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterProjectionPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("PROJECTION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET BODY -> <expression>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SET") },
						func() bool { return v.MatchWord("BODY") },
						func() bool { return v.MatchOp("-") },
						func() bool { return v.MatchOp(">") },
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterReplicationGroup validates the Snowflake `ALTER REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-replication-group
//
// Syntax:
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  [ OBJECT_TYPES = <object_type> [ , <object_type> , ... ] ]
//	  [ ALLOWED_DATABASES = <db_name> [ , <db_name> , ... ] ]
//	  [ ALLOWED_EXTERNAL_VOLUMES = <external_volume_name> [ , <external_volume_name> , ... ] ]
//	  [ ALLOWED_SHARES = <share_name> [ , <share_name> , ... ] ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  OBJECT_TYPES = INTEGRATIONS [ , <object_type> , ... ]
//	  ALLOWED_INTEGRATION_TYPES = <integration_type_name> [ , <integration_type_name> ... ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  COMMENT = '<string_literal>'
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  REPLICATION_SCHEDULE = '{ <num> MINUTE | USING CRON <expr> <time_zone> }'
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  ERROR_INTEGRATION = <integration_name>
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SET
//	  TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> UNSET
//	  { COMMENT | REPLICATION_SCHEDULE | ERROR_INTEGRATION } [ , ... ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> UNSET
//	  TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  ADD <db_name> [ , <db_name> ,  ... ] TO ALLOWED_DATABASES
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  MOVE DATABASES <db_name> [ , <db_name> ,  ... ] TO REPLICATION GROUP <move_to_rg_name>
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  REMOVE <db_name> [ , <db_name> ,  ... ] FROM ALLOWED_DATABASES
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  ADD <share_name> [ , <share_name> ,  ... ] TO ALLOWED_SHARES
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  MOVE SHARES <share_name> [ , <share_name> ,  ... ] TO REPLICATION GROUP <move_to_rg_name>
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  REMOVE <share_name> [ , <share_name> ,  ... ] FROM ALLOWED_SHARES
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  ADD <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ] TO ALLOWED_ACCOUNTS
//	  [ IGNORE EDITION CHECK ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name>
//	  REMOVE <org_name>.<target_account_name> [ , <org_name>.<target_account_name> ,  ... ] FROM ALLOWED_ACCOUNTS
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> REFRESH
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> SUSPEND [ IMMEDIATE ]
//
//	ALTER REPLICATION GROUP [ IF EXISTS ] <name> RESUME
func (v *Validator) ParseAlterReplicationGroup() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("REPLICATION", "GROUP") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				v.wordsValue("REFRESH", "RESUME"),
				func() bool {
					return v.MatchWord("SUSPEND") && v.Optional(func() bool { return v.MatchWord("IMMEDIATE") })
				},
				func() bool { return v.wordsValue("ADD", "MOVE", "REMOVE")() && v.consumeRest() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterResourceMonitor validates the Snowflake `ALTER RESOURCE MONITOR` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-resource-monitor
//
// Syntax:
//
//	ALTER RESOURCE MONITOR [ IF EXISTS ] <name> [ SET { [ CREDIT_QUOTA = <num> ]
//	                                                    [ FREQUENCY = { MONTHLY | DAILY | WEEKLY | YEARLY | NEVER } ]
//	                                                    [ START_TIMESTAMP = { <timestamp> | IMMEDIATELY } ]
//	                                                    [ END_TIMESTAMP = <timestamp> ]
//	                                                    [ NOTIFY_USERS = ( <user_name> [ , <user_name> , ... ] ) ] } ]
//	                                            [ TRIGGERS triggerDefinition [ triggerDefinition ... ] ]
//
//	Where:
//
//	triggerDefinition ::=
//	   ON <threshold> PERCENT DO { SUSPEND | SUSPEND_IMMEDIATE | NOTIFY }
func (v *Validator) ParseAlterResourceMonitor() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("RESOURCE", "MONITOR") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// [ SET <opts> ] [ TRIGGERS <triggerDefinition> ... ] — both optional.
		func() bool {
			return v.Optional(func() bool {
				return v.Choice(
					func() bool { return v.MatchWord("SET") && v.consumeRest() },
					func() bool { return v.MatchWord("TRIGGERS") && v.consumeRest() },
				)
			})
		},
	)
}

// ParseAlterRole validates the Snowflake `ALTER ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-role
//
// Syntax:
//
//	ALTER ROLE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER ROLE [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER ROLE [ IF EXISTS ] <name> UNSET COMMENT
//
//	ALTER ROLE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ROLE [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER ROLE [ IF EXISTS ] <name> UNSET DCM PROJECT
func (v *Validator) ParseAlterRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("ROLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterRowAccessPolicy validates the Snowflake `ALTER ROW ACCESS POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-row-access-policy
//
// Syntax:
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> SET BODY -> <expression_on_arg_name>
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER ROW ACCESS POLICY [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterRowAccessPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ROW", "ACCESS", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET BODY -> <expression>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SET") },
						func() bool { return v.MatchWord("BODY") },
						func() bool { return v.MatchOp("-") },
						func() bool { return v.MatchOp(">") },
						v.consumeRest,
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSchema validates the Snowflake `ALTER SCHEMA` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-schema
//
// Syntax:
//
//	ALTER SCHEMA [ IF EXISTS ] <name> RENAME TO <new_schema_name>
//
//	ALTER SCHEMA [ IF EXISTS ] <name> SWAP WITH <target_schema_name>
//
//	ALTER SCHEMA [ IF EXISTS ] <name> SET {
//	                                      [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	                                      [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	                                      [ EXTERNAL_VOLUME = <external_volume_name> ]
//	                                      [ CATALOG = <catalog_integration_name> ]
//	                                      [ ICEBERG_VERSION_DEFAULT = <integer> ]
//	                                      [ ICEBERG_MERGE_ON_READ_BEHAVIOR = { 'AUTO' | 'ENABLED' | 'DISABLED' } ]
//	                                      [ ENABLE_ICEBERG_MERGE_ON_READ = { TRUE | FALSE } ]
//	                                      [ REPLACE_INVALID_CHARACTERS = { TRUE | FALSE } ]
//	                                      [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	                                      [ DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU = '<compute_pool_name>' ]
//	                                      [ DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU = '<compute_pool_name>' ]
//	                                      [ LOG_LEVEL = '<log_level>' ]
//	                                      [ TRACE_LEVEL = '<trace_level>' ]
//	                                      [ STORAGE_SERIALIZATION_POLICY = { COMPATIBLE | OPTIMIZED } ]
//	                                      [ CLASSIFICATION_PROFILE = '<profile_name>' ]
//	                                      [ COMMENT = '<string_literal>' ]
//	                                      [ CATALOG_SYNC = '<snowflake_open_catalog_integration_name>' ]
//	                                      [ REPLICABLE_WITH_FAILOVER_GROUPS = { 'YES' | 'NO' } ]
//	                                      [ BASE_LOCATION_PREFIX = '<string>']
//	                                      [ DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE = '<warehouse_name>']
//	                                      [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	                                      [ OBJECT_VISIBILITY = PRIVILEGED } ]
//	                                      [ ENABLE_DATA_COMPACTION = { TRUE | FALSE } ]
//	                                      }
//
//	ALTER SCHEMA [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SCHEMA [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER SCHEMA [ IF EXISTS ] <name> UNSET {
//	                                        DATA_RETENTION_TIME_IN_DAYS         |
//	                                        MAX_DATA_EXTENSION_TIME_IN_DAYS     |
//	                                        EXTERNAL_VOLUME                     |
//	                                        CATALOG                             |
//	                                        ICEBERG_VERSION_DEFAULT             |
//	                                        ICEBERG_MERGE_ON_READ_BEHAVIOR      |
//	                                        ENABLE_ICEBERG_MERGE_ON_READ        |
//	                                        REPLACE_INVALID_CHARACTERS          |
//	                                        DEFAULT_DDL_COLLATION               |
//	                                        LOG_LEVEL                           |
//	                                        TRACE_LEVEL                         |
//	                                        STORAGE_SERIALIZATION_POLICY        |
//	                                        COMMENT                             |
//	                                        CATALOG_SYNC                        |
//	                                        REPLICABLE_WITH_FAILOVER_GROUPS     |
//	                                        BASE_LOCATION_PREFIX                |
//	                                        DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE|
//	                                        CONTACT <purpose>                   |
//	                                        CLASSIFICATION_PROFILE              |
//	                                        OBJECT_VISIBILITY                   |
//	                                        ENABLE_DATA_COMPACTION              |
//	                                        DCM PROJECT
//	                                        }
//	                                        [ , ... ]
//
//	ALTER SCHEMA [ IF EXISTS ] <name> { ENABLE | DISABLE } MANAGED ACCESS
func (v *Validator) ParseAlterSchema() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SCHEMA") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("SWAP", "WITH") && v.parseIdentPath() },
				// { ENABLE | DISABLE } MANAGED ACCESS
				func() bool { return v.wordsValue("ENABLE", "DISABLE")() && v.phrase("MANAGED", "ACCESS") },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSecret validates the Snowflake `ALTER SECRET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-secret
//
// Syntax:
//
//	ALTER SECRET [ IF EXISTS ] <name> SET [ OAUTH_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] ) ]
//	                                      [ COMMENT = '<string_literal>' ]
//
//	ALTER SECRET [ IF EXISTS ] <name> UNSET COMMENT
//
//	ALTER SECRET [ IF EXISTS ] <name> SET [ OAUTH_REFRESH_TOKEN = '<token>' ]
//	                                      [ OAUTH_REFRESH_TOKEN_EXPIRY_TIME = '<string_literal>' ]
//	                                      [ COMMENT = '<string_literal>' ]
//
//	ALTER SECRET [ IF EXISTS ] <name> SET [ API_AUTHENTICATION = '<cloud_provider_security_integration>' ]
//	                                      [ COMMENT = '<string_literal>' ]
//
//	ALTER SECRET [ IF EXISTS ] <name> SET [ USERNAME = '<username>' ]
//	                                      [ PASSWORD = '<password>' ]
//	                                      [ COMMENT = '<string_literal>' ]
//
//	ALTER SECRET [ IF EXISTS ] <name> SET [ SECRET_STRING = '<string_literal>' ]
//	                                      [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseAlterSecret() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SECRET") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSecurityIntegration validates the Snowflake `ALTER SECURITY INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration
//
// Syntax:
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET <parameters>
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name>  UNSET <parameter>
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegration() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSecurityIntegrationExternalApiAuthentication validates the Snowflake `ALTER SECURITY INTEGRATION (External API Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-api-auth
//
// Syntax:
//
//	-- OAuth: Client credentials
//
//	ALTER SECURITY INTEGRATION <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_TOKEN_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_CLIENT_AUTH_METHOD = { CLIENT_SECRET_BASIC | CLIENT_SECRET_POST } ]
//	  [ OAUTH_CLIENT_ID = '<string_literal>' ]
//	  [ OAUTH_CLIENT_SECRET = '<string_literal>' ]
//	  [ OAUTH_GRANT = 'CLIENT_CREDENTIALS']
//	  [ OAUTH_ACCESS_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_ALLOWED_SCOPES = ( '<scope_1>' [ , '<scope_2>' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- OAuth: Authorization code grant flow
//
//	ALTER SECURITY INTEGRATION <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_AUTHORIZATION_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_TOKEN_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_CLIENT_AUTH_METHOD = { CLIENT_SECRET_BASIC | CLIENT_SECRET_POST } ]
//	  [ OAUTH_CLIENT_ID = '<string_literal>' ]
//	  [ OAUTH_CLIENT_SECRET = '<string_literal>' ]
//	  [ OAUTH_GRANT = 'AUTHORIZATION_CODE']
//	  [ OAUTH_ACCESS_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- OAuth: JWT bearer flow
//
//	ALTER SECURITY INTEGRATION <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_AUTHORIZATION_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_TOKEN_ENDPOINT = '<string_literal>' ]
//	  [ OAUTH_CLIENT_AUTH_METHOD = { CLIENT_SECRET_BASIC | CLIENT_SECRET_POST } ]
//	  [ OAUTH_CLIENT_ID = '<string_literal>' ]
//	  [ OAUTH_CLIENT_SECRET = '<string_literal>' ]
//	  [ OAUTH_GRANT = 'JWT_BEARER']
//	  [ OAUTH_ACCESS_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> UNSET { ENABLED | [ , ... ] }
func (v *Validator) ParseAlterSecurityIntegrationExternalApiAuthentication() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSecurityIntegrationAwsIamAuthentication validates the Snowflake `ALTER SECURITY INTEGRATION (AWS IAM Authentication)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-aws-iam
//
// Syntax:
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ TYPE = AWS_IAM ]
//	  [ AWS_ROLE_ARN = '<iam_role_arn>' ]
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegrationAwsIamAuthentication() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSecurityIntegrationExternalOauth validates the Snowflake `ALTER SECURITY INTEGRATION (External OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-oauth-external
//
// Syntax:
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ TYPE = EXTERNAL_OAUTH ]
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ EXTERNAL_OAUTH_TYPE = { OKTA | AZURE | PING_FEDERATE | CUSTOM } ]
//	  [ EXTERNAL_OAUTH_ISSUER = '<string_literal>' ]
//	  [ EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = '<string_literal>' | ('<string_literal>', '<string_literal>' [ , ... ] ) ]
//	  [ EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = 'LOGIN_NAME | EMAIL_ADDRESS' ]
//	  [ EXTERNAL_OAUTH_JWS_KEYS_URL = '<string_literal>' ] -- For OKTA | PING_FEDERATE | CUSTOM
//	  [ EXTERNAL_OAUTH_JWS_KEYS_URL = '<string_literal>' | ('<string_literal>' [ , '<string_literal>' ... ] ) ] -- For Azure
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY = <public_key1> ]
//	  [ EXTERNAL_OAUTH_RSA_PUBLIC_KEY_2 = <public_key2> ]
//	  [ EXTERNAL_OAUTH_BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ EXTERNAL_OAUTH_AUDIENCE_LIST = ('<string_literal>') ]
//	  [ EXTERNAL_OAUTH_ANY_ROLE_MODE = DISABLE | ENABLE | ENABLE_FOR_PRIVILEGE ]
//	  [ EXTERNAL_OAUTH_SCOPE_DELIMITER = '<string_literal>' ] -- Only for EXTERNAL_OAUTH_TYPE = CUSTOM
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name>  UNSET {
//	                                                            ENABLED                      |
//	                                                            EXTERNAL_OAUTH_AUDIENCE_LIST |
//	                                                            }
//	                                                            [ , ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegrationExternalOauth() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSecurityIntegrationSnowflakeOauth validates the Snowflake `ALTER SECURITY INTEGRATION (Snowflake OAuth)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-oauth-snowflake
//
// Syntax:
//
//	-- Snowflake OAuth for partner applications
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_ISSUE_REFRESH_TOKENS = { TRUE | FALSE } ]
//	  [ OAUTH_REDIRECT_URI ] = '<uri>'
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED = { TRUE | FALSE } ]
//	  [ OAUTH_USE_SECONDARY_ROLES = { IMPLICIT | NONE } ]
//	  [ ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	-- Snowflake OAuth for custom clients
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ OAUTH_REDIRECT_URI = '<uri>' ]
//	  [ OAUTH_ALLOW_NON_TLS_REDIRECT_URI = { TRUE | FALSE } ]
//	  [ OAUTH_ALTERNATE_REDIRECT_URIS = ( '<uri>' [ , '<uri>' , ... ] ) ]
//	  [ OAUTH_ENFORCE_PKCE = { TRUE | FALSE } ]
//	  [ PRE_AUTHORIZED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ ALLOWED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ BLOCKED_ROLES_LIST = ( '<role_name>' [ , '<role_name>' , ... ] ) ]
//	  [ OAUTH_ISSUE_REFRESH_TOKENS = { TRUE | FALSE } ]
//	  [ OAUTH_REFRESH_TOKEN_VALIDITY = <integer> ]
//	  [ OAUTH_SINGLE_USE_REFRESH_TOKENS_REQUIRED = { TRUE | FALSE } ]
//	  [ OAUTH_USE_SECONDARY_ROLES = IMPLICIT | NONE ]
//	  [ NETWORK_POLICY = '<network_policy>' ]
//	  [ OAUTH_CLIENT_RSA_PUBLIC_KEY = <public_key1> ]
//	  [ OAUTH_CLIENT_RSA_PUBLIC_KEY_2 = <public_key2> ]
//	  [ USE_PRIVATELINK_FOR_AUTHORIZATION_ENDPOINT = { TRUE | FALSE } ]
//	  [ COMMENT = '{string_literal}' ]
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name>
//	  REFRESH { OAUTH_CLIENT_SECRET | OAUTH_CLIENT_SECRET_2 }
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegrationSnowflakeOauth() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// REFRESH { OAUTH_CLIENT_SECRET | OAUTH_CLIENT_SECRET_2 }
				func() bool {
					return v.MatchWord("REFRESH") && v.wordsValue("OAUTH_CLIENT_SECRET", "OAUTH_CLIENT_SECRET_2")()
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSecurityIntegrationSaml2 validates the Snowflake `ALTER SECURITY INTEGRATION (SAML2)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-saml2
//
// Syntax:
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	    [ TYPE = SAML2 ]
//	    [ ENABLED = { TRUE | FALSE } ]
//	    [ METADATA_URL = '<string_literal>' ]
//	    [ SAML2_ISSUER = '<string_literal>' ]
//	    [ SAML2_SSO_URL = '<string_literal>' ]
//	    [ SAML2_PROVIDER = '<string_literal>' ]
//	    [ SAML2_X509_CERT = '<string_literal>' ]
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
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> UNSET {
//	    ENABLED |
//	    [ , ... ]
//	    }
//
//	ALTER [ SECURITY ] INTEGRATION <name> REFRESH
//	  [ SAML2_SNOWFLAKE_PRIVATE_KEY ]
//	  [ METADATA_URL ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegrationSaml2() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// REFRESH [ SAML2_SNOWFLAKE_PRIVATE_KEY ] [ METADATA_URL ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("REFRESH") },
						func() bool { return v.Optional(func() bool { return v.MatchWord("SAML2_SNOWFLAKE_PRIVATE_KEY") }) },
						func() bool { return v.Optional(func() bool { return v.MatchWord("METADATA_URL") }) },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSecurityIntegrationScim validates the Snowflake `ALTER SECURITY INTEGRATION (SCIM)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-security-integration-scim
//
// Syntax:
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name> SET
//	    [ ENABLED = { TRUE | FALSE } ]
//	    [ NETWORK_POLICY = '<network_policy>' ]
//	    [ REJECT_TOKENS_ISSUED_BEFORE = '<datetime_string>' ]
//	    [ SYNC_PASSWORD = { TRUE | FALSE } ]
//	    [ COMMENT = '<string_literal>' ]
//
//	ALTER [ SECURITY ] INTEGRATION [ IF EXISTS ] <name>  UNSET {
//	                                                            NETWORK_POLICY |
//	                                                            [ , ... ]
//	                                                            }
//
//	ALTER [ SECURITY ] INTEGRATION <name> SET TAG <tag_name> = '<tag_value>'
//	    [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ SECURITY ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSecurityIntegrationScim() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("SECURITY") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterSemanticView validates the Snowflake `ALTER SEMANTIC VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-semantic-view
//
// Syntax:
//
//	ALTER SEMANTIC VIEW [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER SEMANTIC VIEW [ IF EXISTS ] <name> SET
//	  COMMENT = '<string_literal>'
//
//	ALTER SEMANTIC VIEW [ IF EXISTS ] <name> UNSET
//	  COMMENT
//
//	ALTER SEMANTIC VIEW <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SEMANTIC VIEW <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSemanticView() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("SEMANTIC", "VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSequence validates the Snowflake `ALTER SEQUENCE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-sequence
//
// Syntax:
//
//	ALTER SEQUENCE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER SEQUENCE [ IF EXISTS ] <name> [ SET ] [ INCREMENT [ BY ] [ = ] <sequence_interval> ]
//
//	ALTER SEQUENCE [ IF EXISTS ] <name> SET
//	  [ { ORDER | NOORDER } ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER SEQUENCE [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterSequence() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SEQUENCE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// RENAME TO <new_name>
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// [ SET ] [ INCREMENT [ BY ] [ = ] <interval> ] / SET <opts> / UNSET COMMENT
				func() bool {
					v.Optional(func() bool { return v.MatchWord("SET") })
					return v.Sequence(
						func() bool {
							return v.Optional(func() bool {
								return v.Sequence(
									func() bool { return v.MatchWord("INCREMENT") },
									func() bool { return v.Optional(func() bool { return v.MatchWord("BY") }) },
									func() bool { return v.Optional(func() bool { return v.MatchOp("=") }) },
									func() bool { return v.Match(sqltok.NumberLit) },
								)
							})
						},
						func() bool { return v.Optional(v.consumeRest) },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterService validates the Snowflake `ALTER SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-service
//
// Syntax:
//
//	ALTER SERVICE [ IF EXISTS ] <name> { SUSPEND | RESUME }
//
//	ALTER SERVICE [ IF EXISTS ] <name>
//	  {
//	     fromSpecification
//	     | fromSpecificationTemplate
//	  }
//
//	ALTER SERVICE [IF EXISTS] <service_name> RESTORE VOLUME <volume_name>
//	                                                 INSTANCES <comma_separated_instance_ids>
//	                                                 FROM SNAPSHOT <snapshot_name>
//
//	ALTER SERVICE [ IF EXISTS ] <name> SET [ MIN_INSTANCES = <num> ]
//	                                       [ MAX_INSTANCES = <num> ]
//	                                       [ LOG_LEVEL = '<log_level>' ]
//	                                       [ AUTO_SUSPEND_SECS = <num> ]
//	                                       [ MIN_READY_INSTANCES = <num> ]
//	                                       [ QUERY_WAREHOUSE = <warehouse_name> ]
//	                                       [ AUTO_RESUME = { TRUE | FALSE } ]
//	                                       [ EXTERNAL_ACCESS_INTEGRATIONS = ( <EAI_name> [ , ... ] ) ]
//	                                       [ COMMENT = '<string_literal>' ]
//
//	ALTER SERVICE [ IF EXISTS ] <name> UNSET { MIN_INSTANCES                |
//	                                           AUTO_SUSPEND_SECS            |
//	                                           MAX_INSTANCES                |
//	                                           LOG_LEVEL                    |
//	                                           MIN_READY_INSTANCES          |
//	                                           QUERY_WAREHOUSE              |
//	                                           AUTO_RESUME                  |
//	                                           EXTERNAL_ACCESS_INTEGRATIONS |
//	                                           COMMENT
//	                                         }
//	                                         [ , ... ]
//
//	ALTER SERVICE [ IF EXISTS ] <name> SET [ TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]]
//
//	Where:
//
//	fromSpecification ::=
//	  {
//	    FROM SPECIFICATION_FILE = '<yaml_file_path>' -- for native app service.
//	    | FROM @<stage> SPECIFICATION_FILE = '<yaml_file_path>' -- for non-native app service.
//	    | FROM SPECIFICATION <specification_text>
//	  }
//
//	fromSpecificationTemplate ::=
//	  {
//	    FROM SPECIFICATION_TEMPLATE_FILE = '<yaml_file_path>' -- for native app service.
//	    | FROM @<stage> SPECIFICATION_TEMPLATE_FILE = '<yaml_file_path>' -- for non-native app service.
//	    | FROM SPECIFICATION_TEMPLATE <specification_text>
//	  }
//	  USING ( <key> => <value> [ , <key> => <value> [ , ... ] ]  )
func (v *Validator) ParseAlterService() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// SUSPEND | RESUME
				v.wordsValue("SUSPEND", "RESUME"),
				// RESTORE VOLUME ... INSTANCES ... FROM SNAPSHOT ...
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("RESTORE") },
						func() bool { return v.MatchWord("VOLUME") },
						func() bool { return v.consumeRest() },
					)
				},
				// FROM SPECIFICATION[_TEMPLATE][_FILE] ... (free-form)
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("FROM") },
						func() bool { return v.consumeRest() },
					)
				},
				// SET <opts> | SET TAG ... | UNSET ...
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSession validates the Snowflake `ALTER SESSION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-session
//
// Syntax:
//
//	ALTER SESSION SET sessionParams
//
//	ALTER SESSION UNSET <param_name> [ , <param_name> , ... ]
//
//	Where:
//
//	sessionParams ::=
//	  ABORT_DETACHED_QUERY = TRUE | FALSE
//	  ACTIVE_PYTHON_PROFILER = 'LINE' | 'MEMORY'
//	  AUTOCOMMIT = TRUE | FALSE
//	  BINARY_INPUT_FORMAT = <string>
//	  BINARY_OUTPUT_FORMAT = <string>
//	  DATE_INPUT_FORMAT = <string>
//	  DATE_OUTPUT_FORMAT = <string>
//	  ERROR_ON_NONDETERMINISTIC_MERGE = TRUE | FALSE
//	  ERROR_ON_NONDETERMINISTIC_UPDATE = TRUE | FALSE
//	  GEOGRAPHY_OUTPUT_FORMAT = 'GeoJSON' | 'WKT' | 'WKB' | 'EWKT' | 'EWKB'
//	  HYBRID_TABLE_LOCK_TIMEOUT = <num>
//	  JSON_INDENT = <num>
//	  LOG_LEVEL = <string>
//	  LOCK_TIMEOUT = <num>
//	  OPT_OUT_ERROR_LOGGING = TRUE | FALSE
//	  PYTHON_PROFILER_TARGET_STAGE = <string>
//	  PYTHON_PROFILER_MODULES = <string>
//	  QUERY_TAG = <string>
//	  ROWS_PER_RESULTSET = <num>
//	  S3_STAGE_VPCE_DNS_NAME = <string>
//	  SEARCH_PATH = <string>
//	  SIMULATED_DATA_SHARING_CONSUMER = <string>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
//	  STRICT_JSON_OUTPUT = TRUE | FALSE
//	  TIMESTAMP_DAY_IS_ALWAYS_24H = TRUE | FALSE
//	  TIMESTAMP_INPUT_FORMAT = <string>
//	  TIMESTAMP_LTZ_OUTPUT_FORMAT = <string>
//	  TIMESTAMP_NTZ_OUTPUT_FORMAT = <string>
//	  TIMESTAMP_OUTPUT_FORMAT = <string>
//	  TIMESTAMP_TYPE_MAPPING = <string>
//	  TIMESTAMP_TZ_OUTPUT_FORMAT = <string>
//	  TIMEZONE = <string>
//	  TIME_INPUT_FORMAT = <string>
//	  TIME_OUTPUT_FORMAT = <string>
//	  TRACE_LEVEL = <string>
//	  TRANSACTION_DEFAULT_ISOLATION_LEVEL = <string>
//	  TWO_DIGIT_CENTURY_START = <num>
//	  UNSUPPORTED_DDL_ACTION = <string>
//	  USE_CACHED_RESULT = TRUE | FALSE
//	  WEEK_OF_YEAR_POLICY = <num>
//	  WEEK_START = <num>
func (v *Validator) ParseAlterSession() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SESSION") },
		func() bool {
			return v.Choice(
				// SET sessionParams ( <param> = <value> [ ... ] )
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("SET") },
						v.option2(v.parseIdentPath, v.parseScalar),
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(
									func() bool { return v.Optional(func() bool { return v.Match(sqltok.Comma) }) },
									v.option2(v.parseIdentPath, v.parseScalar),
								)
							})
						},
					)
				},
				// UNSET <param> [ , <param> ... ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("UNSET") },
						v.parseIdentPath,
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(
									func() bool { return v.Match(sqltok.Comma) },
									v.parseIdentPath,
								)
							})
						},
					)
				},
			)
		},
	)
}

// ParseAlterSessionPolicy validates the Snowflake `ALTER SESSION POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-session-policy
//
// Syntax:
//
//	ALTER SESSION POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER SESSION POLICY [ IF EXISTS ] <name> SET
//	  [ SESSION_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_UI_IDLE_TIMEOUT_MINS = <integer> ]
//	  [ SESSION_MAX_LIFESPAN_MINS = <integer> ]
//	  [ SESSION_UI_MAX_LIFESPAN_MINS = <integer> ]
//	  [ ALLOWED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ BLOCKED_SECONDARY_ROLES = ( [ { 'ALL' | <role_name> [ , <role_name> ... ] } ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER SESSION POLICY [ IF EXISTS ] <name> SET
//	  TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SESSION POLICY [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER SESSION POLICY [ IF EXISTS ] <name> UNSET
//	  [ SESSION_IDLE_TIMEOUT_MINS ]
//	  [ SESSION_UI_IDLE_TIMEOUT_MINS ]
//	  [ SESSION_MAX_LIFESPAN_MINS ]
//	  [ SESSION_UI_MAX_LIFESPAN_MINS ]
//	  [ ALLOWED_SECONDARY_ROLES ]
//	  [ BLOCKED_SECONDARY_ROLES ]
//	  [ COMMENT ]
func (v *Validator) ParseAlterSessionPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("SESSION", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterShare validates the Snowflake `ALTER SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-share
//
// Syntax:
//
//	ALTER SHARE [ IF EXISTS ] <name> { ADD | REMOVE } ACCOUNTS = <consumer_account> [ , <consumer_account> , ... ]
//	                                        [ SHARE_RESTRICTIONS = { TRUE | FALSE } ]
//
//	ALTER SHARE [ IF EXISTS ] <name> SET { [ ACCOUNTS = <consumer_account> [ , <consumer_account> ... ] ]
//	                                       [ COMMENT = '<string_literal>' ] }
//
//	ALTER SHARE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SHARE <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER SHARE [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterShare() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SHARE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// { ADD | REMOVE } ACCOUNTS = ...
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "REMOVE"),
						func() bool { return v.MatchWord("ACCOUNTS") },
						func() bool { return v.consumeRest() },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSnapshot validates the Snowflake `ALTER SNAPSHOT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot
//
// Syntax:
//
//	ALTER SNAPSHOT [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
func (v *Validator) ParseAlterSnapshot() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("SNAPSHOT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// SET COMMENT = '<string>'
		func() bool { return v.MatchWord("SET") },
		v.commentOption(),
	)
}

// ParseAlterSnapshotPolicy validates the Snowflake `ALTER SNAPSHOT POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot-policy
//
// Syntax:
//
//	ALTER SNAPSHOT POLICY <name> RENAME TO <new_name>
//
//	ALTER SNAPSHOT POLICY <name> SET
//	  [ COMMENT = '<string_literal>' ]
//	  [ SCHEDULE = '{ <num> MINUTE | <num> HOUR | USING CRON <expr> <time_zone> }' ]
//	  [ EXPIRE_AFTER_DAYS = <days_integer> ]
//
//	ALTER SNAPSHOT POLICY <name> UNSET { COMMENT | SCHEDULE | EXPIRE_AFTER_DAYS }
//
//	ALTER SNAPSHOT POLICY <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SNAPSHOT POLICY <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSnapshotPolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("SNAPSHOT", "POLICY") },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterSnapshotSet validates the Snowflake `ALTER SNAPSHOT SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-snapshot-set
//
// Syntax:
//
//	ALTER SNAPSHOT SET <name> ADD SNAPSHOT
//
//	ALTER SNAPSHOT SET <name> APPLY SNAPSHOT POLICY <policy_name> [ FORCE ]
//
//	ALTER SNAPSHOT SET <name> SUSPEND SNAPSHOT [ { CREATION | EXPIRATION } ] POLICY
//
//	ALTER SNAPSHOT SET <name> RESUME SNAPSHOT [ { CREATION | EXPIRATION } ] POLICY
//
//	ALTER SNAPSHOT SET <name> DELETE SNAPSHOT IDENTIFIER '<snapshot_id>'
//
//	ALTER SNAPSHOT SET <name> MODIFY SNAPSHOT IDENTIFIER '<snapshot_id>' { ADD | REMOVE } LEGAL HOLD
//
//	ALTER SNAPSHOT SET <name> MODIFY SNAPSHOT IDENTIFIER '<snapshot_id>' SET COMMENT = '<string_literal>'
//
//	ALTER SNAPSHOT SET <name> MODIFY SNAPSHOT IDENTIFIER '<snapshot_id>' UNSET COMMENT
//
//	ALTER SNAPSHOT SET <name> SET COMMENT = '<string_literal>'
//
//	ALTER SNAPSHOT SET <name> UNSET COMMENT
//
//	ALTER SNAPSHOT SET <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER SNAPSHOT SET <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterSnapshotSet() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("SNAPSHOT", "SET") },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// ADD SNAPSHOT
				func() bool { return v.phrase("ADD", "SNAPSHOT") },
				// APPLY SNAPSHOT POLICY <name> [ FORCE ]
				func() bool {
					return v.Sequence(
						func() bool { return v.phrase("APPLY", "SNAPSHOT", "POLICY") },
						v.parseIdentPath,
						func() bool { return v.Optional(func() bool { return v.MatchWord("FORCE") }) },
					)
				},
				// { SUSPEND | RESUME } SNAPSHOT [ { CREATION | EXPIRATION } ] POLICY
				func() bool {
					return v.Sequence(
						v.wordsValue("SUSPEND", "RESUME"),
						func() bool { return v.MatchWord("SNAPSHOT") },
						func() bool { return v.Optional(v.wordsValue("CREATION", "EXPIRATION")) },
						func() bool { return v.MatchWord("POLICY") },
					)
				},
				// DELETE / MODIFY SNAPSHOT IDENTIFIER '<id>' ...
				func() bool {
					return v.Sequence(
						v.wordsValue("DELETE", "MODIFY"),
						func() bool { return v.MatchWord("SNAPSHOT") },
						func() bool { return v.MatchWord("IDENTIFIER") },
						v.parseString,
						func() bool { return v.Optional(v.consumeRest) },
					)
				},
				// SET COMMENT/TAG ... | UNSET COMMENT/TAG ...
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterStage validates the Snowflake `ALTER STAGE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-stage
//
// Syntax:
//
//	ALTER STAGE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER STAGE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER STAGE <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER STAGE [ IF EXISTS ] <name> UNSET DCM PROJECT
//
//	-- Internal stage
//	ALTER STAGE [ IF EXISTS ] <name> SET
//	  [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	  { [ COMMENT = '<string_literal>' ] }
//
//	-- External stage
//	ALTER STAGE [ IF EXISTS ] <name> SET {
//	    [ externalStageParams ]
//	    [ FILE_FORMAT = ( { FORMAT_NAME = '<file_format_name>' | TYPE = { CSV | JSON | AVRO | ORC | PARQUET | XML | CUSTOM } [ formatTypeOptions ] } ) ]
//	    [ COMMENT = '<string_literal>' ]
//	    }
//
//	ALTER STAGE [ IF EXISTS ] <name> SET DIRECTORY = ( { ENABLE = TRUE | FALSE } )
//
//	ALTER STAGE [ IF EXISTS ] <name> REFRESH [ SUBPATH = '<relative-path>' ]
func (v *Validator) ParseAlterStage() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("STAGE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// REFRESH [ SUBPATH = '<path>' ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("REFRESH") },
						func() bool { return v.Optional(v.option("SUBPATH", v.parseString)) },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterStorageIntegration validates the Snowflake `ALTER STORAGE INTEGRATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-storage-integration
//
// Syntax:
//
//	ALTER [ STORAGE ] INTEGRATION [ IF EXISTS ] <name> SET
//	  [ cloudProviderParams ]
//	  [ ENABLED = { TRUE | FALSE } ]
//	  [ STORAGE_ALLOWED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] ) ]
//	  [ STORAGE_BLOCKED_LOCATIONS = ('<cloud>://<bucket>/<path>/' [ , '<cloud>://<bucket>/<path>/' ... ] ) ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER [ STORAGE ] INTEGRATION [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER [ STORAGE ] INTEGRATION <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER [ STORAGE ] INTEGRATION [ IF EXISTS ] <name>  UNSET {
//	                                                          ENABLED                   |
//	                                                          STORAGE_BLOCKED_LOCATIONS |
//	                                                          COMMENT
//	                                                          }
//	                                                          [ , ... ]
//
//	Where:
//
//	cloudProviderParams (for Amazon S3) ::=
//	  STORAGE_AWS_ROLE_ARN = '<iam_role>'
//	  [ STORAGE_AWS_OBJECT_ACL = 'bucket-owner-full-control' ]
//	  [ STORAGE_AWS_EXTERNAL_ID = '<external_id>' ]
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
//
//	cloudProviderParams (for Microsoft Azure) ::=
//	  AZURE_TENANT_ID = '<tenant_id>'
//	  [ USE_PRIVATELINK_ENDPOINT = { TRUE | FALSE } ]
func (v *Validator) ParseAlterStorageIntegration() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("STORAGE") }) },
		func() bool { return v.MatchWord("INTEGRATION") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterStorageLifecyclePolicy validates the Snowflake `ALTER STORAGE LIFECYCLE POLICY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-storage-lifecycle-policy
//
// Syntax:
//
//	ALTER STORAGE LIFECYCLE POLICY [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER STORAGE LIFECYCLE POLICY [ IF EXISTS ] <name> SET
//
//	  BODY -> <expression_on_arg_name>
//	  | ARCHIVE_TIER = { COOL | COLD }
//	  | ARCHIVE_FOR_DAYS = <number_of_days>
//	  | COMMENT = '<string_literal>'
//	  | TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER STORAGE LIFECYCLE POLICY [ IF EXISTS ] <name> UNSET
//	  ARCHIVE_FOR_DAYS
//	  | COMMENT
//	  | TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterStorageLifecyclePolicy() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("STORAGE", "LIFECYCLE", "POLICY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET ( BODY -> <expr> | <opt> = <val> | TAG ... ) / UNSET <opt>
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterStream validates the Snowflake `ALTER STREAM` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-stream
//
// Syntax:
//
//	ALTER STREAM [ IF EXISTS ] <name> SET COMMENT = '<string_literal>'
//
//	ALTER STREAM [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER STREAM <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER STREAM [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterStream() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("STREAM") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// SET COMMENT/TAG ... | UNSET COMMENT/TAG ...
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterStreamlit validates the Snowflake `ALTER STREAMLIT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-streamlit
//
// Syntax:
//
//	ALTER STREAMLIT [ IF EXISTS ] <name> SET
//	  [ MAIN_FILE = '<filename>']
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ RUNTIME_NAME = '<runtime_name>' ]
//	  [ COMPUTE_POOL = <compute_pool_name> ]
//	  [ COMMENT = '<string_literal>']
//	  [ TITLE = '<app_title>' ]
//	  [ IMPORTS = ( '<stage_path_and_directory_or_file_name_to_read>' [ , ... ] ) ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ SECRETS = ( '<snowflake_secret_name>' = <snowflake_secret> [ , ... ] ) ]
//
//	ALTER STREAMLIT [ IF EXISTS ] <name> UNSET { SECRETS                      |
//	                                             EXTERNAL_ACCESS_INTEGRATIONS |
//	                                             QUERY_WAREHOUSE              |
//	                                             TITLE                        |
//	                                             COMMENT
//	                                           }
//	                                           [ , ... ]
//
//	ALTER STREAMLIT [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER STREAMLIT <name> COMMIT
//
//	ALTER STREAMLIT <name> PUSH [ TO <git_branch_uri> ]
//	  [
//	    {
//	      GIT_CREDENTIALS = <snowflake_secret>
//	      | USERNAME = <git_username> PASSWORD = <git_password>
//	    }
//	    NAME = <git_author_name>
//	    EMAIL = <git_author_email>
//	  ]
//	  [ COMMENT = <git_push_comment> ]
//
//	ALTER STREAMLIT <name> ABORT
//
//	ALTER STREAMLIT <name> PULL
//
//	ALTER STREAMLIT <name> ADD LIVE VERSION FROM LAST
func (v *Validator) ParseAlterStreamlit() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("STREAMLIT") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.MatchWord("COMMIT") },
				func() bool { return v.MatchWord("ABORT") },
				func() bool { return v.MatchWord("PULL") },
				// ADD LIVE VERSION FROM LAST
				func() bool { return v.phrase("ADD", "LIVE", "VERSION", "FROM", "LAST") },
				// PUSH [ TO <uri> ] [ ... ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("PUSH") },
						func() bool { return v.Optional(v.consumeRest) },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterTable validates the Snowflake `ALTER TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table
//
// Syntax:
//
//	ALTER TABLE [ IF EXISTS ] <name> RENAME TO <new_table_name>
//
//	ALTER TABLE [ IF EXISTS ] <name> SWAP WITH <target_table_name>
//
//	ALTER TABLE [ IF EXISTS ] <name> { clusteringAction | tableColumnAction | constraintAction  }
//
//	ALTER TABLE [ IF EXISTS ] <name> dataMetricFunctionAction
//
//	ALTER TABLE [ IF EXISTS ] <name> dataGovnPolicyTagAction
//
//	ALTER TABLE [ IF EXISTS ] <name> extTableColumnAction
//
//	ALTER TABLE [ IF EXISTS ] <name> searchOptimizationAction
//
//	ALTER TABLE [ IF EXISTS ] <name> ADD STORAGE LIFECYCLE POLICY <policy_name>
//	   ON ( <col_name> [ , <col_name> ... ] )
//
//	ALTER TABLE [ IF EXISTS ] <name> DROP STORAGE LIFECYCLE POLICY
//
//	ALTER TABLE [ IF EXISTS ] <name> SET
//	   [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	   [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	   [ CHANGE_TRACKING = { TRUE | FALSE  } ]
//	   [ DEFAULT_DDL_COLLATION = '<collation_specification>' ]
//	   [ ENABLE_SCHEMA_EVOLUTION = { TRUE | FALSE } ]
//	   [ ERROR_LOGGING = { TRUE | FALSE } ]
//	   [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	   [ COMMENT = '<string_literal>' ]
//	   [ ROW_TIMESTAMP = { TRUE | FALSE } ]
//
//	ALTER TABLE [ IF EXISTS ] <name> UNSET {
//	                                        DATA_RETENTION_TIME_IN_DAYS         |
//	                                        MAX_DATA_EXTENSION_TIME_IN_DAYS     |
//	                                        CHANGE_TRACKING                     |
//	                                        DEFAULT_DDL_COLLATION               |
//	                                        ENABLE_SCHEMA_EVOLUTION             |
//	                                        ERROR_LOGGING                       |
//	                                        CONTACT <purpose>                   |
//	                                        COMMENT                             |
//	                                        ROW_TIMESTAMP                       |
//	                                        DCM PROJECT
//	                                        }
//	                                        [ , ... ]
func (v *Validator) ParseAlterTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("SWAP", "WITH") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
				// clusteringAction / tableColumnAction / constraintAction /
				// dataMetricFunctionAction / dataGovnPolicyTagAction /
				// searchOptimizationAction / STORAGE LIFECYCLE POLICY — require a
				// documented action verb (so a garbage action is flagged), then accept
				// the free-form remainder.
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "DROP", "ALTER", "MODIFY", "RENAME", "CLUSTER", "RECLUSTER", "SUSPEND", "RESUME"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterTableAlterColumn validates the Snowflake `ALTER TABLE ALTER COLUMN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table-column
//
// Syntax:
//
//	ALTER TABLE <name> { ALTER | MODIFY } [ ( ]
//	                                              [ COLUMN ] <col1_name> DROP DEFAULT
//	                                            , [ COLUMN ] <col1_name> SET DEFAULT <seq_name>.NEXTVAL
//	                                            , [ COLUMN ] <col1_name> { [ SET ] NOT NULL | DROP NOT NULL }
//	                                            , [ COLUMN ] <col1_name> [ [ SET DATA ] TYPE ] <type>
//	                                            , [ COLUMN ] <col1_name> COMMENT '<string>'
//	                                            , [ COLUMN ] <col1_name> UNSET COMMENT
//	                                          [ , [ COLUMN ] <col2_name> ... ]
//	                                          [ , ... ]
//	                                      [ ) ]
//
//	ALTER TABLE <name> { ALTER | MODIFY } [ COLUMN ] dataGovnPolicyTagAction
func (v *Validator) ParseAlterTableAlterColumn() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// { ALTER | MODIFY } <column actions> — free-form remainder.
		v.wordsValue("ALTER", "MODIFY"),
		func() bool {
			if v.AtEnd() {
				return false
			}
			return v.consumeRest()
		},
	)
}

// ParseAlterTableEventTables validates the Snowflake `ALTER TABLE (event tables)` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-table-event-table
//
// Syntax:
//
//	ALTER TABLE [ IF EXISTS ] <name> RENAME TO <new_table_name>
//
//	ALTER TABLE [ IF EXISTS ] <name> clusteringAction
//
//	ALTER TABLE [ IF EXISTS ] <name> dataGovnPolicyTagAction
//
//	ALTER TABLE [ IF EXISTS ] <name> searchOptimizationAction
//
//	ALTER TABLE [ IF EXISTS ] <name> SET
//	  [ DATA_RETENTION_TIME_IN_DAYS = <integer> ]
//	  [ MAX_DATA_EXTENSION_TIME_IN_DAYS = <integer> ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE  } ]
//	  [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER TABLE [ IF EXISTS ] <name> UNSET {
//	                                       DATA_RETENTION_TIME_IN_DAYS         |
//	                                       MAX_DATA_EXTENSION_TIME_IN_DAYS     |
//	                                       CHANGE_TRACKING                     |
//	                                       CONTACT <purpose>                   |
//	                                       COMMENT                             |
//	                                       }
//
//	Where:
//
//	clusteringAction ::=
//	  {
//	     CLUSTER BY ( <expr> [ , <expr> , ... ] )
//	   | { SUSPEND | RESUME } RECLUSTER
//	   | DROP CLUSTERING KEY
//	  }
func (v *Validator) ParseAlterTableEventTables() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TABLE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
				// clusteringAction / dataGovnPolicyTagAction / searchOptimizationAction —
				// require a documented action verb before the free-form remainder.
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "DROP", "CLUSTER", "SUSPEND", "RESUME"),
						v.consumeRest,
					)
				},
			)
		},
	)
}

// ParseAlterTag validates the Snowflake `ALTER TAG` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-tag
//
// Syntax:
//
//	ALTER TAG [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER TAG [ IF EXISTS ] <name> { ADD | DROP } ALLOWED_VALUES '<val_1>' [ , '<val_2>' [ , ... ] ]
//
//	ALTER TAG [ IF EXISTS ] <name> SET
//	  [ ALLOWED_VALUES '<val_1>' [ , '<val_2>' [ , ... ] ] ]
//	  [ PROPAGATE = { ON_DEPENDENCY_AND_DATA_MOVEMENT | ON_DEPENDENCY | ON_DATA_MOVEMENT }
//	    [ ON_CONFLICT = { '<string>' | ALLOWED_VALUES_SEQUENCE } ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER TAG [ IF EXISTS ] <name> UNSET { ALLOWED_VALUES | PROPAGATE | ON_CONFLICT | COMMENT }
//
//	ALTER TAG [ IF EXISTS ] <name> SET MASKING POLICY
//	  <masking_policy_name> [ , MASKING POLICY <masking_policy_2_name> , ... ] [ FORCE ]
//
//	ALTER TAG [ IF EXISTS ] <name> UNSET MASKING POLICY <masking_policy_name> [ , MASKING POLICY <masking_policy_2_name> , ... ]
//
//	ALTER TAG [ IF EXISTS ] <name> UNSET DCM PROJECT
func (v *Validator) ParseAlterTag() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TAG") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// { ADD | DROP } ALLOWED_VALUES '<v1>' [ , '<v2>' ... ]
				func() bool {
					return v.Sequence(
						v.wordsValue("ADD", "DROP"),
						func() bool { return v.MatchWord("ALLOWED_VALUES") },
						v.parseString,
						func() bool {
							return v.ZeroOrMore(func() bool {
								return v.Sequence(func() bool { return v.Match(sqltok.Comma) }, v.parseString)
							})
						},
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterTask validates the Snowflake `ALTER TASK` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-task
//
// Syntax:
//
//	ALTER TASK [ IF EXISTS ] <name> RESUME | SUSPEND
//
//	ALTER TASK [ IF EXISTS ] <name> REMOVE AFTER <string> [ , <string> , ... ]
//	  | ADD AFTER <string> [ , <string> , ... ]
//
//	ALTER TASK [ IF EXISTS ] <name> SET
//	  [ { WAREHOUSE = <string> }
//	    | { USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = <string> } ]
//	  [ SCHEDULE = { '<num> { HOURS | MINUTES | SECONDS }'
//	               | 'USING CRON <expr> <time_zone>' } ]
//	  [ CONFIG = <configuration_string> ]
//	  [ OVERLAP_POLICY = { NO_OVERLAP | ALLOW_CHILD_OVERLAP | ALLOW_ALL_OVERLAP } ]
//	  [ USER_TASK_TIMEOUT_MS = <num> ]
//	  [ SUSPEND_TASK_AFTER_NUM_FAILURES = <num> ]
//	  [ ERROR_INTEGRATION = <integration_name> ]
//	  [ SUCCESS_INTEGRATION = <integration_name> ]
//	  [ LOG_LEVEL = '<log_level>' ]
//	  [ COMMENT = <string> ]
//	  [ <session_parameter> = <value>
//	    [ , <session_parameter> = <value> ... ] ]
//	  [ TASK_AUTO_RETRY_ATTEMPTS = <num> ]
//	  [ USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = <num> ]
//	  [ TARGET_COMPLETION_INTERVAL = '<num> { HOURS | MINUTES | SECONDS }' ]
//	  [ SERVERLESS_TASK_MIN_STATEMENT_SIZE= 'XSMALL | SMALL
//	    | MEDIUM | LARGE | XLARGE | XXLARGE' ]
//	  [ SERVERLESS_TASK_MAX_STATEMENT_SIZE= 'XSMALL | SMALL
//	    | MEDIUM | LARGE | XLARGE | XXLARGE' ]
//	  [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	  [ EXECUTE AS USER <user_name> ]
//
//	ALTER TASK [ IF EXISTS ] <name> UNSET
//	  [ WAREHOUSE ]
//	  [ SCHEDULE ]
//	  [ CONFIG ]
//	  [ OVERLAP_POLICY ]
//	  [ USER_TASK_TIMEOUT_MS ]
//	  [ SUSPEND_TASK_AFTER_NUM_FAILURES ]
//	  [ LOG_LEVEL ]
//	  [ COMMENT ]
//	  [ <session_parameter> [ , <session_parameter> ... ] ]
//	  [ TARGET_COMPLETION_INTERVAL ]
//	  [ SERVERLESS_TASK_MIN_STATEMENT_SIZE ]
//	  [ SERVERLESS_TASK_MAX_STATEMENT_SIZE ]
//	  [ CONTACT <purpose> [ , ... ]]
//	  [ EXECUTE AS USER ]
//	  [ DCM PROJECT ]
//
//	ALTER TASK [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>'
//	  [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER TASK [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER TASK [ IF EXISTS ] <name> SET FINALIZE = <string>
//
//	ALTER TASK [ IF EXISTS ] <name> UNSET FINALIZE
//
//	ALTER TASK [ IF EXISTS ] <name> MODIFY AS <sql>
//
//	ALTER TASK [ IF EXISTS ] <name> MODIFY WHEN <boolean_expr>
//
//	ALTER TASK [ IF EXISTS ] <name> REMOVE WHEN
func (v *Validator) ParseAlterTask() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TASK") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("RESUME", "SUSPEND"),
				// { REMOVE | ADD } AFTER <str> [ , <str> ... ]
				func() bool {
					return v.Sequence(
						v.wordsValue("REMOVE", "ADD"),
						func() bool { return v.MatchWord("AFTER") },
						func() bool { return v.consumeRest() },
					)
				},
				// REMOVE WHEN
				func() bool { return v.phrase("REMOVE", "WHEN") },
				// MODIFY { AS <sql> | WHEN <expr> }
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("MODIFY") },
						v.wordsValue("AS", "WHEN"),
						func() bool { return v.consumeRest() },
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterType validates the Snowflake `ALTER TYPE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-type
//
// Syntax:
//
//	ALTER TYPE [ IF EXISTS ] <name> SET
//	  COMMENT = '<string_literal>'
//
//	ALTER TYPE [ IF EXISTS ] <name> UNSET COMMENT
func (v *Validator) ParseAlterType() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("TYPE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				// SET COMMENT = '<string>'
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("SET") }, v.commentOption())
				},
				// UNSET COMMENT
				func() bool { return v.phrase("UNSET", "COMMENT") },
			)
		},
	)
}

// ParseAlterUser validates the Snowflake `ALTER USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user
//
// Syntax:
//
//	ALTER USER [ IF EXISTS ] [ <name> ] RENAME TO <new_name>
//
//	ALTER USER [ IF EXISTS ] [ <name> ] RESET PASSWORD
//
//	ALTER USER [ IF EXISTS ] [ <name> ] ABORT ALL QUERIES
//
//	ALTER USER [ IF EXISTS ] [ <name> ] ADD DELEGATED AUTHORIZATION OF ROLE <role_name> TO SECURITY INTEGRATION <integration_name>
//
//	ALTER USER [ IF EXISTS ] [ <name> ] REMOVE DELEGATED { AUTHORIZATION OF ROLE <role_name> | AUTHORIZATIONS } FROM SECURITY INTEGRATION <integration_name>
//
//	ALTER USER [ IF EXISTS ] [ <name> ] mfaActions
//
//	ALTER USER [ IF EXISTS ] [ <name> ] SET { AUTHENTICATION | PASSWORD | SESSION } POLICY <policy_name> [ FORCE ]
//
//	ALTER USER [ IF EXISTS ] [ <name> ] UNSET { AUTHENTICATION | PASSWORD | SESSION } POLICY
//
//	ALTER USER [ IF EXISTS ] [ <name> ] SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER USER [ IF EXISTS ] [ <name> ] UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER USER [ IF EXISTS ] [ <name> ] SET { [ objectProperties ] [ objectParams ] [ sessionParams ] }
//
//	ALTER USER [ IF EXISTS ] [ <name> ] UNSET { <object_property_name> | <object_param_name> | <session_param_name> } [ , ... ]
func (v *Validator) ParseAlterUser() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.phrase("RESET", "PASSWORD") },
				func() bool { return v.phrase("ABORT", "ALL", "QUERIES") },
				// [ <name> ] <action> — name then a required action.
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool {
							return v.Choice(
								func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
								func() bool { return v.phrase("RESET", "PASSWORD") },
								func() bool { return v.phrase("ABORT", "ALL", "QUERIES") },
								func() bool {
									if v.AtEnd() {
										return false
									}
									v.advance()
									return v.consumeRest()
								},
							)
						},
					)
				},
				// nameless mfa / delegated actions
				func() bool {
					if v.AtEnd() {
						return false
					}
					v.advance()
					return v.consumeRest()
				},
			)
		},
	)
}

// ParseAlterUserAddProgrammaticAccessToken validates the Snowflake `ALTER USER ADD PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-add-programmatic-access-token
//
// Syntax:
//
//	ALTER USER [ IF EXISTS ] [ <username> ] ADD { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name>
//	  [ ROLE_RESTRICTION = '<string_literal>' ]
//	  [ DAYS_TO_EXPIRY = <integer> ]
//	  [ MINS_TO_BYPASS_NETWORK_POLICY_REQUIREMENT = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
func (v *Validator) ParseAlterUserAddProgrammaticAccessToken() bool {
	patToken := func() bool {
		return v.Sequence(
			func() bool {
				return v.Choice(
					func() bool { return v.phrase("PROGRAMMATIC", "ACCESS", "TOKEN") },
					func() bool { return v.MatchWord("PAT") },
				)
			},
			v.parseIdentPath, // <token_name>
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				// ADD PAT ... directly (no username)
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("ADD") }, patToken)
				},
				// <username> ADD PAT ...
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchWord("ADD") },
						patToken,
					)
				},
			)
		},
		// [ option list ]
		func() bool { return v.Optional(v.consumeRest) },
	)
}

// ParseAlterUserModifyProgrammaticAccessToken validates the Snowflake `ALTER USER MODIFY PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-modify-programmatic-access-token
//
// Syntax:
//
//	ALTER USER [ IF EXISTS ] [ <username> ] MODIFY { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name>
//	  RENAME TO <new_token_name>
//
//	ALTER USER [ IF EXISTS ] [ <username> ] MODIFY { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name> SET
//	  [ DISABLED = { TRUE | FALSE } ]
//	  [ MINS_TO_BYPASS_NETWORK_POLICY_REQUIREMENT = <integer> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER USER [ IF EXISTS ] [ <username> ] MODIFY { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name> UNSET
//	  [ DISABLED ]
//	  [ MINS_TO_BYPASS_NETWORK_POLICY_REQUIREMENT ]
//	  [ COMMENT ]
func (v *Validator) ParseAlterUserModifyProgrammaticAccessToken() bool {
	patToken := func() bool {
		return v.Sequence(
			func() bool {
				return v.Choice(
					func() bool { return v.phrase("PROGRAMMATIC", "ACCESS", "TOKEN") },
					func() bool { return v.MatchWord("PAT") },
				)
			},
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("MODIFY") }, patToken)
				},
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchWord("MODIFY") },
						patToken,
					)
				},
			)
		},
		// RENAME TO <new_name> | SET <opts> | UNSET <opts>
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterUserRemoveProgrammaticAccessToken validates the Snowflake `ALTER USER REMOVE PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-remove-programmatic-access-token
//
// Syntax:
//
//	ALTER USER [ IF EXISTS ] [ <username> ] REMOVE { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name>
func (v *Validator) ParseAlterUserRemoveProgrammaticAccessToken() bool {
	patToken := func() bool {
		return v.Sequence(
			func() bool {
				return v.Choice(
					func() bool { return v.phrase("PROGRAMMATIC", "ACCESS", "TOKEN") },
					func() bool { return v.MatchWord("PAT") },
				)
			},
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("REMOVE") }, patToken)
				},
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchWord("REMOVE") },
						patToken,
					)
				},
			)
		},
	)
}

// ParseAlterUserRotateProgrammaticAccessToken validates the Snowflake `ALTER USER ROTATE PROGRAMMATIC ACCESS TOKEN` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-user-rotate-programmatic-access-token
//
// Syntax:
//
//	ALTER USER [ IF EXISTS ] [ <username> ] ROTATE { PROGRAMMATIC ACCESS TOKEN | PAT } <token_name>
//	  [ EXPIRE_ROTATED_TOKEN_AFTER_HOURS = <integer> ]
func (v *Validator) ParseAlterUserRotateProgrammaticAccessToken() bool {
	patToken := func() bool {
		return v.Sequence(
			func() bool {
				return v.Choice(
					func() bool { return v.phrase("PROGRAMMATIC", "ACCESS", "TOKEN") },
					func() bool { return v.MatchWord("PAT") },
				)
			},
			v.parseIdentPath,
		)
	}
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("USER") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				func() bool {
					return v.Sequence(func() bool { return v.MatchWord("ROTATE") }, patToken)
				},
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool { return v.MatchWord("ROTATE") },
						patToken,
					)
				},
			)
		},
		// [ EXPIRE_ROTATED_TOKEN_AFTER_HOURS = <integer> ]
		func() bool {
			return v.Optional(v.option("EXPIRE_ROTATED_TOKEN_AFTER_HOURS",
				func() bool { return v.Match(sqltok.NumberLit) }))
		},
	)
}

// ParseAlterView validates the Snowflake `ALTER VIEW` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-view
//
// Syntax:
//
//	ALTER VIEW [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER VIEW [ IF EXISTS ] <name> SET
//	  [ SECURE ]
//	  [ CHANGE_TRACKING = { TRUE | FALSE } ]
//	  [ CONTACT <purpose> = <contact_name> [ , <purpose> = <contact_name> ... ] ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER VIEW [ IF EXISTS ] <name> UNSET
//	  [ SECURE ]
//	  [ CONTACT <purpose> ]
//	  [ COMMENT = '<string_literal>' ]
//	  [ DCM PROJECT ]
//
//	ALTER VIEW <name> dataMetricFunctionAction
//
//	ALTER VIEW [ IF EXISTS ] <name> dataGovnPolicyTagAction
func (v *Validator) ParseAlterView() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("VIEW") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
				// dataMetricFunctionAction / dataGovnPolicyTagAction (ADD/DROP/MODIFY ...)
				func() bool {
					if v.AtEnd() {
						return false
					}
					v.advance()
					return v.consumeRest()
				},
			)
		},
	)
}

// ParseAlterWarehouse validates the Snowflake `ALTER WAREHOUSE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-warehouse
//
// Syntax:
//
//	ALTER WAREHOUSE [ IF EXISTS ] [ <name> ] { SUSPEND | RESUME [ IF SUSPENDED ] }
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> { ENABLE | DISABLE }
//
//	ALTER WAREHOUSE [ IF EXISTS ] [ <name> ] ABORT ALL QUERIES
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> RENAME TO <new_name>
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> SET [ objectProperties ]
//	                                         [ objectParams ]
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> SET TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> UNSET TAG <tag_name> [ , <tag_name> ... ]
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> UNSET { <property_name> | <param_name> } [ , ... ]
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> UNSET DCM PROJECT
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> ADD TABLES ( <table_name> [ , <table_name> ... ] )
//
//	ALTER WAREHOUSE [ IF EXISTS ] <name> DROP TABLES ( <table_name> [ , <table_name> ... ] )
//
//	Where:
//
//	objectProperties ::=
//	  WAREHOUSE_TYPE = { STANDARD | 'SNOWPARK-OPTIMIZED' | ADAPTIVE }
//	  WAREHOUSE_SIZE = { XSMALL | SMALL | MEDIUM | LARGE | XLARGE | XXLARGE | XXXLARGE | X4LARGE | X5LARGE | X6LARGE }
//	  GENERATION = { '1' | '2' }
//	  RESOURCE_CONSTRAINT = { STANDARD_GEN_1 | STANDARD_GEN_2 | MEMORY_1X | MEMORY_1X_x86 | MEMORY_16X | MEMORY_16X_x86 | MEMORY_64X | MEMORY_64X_x86 }
//	  WAIT_FOR_COMPLETION = { TRUE | FALSE }
//	  MAX_CLUSTER_COUNT = <num>
//	  MIN_CLUSTER_COUNT = <num>
//	  SCALING_POLICY = { STANDARD | ECONOMY }
//	  AUTO_SUSPEND = { <num> | NULL }
//	  AUTO_RESUME = { TRUE | FALSE }
//	  RESOURCE_MONITOR = <monitor_name>
//	  COMMENT = '<string_literal>'
//	  ENABLE_QUERY_ACCELERATION = { TRUE | FALSE }
//	  QUERY_ACCELERATION_MAX_SCALE_FACTOR = <num>
//
//	objectParams ::=
//	  MAX_CONCURRENCY_LEVEL = <num>
//	  STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = <num>
//	  STATEMENT_TIMEOUT_IN_SECONDS = <num>
func (v *Validator) ParseAlterWarehouse() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("WAREHOUSE") },
		func() bool { return v.ifExists() },
		func() bool {
			return v.Choice(
				// Nameless forms: { SUSPEND | RESUME [ IF SUSPENDED ] } | ABORT ALL QUERIES
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("RESUME") },
						func() bool { return v.Optional(func() bool { return v.phrase("IF", "SUSPENDED") }) },
					)
				},
				func() bool { return v.MatchWord("SUSPEND") },
				func() bool { return v.phrase("ABORT", "ALL", "QUERIES") },
				// <name> <action>
				func() bool {
					return v.Sequence(
						v.parseIdentPath,
						func() bool {
							return v.Choice(
								func() bool {
									return v.Sequence(
										func() bool { return v.MatchWord("RESUME") },
										func() bool { return v.Optional(func() bool { return v.phrase("IF", "SUSPENDED") }) },
									)
								},
								v.wordsValue("SUSPEND", "ENABLE", "DISABLE"),
								func() bool { return v.phrase("ABORT", "ALL", "QUERIES") },
								func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
								// { ADD | DROP } TABLES ( ... )
								func() bool {
									return v.Sequence(
										v.wordsValue("ADD", "DROP"),
										func() bool { return v.MatchWord("TABLES") },
										v.consumeBalancedParens,
									)
								},
								func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
							)
						},
					)
				},
			)
		},
	)
}

// ParseAlterApplicationService validates the Snowflake `ALTER APPLICATION SERVICE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-application-service
//
// Syntax:
//
//	ALTER APPLICATION SERVICE [ IF EXISTS ] <name> { SUSPEND | RESUME }
//
//	ALTER APPLICATION SERVICE [ IF EXISTS ] <name>
//	  UPGRADE [ TO VERSION <version_alias> ]
//
//	ALTER APPLICATION SERVICE [ IF EXISTS ] <name> SET
//	  [ AUTO_RESUME = { TRUE | FALSE } ]
//	  [ AUTO_SUSPEND_SECS = <num> ]
//	  [ EXTERNAL_ACCESS_INTEGRATIONS = ( <integration_name> [ , ... ] ) ]
//	  [ QUERY_WAREHOUSE = <warehouse_name> ]
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER APPLICATION SERVICE [ IF EXISTS ] <name> UNSET
//	  {
//	    AUTO_RESUME                  |
//	    AUTO_SUSPEND_SECS            |
//	    EXTERNAL_ACCESS_INTEGRATIONS |
//	    QUERY_WAREHOUSE              |
//	    COMMENT
//	  }
//	  [ , ... ]
func (v *Validator) ParseAlterApplicationService() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("APPLICATION", "SERVICE") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		func() bool {
			return v.Choice(
				v.wordsValue("SUSPEND", "RESUME"),
				// UPGRADE [ TO VERSION <version_alias> ]
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("UPGRADE") },
						func() bool {
							return v.Optional(func() bool {
								return v.Sequence(func() bool { return v.phrase("TO", "VERSION") }, v.parseIdentPath)
							})
						},
					)
				},
				func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
			)
		},
	)
}

// ParseAlterArtifactRepository validates the Snowflake `ALTER ARTIFACT REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-artifact-repository
//
// Syntax:
//
//	ALTER ARTIFACT REPOSITORY [ IF EXISTS ] <name> SET
//	  [ COMMENT = '<string_literal>' ]
//
//	ALTER ARTIFACT REPOSITORY [ IF EXISTS ] <name> UNSET
//	  { COMMENT }
//
//	ALTER ARTIFACT REPOSITORY [ IF EXISTS ] <name> SET
//	  TAG <tag_name> = '<tag_value>' [ , <tag_name> = '<tag_value>' ... ]
//
//	ALTER ARTIFACT REPOSITORY [ IF EXISTS ] <name> UNSET
//	  TAG <tag_name> [ , <tag_name> ... ]
func (v *Validator) ParseAlterArtifactRepository() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("ARTIFACT", "REPOSITORY") },
		func() bool { return v.ifExists() },
		v.parseIdentPath,
		// SET ( COMMENT | TAG ... ) | UNSET ( COMMENT | TAG ... )
		func() bool { return v.Sequence(v.wordsValue("SET", "UNSET"), v.consumeRest) },
	)
}

// ParseAlterEventRoutingTable validates the Snowflake `ALTER EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-event-routing-table
//
// Syntax:
//
//	ALTER EVENT ROUTING TABLE <table_name> [ FORCE ]
//	  SET RULES
//	  <rule_name> = (REGION_GROUP=<region_group>, REGIONS=('<region1>', '<region2>', ...), DESTINATION_ACCOUNT = <organization>.<account_name>),
//	  ...
//
//	ALTER EVENT ROUTING TABLE <table_name> [ FORCE ]
//	  SET RULE
//	  <rule_name> REGION_GROUP=<region_group> REGIONS=('<region1>', '<region2>', ...) DESTINATION_ACCOUNT = <organization>.<account_name>
//
//	ALTER EVENT ROUTING TABLE <table_name> [ FORCE ]
//	  UNSET RULE <rule_name>
//
//	ALTER EVENT ROUTING TABLE <table_name>
//	  MODIFY RULE <rule_name> RENAME TO <new_rule_name>
//
//	ALTER EVENT ROUTING TABLE <table_name>
//	  RENAME TO <new_table_name>
func (v *Validator) ParseAlterEventRoutingTable() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.phrase("EVENT", "ROUTING", "TABLE") },
		v.parseIdentPath,
		func() bool { return v.Optional(func() bool { return v.MatchWord("FORCE") }) },
		func() bool {
			return v.Choice(
				// RENAME TO <new_table_name>
				func() bool { return v.phrase("RENAME", "TO") && v.parseIdentPath() },
				// SET { RULES | RULE } ... | UNSET RULE <name>
				func() bool {
					return v.Sequence(
						v.wordsValue("SET", "UNSET"),
						v.wordsValue("RULES", "RULE"),
						func() bool { return v.consumeRest() },
					)
				},
				// MODIFY RULE <name> RENAME TO <new_rule_name>
				func() bool {
					return v.Sequence(
						func() bool { return v.MatchWord("MODIFY") },
						func() bool { return v.MatchWord("RULE") },
						v.parseIdentPath,
						func() bool { return v.phrase("RENAME", "TO") },
						v.parseIdentPath,
					)
				},
			)
		},
	)
}

// ParseAlterOrganizationSetEventRoutingTable validates the Snowflake `ALTER ORGANIZATION SET EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-set-event-routing-table
//
// Syntax:
//
//	ALTER ORGANIZATION SET EVENT ROUTING TABLE <table_name> FOR ALL APPLICATION LISTINGS
func (v *Validator) ParseAlterOrganizationSetEventRoutingTable() bool {
	// ALTER ORGANIZATION SET EVENT ROUTING TABLE <name> FOR ALL APPLICATION LISTINGS
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("SET") },
		func() bool { return v.phrase("EVENT", "ROUTING", "TABLE") },
		v.parseIdentPath,
		func() bool { return v.phrase("FOR", "ALL", "APPLICATION", "LISTINGS") },
	)
}

// ParseAlterOrganizationUnsetEventRoutingTable validates the Snowflake `ALTER ORGANIZATION UNSET EVENT ROUTING TABLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/alter-organization-unset-event-routing-table
//
// Syntax:
//
//	ALTER ORGANIZATION UNSET EVENT ROUTING TABLE FOR ALL APPLICATION LISTINGS
func (v *Validator) ParseAlterOrganizationUnsetEventRoutingTable() bool {
	// ALTER ORGANIZATION UNSET EVENT ROUTING TABLE FOR ALL APPLICATION LISTINGS
	return v.Sequence(
		func() bool { return v.MatchWord("ALTER") },
		func() bool { return v.MatchWord("ORGANIZATION") },
		func() bool { return v.MatchWord("UNSET") },
		func() bool { return v.phrase("EVENT", "ROUTING", "TABLE") },
		func() bool { return v.phrase("FOR", "ALL", "APPLICATION", "LISTINGS") },
	)
}
