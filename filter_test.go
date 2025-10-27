package main

import (
	"testing"
)

func TestFilterHistorySorting(t *testing.T) {
	// Sample history
	history := []string{
		"history command",        // prefix match for "hist"
		"git push",              // no match for "hist"
		"show history",          // substring match for "hist"
		"historical data",       // prefix match for "hist"
		"git pull",              // no match for "hist"
		"bash history file",     // substring match for "hist"
		"git add .",             // no match for "hist"
	}

	// Test filtering with "hist"
	filtered := filterAndSortCommands(history, "hist")

	if len(filtered) != 4 {
		t.Errorf("Expected 4 matches for 'hist', got %d", len(filtered))
	}

	// Check that prefix matches come first
	if len(filtered) >= 2 {
		// First two should be "history command" and "historical data" (prefix matches)
		cmd1 := filtered[0]
		cmd2 := filtered[1]

		isPrefixMatch1 := cmd1 == "history command" || cmd1 == "historical data"
		isPrefixMatch2 := cmd2 == "history command" || cmd2 == "historical data"

		if !isPrefixMatch1 {
			t.Errorf("First result should be a prefix match, got: %s", cmd1)
		}
		if !isPrefixMatch2 {
			t.Errorf("Second result should be a prefix match, got: %s", cmd2)
		}
	}

	// Check that substring matches come after prefix matches
	if len(filtered) >= 4 {
		cmd3 := filtered[2]
		cmd4 := filtered[3]

		isSubstringMatch3 := cmd3 == "show history" || cmd3 == "bash history file"
		isSubstringMatch4 := cmd4 == "show history" || cmd4 == "bash history file"

		if !isSubstringMatch3 {
			t.Errorf("Third result should be a substring match, got: %s", cmd3)
		}
		if !isSubstringMatch4 {
			t.Errorf("Fourth result should be a substring match, got: %s", cmd4)
		}
	}

	t.Logf("\nFiltered results for 'hist':")
	for i, cmd := range filtered {
		t.Logf("  [%d] %s", i, cmd)
	}
}

func TestFilterHistoryFuzzyMatches(t *testing.T) {
	history := []string{
		"git push",              // prefix match for "git"
		"git pull",              // prefix match for "git"
		"go install tools",      // fuzzy match for "git" (g-i-t)
		"gradle integration test", // fuzzy match for "git" (g-i-t)
		"git commit",            // prefix match for "git"
	}

	filtered := filterAndSortCommands(history, "git")

	if len(filtered) != 5 {
		t.Errorf("Expected 5 matches for 'git', got %d", len(filtered))
	}

	// First three should be prefix matches (git push, git pull, git commit)
	for i := 0; i < 3 && i < len(filtered); i++ {
		cmd := filtered[i]
		if len(cmd) < 3 || cmd[0:3] != "git" {
			t.Errorf("Result %d should be a prefix match, got: %s", i, cmd)
		}
	}

	// Last two should be fuzzy matches
	if len(filtered) >= 5 {
		cmd4 := filtered[3]
		cmd5 := filtered[4]

		isFuzzyMatch4 := cmd4 == "go install tools" || cmd4 == "gradle integration test"
		isFuzzyMatch5 := cmd5 == "go install tools" || cmd5 == "gradle integration test"

		if !isFuzzyMatch4 {
			t.Errorf("Result 4 should be a fuzzy match, got: %s", cmd4)
		}
		if !isFuzzyMatch5 {
			t.Errorf("Result 5 should be a fuzzy match, got: %s", cmd5)
		}
	}

	t.Logf("\nFiltered results for 'git':")
	for i, cmd := range filtered {
		t.Logf("  [%d] %s", i, cmd)
	}
}
