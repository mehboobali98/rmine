package redmine

import (
	"errors"
	"strings"
	"testing"
)

func TestFindIDByNameToleratesSpellingTheWebUIHides(t *testing.T) {
	items := []IDName{
		{ID: 6, Name: "SubTask"},
		{ID: 10, Name: "Internal "},
		{ID: 3, Name: "In Progress"},
		{ID: 21, Name: "1.1"},
	}
	cases := map[string]int{
		"SubTask":     6,
		"subtask":     6,
		"Sub-task":    6,
		"Sub Task":    6,
		"sub_task":    6,
		"Internal":    10,
		"Internal ":   10,
		"in-progress": 3,
		"inprogress":  3,
		"1.1":         21,
	}
	for name, want := range cases {
		got, err := findIDByName(items, name)
		if err != nil {
			t.Errorf("findIDByName(%q) failed: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("findIDByName(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestFindIDByNameKeepsDotsSignificant(t *testing.T) {
	_, err := findIDByName([]IDName{{ID: 21, Name: "1.1"}}, "11")
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("\"11\" should not match version \"1.1\", got %v", err)
	}
}

func TestFindIDByNamePrefersAnExactMatchOverALooseOne(t *testing.T) {
	items := []IDName{{ID: 1, Name: "Sub Task"}, {ID: 2, Name: "SubTask"}}
	got, err := findIDByName(items, "subtask")
	if err != nil || got != 2 {
		t.Fatalf("findIDByName = %d, %v; want 2", got, err)
	}
}

func TestFindIDByNameRefusesAnAmbiguousLooseMatch(t *testing.T) {
	items := []IDName{{ID: 1, Name: "Sub Task"}, {ID: 2, Name: "Sub-Task"}}
	_, err := findIDByName(items, "subtask")
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	if !strings.Contains(err.Error(), `"Sub Task"`) || !strings.Contains(err.Error(), `"Sub-Task"`) {
		t.Errorf("error should name both candidates, got %q", err)
	}
}
