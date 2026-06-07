// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package erdesigner

import (
	"strings"
	"testing"
)

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertNotContains(t *testing.T, sql, substr string) {
	t.Helper()
	if strings.Contains(sql, substr) {
		t.Errorf("expected SQL NOT to contain %q\nSQL:\n%s", substr, sql)
	}
}

func makeState(overrides ...func(*JoinQueryState)) JoinQueryState {
	s := JoinQueryState{
		Database:  "MY_DB",
		BaseTable: TableRef{Schema: "S", Name: "ORDERS"},
		Joins: []JoinEntry{
			{
				Table:       TableRef{Schema: "S", Name: "USERS"},
				JoinType:    "INNER",
				OnCondition: "S.ORDERS.USER_ID = S.USERS.ID",
				FKPairs: []FKPair{{
					From: FKColRef{Schema: "S", Table: "ORDERS", Col: "USER_ID"},
					To:   FKColRef{Schema: "S", Table: "USERS", Col: "ID"},
				}},
				IsIntermediate: false,
			},
		},
		SelectedColumns: map[string][]string{},
	}
	for _, fn := range overrides {
		fn(&s)
	}
	return s
}

func TestBuildJoinSQL(t *testing.T) {
	t.Run("basic INNER JOIN with quoted identifiers", func(t *testing.T) {
		sql := BuildJoinSQL(makeState())
		assertContains(t, sql, "SELECT")
		assertContains(t, sql, "t1.*")
		assertContains(t, sql, "t2.*")
		assertContains(t, sql, `FROM "MY_DB"."S"."ORDERS" t1`)
		assertContains(t, sql, `INNER JOIN "MY_DB"."S"."USERS" t2 ON t1."USER_ID" = t2."ID"`)
	})

	t.Run("LEFT join type", func(t *testing.T) {
		sql := BuildJoinSQL(makeState(func(s *JoinQueryState) {
			s.Joins[0].JoinType = "LEFT"
		}))
		assertContains(t, sql, `LEFT JOIN "MY_DB"."S"."USERS" t2`)
	})

	t.Run("RIGHT join type", func(t *testing.T) {
		sql := BuildJoinSQL(makeState(func(s *JoinQueryState) {
			s.Joins[0].JoinType = "RIGHT"
		}))
		assertContains(t, sql, `RIGHT JOIN "MY_DB"."S"."USERS" t2`)
	})

	t.Run("FULL OUTER join type", func(t *testing.T) {
		sql := BuildJoinSQL(makeState(func(s *JoinQueryState) {
			s.Joins[0].JoinType = "FULL OUTER"
		}))
		assertContains(t, sql, `FULL OUTER JOIN "MY_DB"."S"."USERS" t2`)
	})

	t.Run("selected columns", func(t *testing.T) {
		sql := BuildJoinSQL(makeState(func(s *JoinQueryState) {
			s.SelectedColumns = map[string][]string{
				"S.ORDERS": {"ID", "TOTAL"},
				"S.USERS":  {"NAME", "EMAIL"},
			}
		}))
		assertContains(t, sql, `t1."ID"`)
		assertContains(t, sql, `t1."TOTAL"`)
		assertContains(t, sql, `t2."NAME"`)
		assertContains(t, sql, `t2."EMAIL"`)
		assertNotContains(t, sql, "t1.*")
		assertNotContains(t, sql, "t2.*")
	})

	t.Run("composite FK with AND", func(t *testing.T) {
		sql := BuildJoinSQL(makeState(func(s *JoinQueryState) {
			s.Joins = []JoinEntry{{
				Table:       TableRef{Schema: "S", Name: "DETAILS"},
				JoinType:    "INNER",
				OnCondition: "S.ORDERS.ID = S.DETAILS.ORDER_ID AND S.ORDERS.REGION = S.DETAILS.REGION",
				FKPairs: []FKPair{
					{
						From: FKColRef{Schema: "S", Table: "ORDERS", Col: "ID"},
						To:   FKColRef{Schema: "S", Table: "DETAILS", Col: "ORDER_ID"},
					},
					{
						From: FKColRef{Schema: "S", Table: "ORDERS", Col: "REGION"},
						To:   FKColRef{Schema: "S", Table: "DETAILS", Col: "REGION"},
					},
				},
			}}
		}))
		assertContains(t, sql, `ON t1."ID" = t2."ORDER_ID" AND t1."REGION" = t2."REGION"`)
	})

	t.Run("multiple joins with correct aliases", func(t *testing.T) {
		sql := BuildJoinSQL(JoinQueryState{
			Database:  "MY_DB",
			BaseTable: TableRef{Schema: "S", Name: "ORDER_ITEMS"},
			Joins: []JoinEntry{
				{
					Table:       TableRef{Schema: "S", Name: "ORDERS"},
					JoinType:    "INNER",
					OnCondition: "S.ORDER_ITEMS.ORDER_ID = S.ORDERS.ID",
					FKPairs: []FKPair{{
						From: FKColRef{Schema: "S", Table: "ORDER_ITEMS", Col: "ORDER_ID"},
						To:   FKColRef{Schema: "S", Table: "ORDERS", Col: "ID"},
					}},
					IsIntermediate: true,
				},
				{
					Table:       TableRef{Schema: "S", Name: "USERS"},
					JoinType:    "LEFT",
					OnCondition: "S.ORDERS.USER_ID = S.USERS.ID",
					FKPairs: []FKPair{{
						From: FKColRef{Schema: "S", Table: "ORDERS", Col: "USER_ID"},
						To:   FKColRef{Schema: "S", Table: "USERS", Col: "ID"},
					}},
				},
			},
			SelectedColumns: map[string][]string{},
		})
		assertContains(t, sql, `FROM "MY_DB"."S"."ORDER_ITEMS" t1`)
		assertContains(t, sql, `INNER JOIN "MY_DB"."S"."ORDERS" t2 ON t1."ORDER_ID" = t2."ID"`)
		assertContains(t, sql, `LEFT JOIN "MY_DB"."S"."USERS" t3 ON t2."USER_ID" = t3."ID"`)
		assertContains(t, sql, "t1.*")
		assertContains(t, sql, "t2.*")
		assertContains(t, sql, "t3.*")
	})

	t.Run("cross-schema", func(t *testing.T) {
		sql := BuildJoinSQL(JoinQueryState{
			Database:  "MY_DB",
			BaseTable: TableRef{Schema: "SALES", Name: "ORDERS"},
			Joins: []JoinEntry{{
				Table:       TableRef{Schema: "CATALOG", Name: "PRODUCTS"},
				JoinType:    "INNER",
				OnCondition: "SALES.ORDERS.PRODUCT_ID = CATALOG.PRODUCTS.ID",
				FKPairs: []FKPair{{
					From: FKColRef{Schema: "SALES", Table: "ORDERS", Col: "PRODUCT_ID"},
					To:   FKColRef{Schema: "CATALOG", Table: "PRODUCTS", Col: "ID"},
				}},
			}},
			SelectedColumns: map[string][]string{},
		})
		assertContains(t, sql, `FROM "MY_DB"."SALES"."ORDERS" t1`)
		assertContains(t, sql, `INNER JOIN "MY_DB"."CATALOG"."PRODUCTS" t2 ON t1."PRODUCT_ID" = t2."ID"`)
	})

	t.Run("LIMIT 1000", func(t *testing.T) {
		sql := BuildJoinSQL(makeState())
		assertContains(t, sql, "LIMIT 1000;")
	})

	t.Run("reserved keyword as table name is properly quoted", func(t *testing.T) {
		sql := BuildJoinSQL(JoinQueryState{
			Database:  "MY_DB",
			BaseTable: TableRef{Schema: "S", Name: "SELECT"},
			Joins: []JoinEntry{{
				Table:       TableRef{Schema: "S", Name: "FROM"},
				JoinType:    "INNER",
				OnCondition: "S.SELECT.ID = S.FROM.SELECT_ID",
				FKPairs: []FKPair{{
					From: FKColRef{Schema: "S", Table: "SELECT", Col: "ID"},
					To:   FKColRef{Schema: "S", Table: "FROM", Col: "SELECT_ID"},
				}},
			}},
			SelectedColumns: map[string][]string{},
		})
		// QuoteIdent wraps everything, so reserved keywords are safe.
		assertContains(t, sql, `"SELECT"`)
		assertContains(t, sql, `"FROM"`)
		// Column names are also quoted via fkPairs path.
		assertContains(t, sql, `t1."ID" = t2."SELECT_ID"`)
	})

	t.Run("fallback ON condition without fkPairs", func(t *testing.T) {
		sql := BuildJoinSQL(JoinQueryState{
			Database:  "MY_DB",
			BaseTable: TableRef{Schema: "S", Name: "ORDERS"},
			Joins: []JoinEntry{{
				Table:       TableRef{Schema: "S", Name: "USERS"},
				JoinType:    "INNER",
				OnCondition: "S.ORDERS.USER_ID = S.USERS.ID",
				// FKPairs intentionally nil — exercises the fallback path.
			}},
			SelectedColumns: map[string][]string{},
		})
		assertContains(t, sql, "ON t1.USER_ID = t2.ID")
	})
}
