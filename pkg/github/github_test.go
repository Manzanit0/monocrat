package github

import (
	"testing"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestHelloName(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		runID int64
		fail  bool
	}{
		{
			name:  "real URL",
			url:   "https://api.github.com/repos/Manzanit0/gitops-env-per-folder-poc/actions/runs/4810948216/deployment_protection_rule",
			runID: 4810948216,
			fail:  false,
		},
		{
			name:  "without protocol",
			url:   "api.github.com/repos/Manzanit0/gitops-env-per-folder-poc/actions/runs/4810948216/deployment_protection_rule",
			runID: 4810948216,
			fail:  false,
		},
		{
			name:  "substring of the endpoint",
			url:   "actions/runs/4810948216/deployment_protection_rule",
			runID: 4810948216,
			fail:  false,
		},
		{
			name:  "runID has characters",
			url:   "actions/runs/4810948216lkjklj/deployment_protection_rule",
			runID: 1,
			fail:  true,
		},
		{
			name:  "completely different endpoint",
			url:   "foo/bar/4810948216/baz",
			runID: 1,
			fail:  true,
		},
	}

	for idx := range tests {
		t.Run(tests[idx].name, func(t *testing.T) {
			runID, err := extractRunID(tests[idx].url)
			if err != nil && tests[idx].fail {
				return
			} else if err != nil {
				t.Fatalf(err.Error())
			}

			if runID != tests[idx].runID {
				t.Fatalf("runID no match: %d", runID)
			}
		})
	}
}
