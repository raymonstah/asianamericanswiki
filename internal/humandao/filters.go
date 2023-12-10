package humandao

import (
	"slices"
	"time"
)

type Filterable []Human

func (f Filterable) ByGender(gender Gender) Filterable {
	filtered := make([]Human, 0, len(f))
	for _, human := range f {
		if human.Gender == gender {
			filtered = append(filtered, human)
		}
	}

	return filtered
}

func (f Filterable) ByEthnicity(ethnicity string) Filterable {
	filtered := make([]Human, 0, len(f))
	for _, human := range f {
		if slices.Contains(human.Ethnicity, ethnicity) {
			filtered = append(filtered, human)
		}
	}

	return filtered
}

func (f Filterable) ByAgeOlderThan(age time.Time) Filterable {
	filtered := make([]Human, 0, len(f))
	for _, human := range f {
		// convert human.DOB into a time.Time
		dobTime, err := time.Parse("2006-01-02", human.DOB)
		if err != nil {
			continue
		}
		if dobTime.Before(age) {
			filtered = append(filtered, human)
		}
	}

	return filtered
}

func (f Filterable) ByAgeYoungerThan(age time.Time) Filterable {
	filtered := make([]Human, 0, len(f))
	for _, human := range f {
		// convert human.DOB into a time.Time
		dobTime, err := time.Parse("2006-01-02", human.DOB)
		if err != nil {
			continue
		}
		if dobTime.After(age) {
			filtered = append(filtered, human)
		}
	}

	return filtered
}

func (f Filterable) ByTags(tags ...string) Filterable {
	filteredMap := make([]int, len(f)) // to prevent dupes
	for i, human := range f {
		for _, tag := range tags {
			if slices.Contains(human.Tags, tag) {
				filteredMap[i] = 1
			}
		}
	}

	filtered := make([]Human, 0, len(filteredMap))
	for i, val := range filteredMap {
		if val == 1 {
			filtered = append(filtered, f[i])
		}
	}

	return filtered
}

func (f Filterable) Humans() []Human {
	return f
}
