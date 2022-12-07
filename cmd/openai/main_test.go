package main

import (
	"fmt"
	"testing"

	"github.com/tj/assert"
)

func TestOverwriteDescription(t *testing.T) {
	content := `---
title: "FirstName LastName"
draft: false
---

This is the old description`

	wantNewContent := fmt.Sprintf(`---
title: "FirstName LastName"
draft: false
---

This is the new description
`)
	newDescription := "This is the new description\n"
	gotContent, err := overwriteDescription([]byte(content), newDescription)
	assert.NoError(t, err)
	assert.Equal(t, wantNewContent, string(gotContent))
}
