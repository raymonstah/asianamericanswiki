package humandao

import (
	"slices"
	"strings"
	"time"
)

type FilterOpt func(f Filterable) Filterable
type Filterable []Human

func ApplyFilters(f Filterable, opts ...FilterOpt) Filterable {
	for _, opt := range opts {
		f = opt(f)
	}
	return f
}

// ByGender is a FilterOpts implementation to filter by gender
func ByGender(gender Gender) FilterOpt {
	return func(f Filterable) Filterable {
		filtered := make([]Human, 0, len(f))
		for _, human := range f {
			if human.Gender == gender {
				filtered = append(filtered, human)
			}
		}
		return filtered
	}
}

func ByEthnicity(ethnicity string) FilterOpt {
	return func(f Filterable) Filterable {
		filtered := make([]Human, 0, len(f))
		for _, human := range f {
			if slices.ContainsFunc(human.Ethnicity, func(e string) bool {
				return strings.EqualFold(e, ethnicity)
			}) {
				filtered = append(filtered, human)
			}
		}

		return filtered

	}
}

func ByAgeOlderThan(age time.Time) FilterOpt {
	return func(f Filterable) Filterable {
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
}

func ByAgeYoungerThan(age time.Time) FilterOpt {
	return func(f Filterable) Filterable {
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
}

func ByTags(tags ...string) FilterOpt {
	return func(f Filterable) Filterable {
		filteredMap := make([]int, len(f)) // to prevent dupes
		for i, human := range f {
			for _, tag := range tags {
				if slices.ContainsFunc(human.Tags, func(s string) bool {
					return strings.EqualFold(s, tag)
				}) {
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
}

func ByIDs(ids ...string) FilterOpt {
	return func(f Filterable) Filterable {
		idToHuman := make(map[string]Human, len(f))
		for _, human := range f {
			idToHuman[human.ID] = human
		}

		filtered := make([]Human, 0, len(f))
		for _, id := range ids {
			human, ok := idToHuman[id]
			if !ok {
				continue
			}
			filtered = append(filtered, human)
		}

		return filtered
	}
}
