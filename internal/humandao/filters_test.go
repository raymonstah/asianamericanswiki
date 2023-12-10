package humandao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFilterable_ByGender(t *testing.T) {
	f := Filterable{
		{Gender: GenderMale},
		{Gender: GenderFemale},
		{Gender: GenderMale},
	}

	result := f.ByGender(GenderMale)

	expected := Filterable{
		{Gender: GenderMale},
		{Gender: GenderMale},
	}

	require.Equal(t, expected, result)
}

func TestFilterable_ByEthnicity(t *testing.T) {
	f := Filterable{
		{Ethnicity: []string{"Chinese"}},
		{Ethnicity: []string{"Korean"}},
		{Ethnicity: []string{"Korean", "Chinese"}},
	}

	result := f.ByEthnicity("Chinese")

	expected := Filterable{
		{Ethnicity: []string{"Chinese"}},
		{Ethnicity: []string{"Korean", "Chinese"}},
	}

	require.Equal(t, expected, result)
}

func TestFilterable_ByAgeGreaterThan(t *testing.T) {
	f := Filterable{
		{DOB: "1994-01-01"},
		{DOB: "1995-01-01"},
		{DOB: "1996-01-01"},
	}

	age, err := time.Parse("2006-01-02", "1997-01-01")
	require.NoError(t, err)
	result := f.ByAgeOlderThan(age)

	require.Equal(t, f, result)
}

func TestFilterable_ByAgeLessThan(t *testing.T) {
	f := Filterable{
		{DOB: "1994-01-01"},
		{DOB: "1995-01-01"},
		{DOB: "1996-01-01"},
	}

	age, err := time.Parse("2006-01-02", "1994-01-01")
	require.NoError(t, err)
	result := f.ByAgeYoungerThan(age)
	expected := Filterable{
		{DOB: "1995-01-01"},
		{DOB: "1996-01-01"},
	}

	require.Equal(t, expected, result)
}

func TestFilterable_ByTags(t *testing.T) {
	f := Filterable{
		{Tags: []string{"tag1", "tag2"}},
		{Tags: []string{"tag2", "tag3"}},
		{Tags: []string{"tag1", "tag3"}},
	}

	result := f.ByTags("tag1")

	expected := Filterable{
		{Tags: []string{"tag1", "tag2"}},
		{Tags: []string{"tag1", "tag3"}},
	}

	require.Equal(t, expected, result)
}

func TestFilterable_Chained(t *testing.T) {
	f := Filterable{
		{DOB: "1994-01-01", Tags: []string{"tag1", "tag2"}, Ethnicity: []string{"Chinese"}},
		{DOB: "1995-01-01", Tags: []string{"tag2", "tag3"}, Ethnicity: []string{"Korean", "Chinese"}},
		{DOB: "1996-01-01", Tags: []string{"tag1", "tag3"}, Ethnicity: []string{"Korean"}},
	}

	result := f.
		ByAgeOlderThan(time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)).   // 94
		ByAgeYoungerThan(time.Date(1993, 1, 1, 0, 0, 0, 0, time.UTC)). // 94, 95, 96
		ByTags("tag2").
		ByEthnicity("Chinese")

	expected := Filterable{
		{DOB: "1994-01-01", Tags: []string{"tag1", "tag2"}, Ethnicity: []string{"Chinese"}},
	}
	require.Equal(t, expected, result)
}
