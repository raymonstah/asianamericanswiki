package main

import "testing"

func TestDiff(t *testing.T) {
	testcases := map[string]struct {
		existing, updated                    []string
		expectedToFollow, expectedToUnfollow []string
	}{
		"basic-add-and-delete": {
			existing:           []string{"@user-1"},
			updated:            []string{"@user-2"},
			expectedToFollow:   []string{"@user-2"},
			expectedToUnfollow: []string{"@user-1"},
		},
		"add-few-peeps": {
			existing:         []string{"@user-1"},
			updated:          []string{"@user-1", "@user-2", "@user-3"},
			expectedToFollow: []string{"@user-2", "@user-3"},
		},
		"remove-few-peeps": {
			existing:           []string{"@user-1", "remove-me", "remove-me-too"},
			updated:            []string{"@user-1"},
			expectedToUnfollow: []string{"remove-me", "remove-me-too"},
		},
		"ignore-case": {
			existing: []string{"brendasong"},
			updated:  []string{"BrendaSong"},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			toFollow, toUnfollow := handleDiffs(tc.existing, tc.updated)
			if !equalSlices(toFollow, tc.expectedToFollow) {
				t.Fatalf("toFollow %v is not expected %v", toFollow, tc.expectedToFollow)
			}

			if !equalSlices(toUnfollow, tc.expectedToUnfollow) {
				t.Fatalf("toUnfollow %v is not expected %v", toUnfollow, tc.expectedToUnfollow)
			}
		})
	}
}

func TestParseHandles(t *testing.T) {
	testcases := map[string]struct {
		Raw            string
		ExpectedHandle string
	}{
		"basic-handle":  {Raw: `"@raymond"`, ExpectedHandle: "raymond"},
		"single-quotes": {Raw: `'https://twitter.com/raymond'`, ExpectedHandle: "raymond"},
		"double-quotes": {Raw: `"https://twitter.com/raymond"`, ExpectedHandle: "raymond"},
		"no-quotes":     {Raw: `https://twitter.com/raymond`, ExpectedHandle: "raymond"},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			got := parseHandle(testcase.Raw)
			if got != testcase.ExpectedHandle {
				t.Fatalf("expected %v, got %v", testcase.ExpectedHandle, got)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, item := range b {
		if a[i] != item {
			return false
		}
	}

	return true
}
