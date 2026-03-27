package plan_test

import (
	"testing"

	"github.com/web-casa/llstack/internal/core/plan"
)

func TestPlanJSONIncludesOperation(t *testing.T) {
	p := plan.New("install", "Install stack")
	p.AddOperation(plan.Operation{
		ID:     "install-apache",
		Kind:   "package.install",
		Target: "httpd",
	})

	raw, err := p.JSON()
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("expected non-empty JSON output")
	}

	if want := `"target": "httpd"`; string(raw) == "" || !contains(string(raw), want) {
		t.Fatalf("expected %s in plan JSON, got %s", want, string(raw))
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && func() bool {
		return stringIndex(haystack, needle) >= 0
	}()
}

func stringIndex(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
