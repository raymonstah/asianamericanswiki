// Code generated by go generate. DO NOT EDIT.

package opt

import (
	"encoding/json"
	"reflect"
	"strings"
)

// RestrictIndicesOption is a wrapper for an RestrictIndices option parameter. It holds
// the actual value of the option that can be accessed by calling Get.
type RestrictIndicesOption struct {
	value []string
}

// RestrictIndices wraps the given value into a RestrictIndicesOption.
func RestrictIndices(v ...string) *RestrictIndicesOption {
	return &RestrictIndicesOption{v}
}

// Get retrieves the actual value of the option parameter.
func (o *RestrictIndicesOption) Get() []string {
	if o == nil {
		return []string{}
	}
	return o.value
}

// MarshalJSON implements the json.Marshaler interface for
// RestrictIndicesOption.
func (o RestrictIndicesOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface for
// RestrictIndicesOption.
func (o *RestrictIndicesOption) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.value = []string{}
		return nil
	}
	var s string
	err := json.Unmarshal(data, &s)
	if err == nil {
		o.value = strings.Split(s, ",")
		if len(o.value) == 1 && o.value[0] == "" {
			o.value = []string{}
		}
		return nil
	}
	return json.Unmarshal(data, &o.value)
}

// Equal returns true if the given option is equal to the instance one. In case
// the given option is nil, we checked the instance one is set to the default
// value of the option.
func (o *RestrictIndicesOption) Equal(o2 *RestrictIndicesOption) bool {
	if o == nil {
		return o2 == nil || reflect.DeepEqual(o2.value, []string{})
	}
	if o2 == nil {
		return o == nil || reflect.DeepEqual(o.value, []string{})
	}
	return reflect.DeepEqual(o.value, o2.value)
}

// RestrictIndicesEqual returns true if the two options are equal.
// In case of one option being nil, the value of the other must be nil as well
// or be set to the default value of this option.
func RestrictIndicesEqual(o1, o2 *RestrictIndicesOption) bool {
	return o1.Equal(o2)
}
