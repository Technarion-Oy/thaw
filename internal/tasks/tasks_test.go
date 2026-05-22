package tasks

import (
	"testing"

	"thaw/internal/snowflake"
)

func TestBareIdent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`"MY_TASK"`, "MY_TASK"},
		{`"my""task"`, `my"task`},    // embedded double-quote, surrounded
		{`my""task`, `my"task`},      // embedded double-quote, no surrounding quotes
		{`MY_TASK`, "MY_TASK"},        // no quotes at all
		{`""`, ""},                    // just quotes, empty name
		{`"a""b""c"`, `a"b"c`},       // multiple embedded quotes
	}
	for _, c := range cases {
		got := bareIdent(c.in)
		if got != c.want {
			t.Errorf("bareIdent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestQRoundTrip checks that snowflake.QuoteIdent() and bareIdent() are inverse operations.
func TestQRoundTrip(t *testing.T) {
	names := []string{
		"MY_TASK",
		`my"task`,
		`has""two`,
		`"leading`,
		`trailing"`,
	}
	for _, name := range names {
		quoted := snowflake.QuoteIdent(name)
		got := bareIdent(quoted)
		if got != name {
			t.Errorf("bareIdent(snowflake.QuoteIdent(%q)) = %q, want %q", name, got, name)
		}
	}
}

func TestParsePredecessorRefs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"empty brackets", "[]", nil},
		{"nil string", "<nil>", nil},
		{"null string", "null", nil},
		{"single unqualified", `"TASK_A"`, []string{"TASK_A"}},
		{"single qualified", `"DB"."SCH"."TASK_A"`, []string{"TASK_A"}},
		{"multiple qualified", `["DB"."SCH"."TASK_A","DB"."SCH"."TASK_B"]`, []string{"TASK_A", "TASK_B"}},
		{"dotted names", `[DB.SCH.TASK_A]`, []string{"TASK_A"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parsePredecessorRefs(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("parsePredecessorRefs(%q) = %v (len %d), want %v (len %d)", c.in, got, len(got), c.want, len(c.want))
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("parsePredecessorRefs(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestGetTopologicalOrder(t *testing.T) {
	t.Run("simple chain Root→A→B", func(t *testing.T) {
		rows := []StatusRow{
			{Name: "ROOT", Predecessors: ""},
			{Name: "A", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "B", Predecessors: `["DB"."SCH"."A"]`},
		}
		result := GetTopologicalOrder(rows, "ROOT")

		if len(result.TopoOrder) != 3 {
			t.Fatalf("expected 3 tasks in topoOrder, got %d: %v", len(result.TopoOrder), result.TopoOrder)
		}
		// ROOT must come first, A before B.
		indexOf := func(name string) int {
			for i, n := range result.TopoOrder {
				if n == name {
					return i
				}
			}
			return -1
		}
		if indexOf("ROOT") != 0 {
			t.Errorf("ROOT should be first, got index %d in %v", indexOf("ROOT"), result.TopoOrder)
		}
		if indexOf("A") >= indexOf("B") {
			t.Errorf("A should come before B: %v", result.TopoOrder)
		}
		if len(result.FinalizerNames) != 0 {
			t.Errorf("expected no finalizers, got %v", result.FinalizerNames)
		}
	})

	t.Run("diamond dependency", func(t *testing.T) {
		// ROOT → A → C
		// ROOT → B → C
		rows := []StatusRow{
			{Name: "ROOT", Predecessors: ""},
			{Name: "A", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "B", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "C", Predecessors: `["DB"."SCH"."A","DB"."SCH"."B"]`},
		}
		result := GetTopologicalOrder(rows, "ROOT")

		if len(result.TopoOrder) != 4 {
			t.Fatalf("expected 4 tasks, got %d: %v", len(result.TopoOrder), result.TopoOrder)
		}
		indexOf := func(name string) int {
			for i, n := range result.TopoOrder {
				if n == name {
					return i
				}
			}
			return -1
		}
		if indexOf("ROOT") != 0 {
			t.Errorf("ROOT should be first: %v", result.TopoOrder)
		}
		if indexOf("A") >= indexOf("C") {
			t.Errorf("A should come before C: %v", result.TopoOrder)
		}
		if indexOf("B") >= indexOf("C") {
			t.Errorf("B should come before C: %v", result.TopoOrder)
		}
	})

	t.Run("with finalizer", func(t *testing.T) {
		rows := []StatusRow{
			{Name: "ROOT", Predecessors: ""},
			{Name: "A", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "FIN", Predecessors: "", Finalize: `"DB"."SCH"."ROOT"`},
		}
		result := GetTopologicalOrder(rows, "ROOT")

		if len(result.TopoOrder) != 2 {
			t.Fatalf("expected 2 tasks in topoOrder (ROOT, A), got %d: %v", len(result.TopoOrder), result.TopoOrder)
		}
		if len(result.FinalizerNames) != 1 || result.FinalizerNames[0] != "FIN" {
			t.Errorf("expected finalizerNames=[FIN], got %v", result.FinalizerNames)
		}
		// Suspend: ROOT, A, FIN
		if len(result.SuspendOrder) != 3 {
			t.Fatalf("expected 3 in suspendOrder, got %d: %v", len(result.SuspendOrder), result.SuspendOrder)
		}
		if result.SuspendOrder[len(result.SuspendOrder)-1] != "FIN" {
			t.Errorf("FIN should be last in suspendOrder: %v", result.SuspendOrder)
		}
		// Resume: A, FIN, ROOT
		if len(result.ResumeOrder) != 3 {
			t.Fatalf("expected 3 in resumeOrder, got %d: %v", len(result.ResumeOrder), result.ResumeOrder)
		}
		if result.ResumeOrder[len(result.ResumeOrder)-1] != "ROOT" {
			t.Errorf("ROOT should be last in resumeOrder: %v", result.ResumeOrder)
		}
	})

	t.Run("single task graph", func(t *testing.T) {
		rows := []StatusRow{
			{Name: "SOLO", Predecessors: ""},
		}
		result := GetTopologicalOrder(rows, "SOLO")

		if len(result.TopoOrder) != 1 || result.TopoOrder[0] != "SOLO" {
			t.Errorf("expected topoOrder=[SOLO], got %v", result.TopoOrder)
		}
		if len(result.ResumeOrder) != 1 || result.ResumeOrder[0] != "SOLO" {
			t.Errorf("expected resumeOrder=[SOLO], got %v", result.ResumeOrder)
		}
	})

	t.Run("missing root returns empty", func(t *testing.T) {
		rows := []StatusRow{
			{Name: "A", Predecessors: ""},
		}
		result := GetTopologicalOrder(rows, "NONEXISTENT")

		if len(result.TopoOrder) != 0 {
			t.Errorf("expected empty topoOrder, got %v", result.TopoOrder)
		}
		if len(result.SuspendOrder) != 0 {
			t.Errorf("expected empty suspendOrder, got %v", result.SuspendOrder)
		}
	})

	t.Run("case insensitive root lookup", func(t *testing.T) {
		rows := []StatusRow{
			{Name: "MyTask", Predecessors: ""},
			{Name: "Child", Predecessors: `["DB"."SCH"."MyTask"]`},
		}
		result := GetTopologicalOrder(rows, "mytask")

		if len(result.TopoOrder) != 2 {
			t.Fatalf("expected 2 tasks, got %d: %v", len(result.TopoOrder), result.TopoOrder)
		}
		// Should preserve original casing.
		if result.TopoOrder[0] != "MyTask" {
			t.Errorf("expected original casing MyTask, got %s", result.TopoOrder[0])
		}
	})

	t.Run("complex DAG with multiple predecessors", func(t *testing.T) {
		// ROOT → A → C → E
		// ROOT → B → D → E
		// A → D (cross-link)
		rows := []StatusRow{
			{Name: "ROOT", Predecessors: ""},
			{Name: "A", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "B", Predecessors: `["DB"."SCH"."ROOT"]`},
			{Name: "C", Predecessors: `["DB"."SCH"."A"]`},
			{Name: "D", Predecessors: `["DB"."SCH"."B","DB"."SCH"."A"]`},
			{Name: "E", Predecessors: `["DB"."SCH"."C","DB"."SCH"."D"]`},
		}
		result := GetTopologicalOrder(rows, "ROOT")

		if len(result.TopoOrder) != 6 {
			t.Fatalf("expected 6 tasks, got %d: %v", len(result.TopoOrder), result.TopoOrder)
		}

		indexOf := func(name string) int {
			for i, n := range result.TopoOrder {
				if n == name {
					return i
				}
			}
			return -1
		}

		// Check all dependency constraints.
		deps := map[string][]string{
			"A": {"ROOT"},
			"B": {"ROOT"},
			"C": {"A"},
			"D": {"B", "A"},
			"E": {"C", "D"},
		}
		for task, preds := range deps {
			for _, pred := range preds {
				if indexOf(pred) >= indexOf(task) {
					t.Errorf("%s (idx %d) should come before %s (idx %d): %v",
						pred, indexOf(pred), task, indexOf(task), result.TopoOrder)
				}
			}
		}
	})
}
