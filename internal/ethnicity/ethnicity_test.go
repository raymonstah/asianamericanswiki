package ethnicity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEthnicity(t *testing.T) {
	require.Len(t, All, 24)
}
