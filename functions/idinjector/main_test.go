package main

import (
	"fmt"
	"testing"

	"github.com/segmentio/ksuid"
	"github.com/tj/assert"
)

func TestInjectID(t *testing.T) {
	content := `---
title: "FirstName LastName"
draft: false
---

This is some more content`

	id := ksuid.New().String()

	wantNewContent := fmt.Sprintf(`---
id: %q
title: "FirstName LastName"
draft: false
---

This is some more content`, id)
	human, err := injectID([]byte(content), id)
	assert.NoError(t, err)

	assert.Equal(t, wantNewContent, string(human.NewContent))
}

func TestInjectID_AlreadyHasID(t *testing.T) {
	content := `---
title: "FirstName LastName"
id: blah-id
aka: []
draft: false
---`

	_, err := injectID([]byte(content), ksuid.New().String())
	assert.EqualError(t, err, "id already exists")
}

func TestParseName(t *testing.T) {
	tcs := map[string]struct {
		line    string
		want    string
		hasName bool
	}{
		"no-quotes":                {line: "title: Raymond Ho", want: "Raymond Ho", hasName: true},
		"quotes":                   {line: `title: "Raymond Ho"`, want: "Raymond Ho", hasName: true},
		"single-quotes":            {line: `title: 'Raymond Ho'`, want: "Raymond Ho", hasName: true},
		"name-contains-apostrophe": {line: `title: fir'st last`, want: "fir'st last", hasName: true},
		"extra-spaces":             {line: ` otherfield: FooBar `, want: "", hasName: false},
		"name-missing":             {line: `otherfield: FooBar`, want: "", hasName: false},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			got, hasName := parseName(tc.line)
			assert.Equal(t, tc.hasName, hasName)
			assert.Equal(t, tc.want, got)
		})
	}
}
