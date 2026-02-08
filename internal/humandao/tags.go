package humandao

import "sort"

// AllTags is a list of allowed tags for a human.
var AllTags = []string{
	"academic",
	"activist",
	"actor",
	"artist",
	"athlete",
	"beauty",
	"chef",
	"comedian",
	"content creator",
	"dancer",
	"designer",
	"director",
	"doctor",
	"engineer",
	"entertainer",
	"entrepreneur",
	"executive",
	"fashion",
	"filmmaker",
	"fitness",
	"founder",
	"influencer",
	"journalist",
	"lawyer",
	"martial artist",
	"model",
	"musician",
	"olympian",
	"philanthropist",
	"photographer",
	"pilot",
	"playwright",
	"politician",
	"producer",
	"professor",
	"rapper",
	"scientist",
	"screenwriter",
	"singer",
	"songwriter",
	"writer",
}

var allTagsMap map[string]struct{}

func init() {
	allTagsMap = make(map[string]struct{}, len(AllTags))
	for _, t := range AllTags {
		allTagsMap[t] = struct{}{}
	}
}

// IsValidTag returns true if the tag is in the allowed list.
func IsValidTag(tag string) bool {
	_, ok := allTagsMap[tag]
	return ok
}

// GetTags returns a sorted list of all allowed tags.
func GetTags() []string {
	sort.Strings(AllTags)
	return AllTags
}