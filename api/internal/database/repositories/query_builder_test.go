package repositories

import (
	"strings"
	"testing"
)

func TestQueryBuilder_Basic(t *testing.T) {
	t.Run("empty query builder", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		query, args := qb.Build()

		if query != "SELECT * FROM users WHERE 1=1" {
			t.Errorf("expected base query unchanged, got %q", query)
		}
		if len(args) != 0 {
			t.Errorf("expected no args, got %d", len(args))
		}
	})

	t.Run("single condition", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddCondition("id = $%d", "user-123")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND id = $1"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 1 || args[0] != "user-123" {
			t.Errorf("expected args [user-123], got %v", args)
		}
	})

	t.Run("multiple conditions", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddCondition("id = $%d", "user-123")
		qb.AddCondition("status = $%d", "active")
		qb.AddCondition("org_id = $%d", "org-456")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND id = $1 AND status = $2 AND org_id = $3"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 3 {
			t.Errorf("expected 3 args, got %d", len(args))
		}
	})
}

func TestQueryBuilder_OrderBy(t *testing.T) {
	t.Run("valid single column", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.OrderBy("name ASC")
		query, _ := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 ORDER BY name ASC"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
	})

	t.Run("valid column with NULLS LAST", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.OrderBy("created_at DESC NULLS LAST")
		query, _ := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 ORDER BY created_at DESC NULLS LAST"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
	})

	t.Run("invalid column rejected", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.OrderBy("DROP TABLE users--")
		query, _ := qb.Build()

		// Should NOT contain ORDER BY since column is invalid
		if query != "SELECT * FROM users WHERE 1=1" {
			t.Errorf("invalid ORDER BY should be rejected, got %q", query)
		}
	})

	t.Run("SQL injection in ORDER BY rejected", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.OrderBy("name; DELETE FROM users")
		query, _ := qb.Build()

		// Should only get valid part or nothing
		if query == "SELECT * FROM users WHERE 1=1 ORDER BY name; DELETE FROM users" {
			t.Errorf("SQL injection in ORDER BY should be rejected")
		}
	})

	t.Run("OrderByRaw allows any value", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.OrderByRaw("RANDOM()")
		query, _ := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 ORDER BY RANDOM()"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
	})
}

func TestQueryBuilder_LimitOffset(t *testing.T) {
	t.Run("with limit", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.Limit(10)
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 LIMIT $1"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 1 || args[0] != 10 {
			t.Errorf("expected args [10], got %v", args)
		}
	})

	t.Run("with offset", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.Offset(20)
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 OFFSET $1"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 1 || args[0] != 20 {
			t.Errorf("expected args [20], got %v", args)
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.Limit(10)
		qb.Offset(20)
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 LIMIT $1 OFFSET $2"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 2 || args[0] != 10 || args[1] != 20 {
			t.Errorf("expected args [10, 20], got %v", args)
		}
	})

	t.Run("zero limit ignored", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.Limit(0)
		query, args := qb.Build()

		if query != "SELECT * FROM users WHERE 1=1" {
			t.Errorf("zero limit should be ignored, got %q", query)
		}
		if len(args) != 0 {
			t.Errorf("expected no args, got %v", args)
		}
	})

	t.Run("negative limit ignored", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.Limit(-5)
		query, args := qb.Build()

		if query != "SELECT * FROM users WHERE 1=1" {
			t.Errorf("negative limit should be ignored, got %q", query)
		}
		if len(args) != 0 {
			t.Errorf("expected no args, got %v", args)
		}
	})
}

func TestQueryBuilder_DefaultLimit(t *testing.T) {
	t.Run("uses explicit limit when positive", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.DefaultLimit(25, 100)
		query, args := qb.Build()

		if !strings.Contains(query, "LIMIT $1") {
			t.Errorf("expected LIMIT clause, got %q", query)
		}
		if len(args) != 1 || args[0] != 25 {
			t.Errorf("expected args [25], got %v", args)
		}
	})

	t.Run("uses default when limit is zero", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.DefaultLimit(0, 100)
		query, args := qb.Build()

		if !strings.Contains(query, "LIMIT $1") {
			t.Errorf("expected LIMIT clause, got %q", query)
		}
		if len(args) != 1 || args[0] != 100 {
			t.Errorf("expected args [100], got %v", args)
		}
	})

	t.Run("uses default when limit is negative", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.DefaultLimit(-5, 100)
		query, args := qb.Build()

		if !strings.Contains(query, "LIMIT $1") {
			t.Errorf("expected LIMIT clause, got %q", query)
		}
		if len(args) != 1 || args[0] != 100 {
			t.Errorf("expected args [100], got %v", args)
		}
	})
}

