// Code generated by go generate. DO NOT EDIT.

package opt

import (
	"github.com/algolia/algoliasearch-client-go/v3/algolia/opt"
)

// ExtractAttributeCriteriaComputedByMinProximity returns the first found AttributeCriteriaComputedByMinProximityOption from the
// given variadic arguments or nil otherwise.
func ExtractAttributeCriteriaComputedByMinProximity(opts ...interface{}) *opt.AttributeCriteriaComputedByMinProximityOption {
	for _, o := range opts {
		if v, ok := o.(*opt.AttributeCriteriaComputedByMinProximityOption); ok {
			return v
		}
	}
	return nil
}
