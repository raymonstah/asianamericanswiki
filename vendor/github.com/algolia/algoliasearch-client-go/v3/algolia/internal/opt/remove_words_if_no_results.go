// Code generated by go generate. DO NOT EDIT.

package opt

import (
	"github.com/algolia/algoliasearch-client-go/v3/algolia/opt"
)

// ExtractRemoveWordsIfNoResults returns the first found RemoveWordsIfNoResultsOption from the
// given variadic arguments or nil otherwise.
func ExtractRemoveWordsIfNoResults(opts ...interface{}) *opt.RemoveWordsIfNoResultsOption {
	for _, o := range opts {
		if v, ok := o.(*opt.RemoveWordsIfNoResultsOption); ok {
			return v
		}
	}
	return nil
}
