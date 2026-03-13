package ethnicity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEthnicity(t *testing.T) {
	require.Len(t, All, 31)
}

func TestValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := Validate([]string{"chinese", "korean"})
		require.NoError(t, err)
	})

	t.Run("invalid", func(t *testing.T) {
		err := Validate([]string{"chinese", "martian"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid ethnicity: martian")
	})
}