func TestQueryBuilder_ComplexQuery(t *testing.T) {
	t.Run("full query with all features", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM enrollments e WHERE 1=1")
		qb.AddCondition("e.user_id = $%d", "user-123")
		qb.AddCondition("e.status = $%d", "active")
		qb.OrderByRaw("e.last_activity_at DESC NULLS LAST, e.enrolled_at DESC")
		qb.Limit(50)
		qb.Offset(100)

		query, args := qb.Build()

		expected := "SELECT * FROM enrollments e WHERE 1=1 AND e.user_id = $1 AND e.status = $2 ORDER BY e.last_activity_at DESC NULLS LAST, e.enrolled_at DESC LIMIT $3 OFFSET $4"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 4 {
			t.Errorf("expected 4 args, got %d: %v", len(args), args)
		}
		if args[0] != "user-123" || args[1] != "active" || args[2] != 50 || args[3] != 100 {
			t.Errorf("unexpected args: %v", args)
		}
	})
}

func TestQueryBuilder_RawCondition(t *testing.T) {
	t.Run("raw condition without parameters", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddRawCondition("status != 'deleted'")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND status != 'deleted'"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 0 {
			t.Errorf("expected no args for raw condition, got %v", args)
		}
	})

	t.Run("mix of parameterized and raw conditions", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddCondition("org_id = $%d", "org-123")
		qb.AddRawCondition("is_active = true")
		qb.AddCondition("role = $%d", "admin")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND org_id = $1 AND is_active = true AND role = $2"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 2 || args[0] != "org-123" || args[1] != "admin" {
			t.Errorf("expected args [org-123, admin], got %v", args)
		}
	})
}

func TestQueryBuilder_AddConditionMulti(t *testing.T) {
	t.Run("three placeholders in one condition", func(t *testing.T) {
		// Typical multi-column ILIKE search clause.
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		like := "%alice%"
		qb.AddConditionMulti(
			"(username ILIKE $%d OR email ILIKE $%d OR display_name ILIKE $%d)",
			like, like, like,
		)
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND (username ILIKE $1 OR email ILIKE $2 OR display_name ILIKE $3)"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 3 || args[0] != like || args[1] != like || args[2] != like {
			t.Errorf("expected 3 repeated search args, got %v", args)
		}
	})

	t.Run("advances argNum so subsequent conditions don't collide", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddConditionMulti(
			"(a = $%d OR b = $%d)",
			"x", "y",
		)
		qb.AddCondition("role = $%d", "admin")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND (a = $1 OR b = $2) AND role = $3"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 3 || args[2] != "admin" {
			t.Errorf("expected third arg to be admin, got %v", args)
		}
	})

	t.Run("no args falls back to raw condition", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddConditionMulti("is_active = true")
		query, args := qb.Build()

		expected := "SELECT * FROM users WHERE 1=1 AND is_active = true"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 0 {
			t.Errorf("expected no args, got %v", args)
		}
	})
}

func TestQueryBuilder_HelperMethods(t *testing.T) {
	t.Run("Args returns current arguments", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		qb.AddCondition("id = $%d", "user-123")
		qb.AddCondition("status = $%d", "active")

		args := qb.Args()
		if len(args) != 2 {
			t.Errorf("expected 2 args, got %d", len(args))
		}
	})

	t.Run("ArgCount returns current count", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		if qb.ArgCount() != 0 {
			t.Errorf("expected 0 args initially, got %d", qb.ArgCount())
		}

		qb.AddCondition("id = $%d", "user-123")
		if qb.ArgCount() != 1 {
			t.Errorf("expected 1 arg, got %d", qb.ArgCount())
		}

		qb.AddCondition("status = $%d", "active")
		if qb.ArgCount() != 2 {
			t.Errorf("expected 2 args, got %d", qb.ArgCount())
		}
	})
}

func TestQueryBuilder_ChainedCalls(t *testing.T) {
	t.Run("fluent interface", func(t *testing.T) {
		qb := NewQueryBuilder("SELECT * FROM users WHERE 1=1")
		query, args := qb.
			AddCondition("org_id = $%d", "org-1").
			AddCondition("status = $%d", "active").
			OrderByRaw("name ASC").
			Limit(10).
			Build()

		expected := "SELECT * FROM users WHERE 1=1 AND org_id = $1 AND status = $2 ORDER BY name ASC LIMIT $3"
		if query != expected {
			t.Errorf("expected %q, got %q", expected, query)
		}
		if len(args) != 3 {
			t.Errorf("expected 3 args, got %d", len(args))
		}
	})
}
