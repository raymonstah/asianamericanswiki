package fsevent

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Value holds Firestore fields.
type Value struct {
	CreateTime time.Time                         `json:"createTime"`
	Fields     map[string]map[string]interface{} `json:"fields"`
	Name       string                            `json:"name"`
	UpdateTime time.Time                         `json:"updateTime"`
}

// DataTo uses the document's fields to populate p, which can be a pointer to a
// map[string]interface{} or a pointer to a struct.
func (v *Value) DataTo(p interface{}) error {
	rv := reflect.ValueOf(p)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("fsevent: nil or not a pointer")
	}
	crv := rv.Elem()
	switch crv.Kind() {
	case reflect.Map:
		// TODO: process for map
	case reflect.Struct:
		rt := crv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.PkgPath != "" {
				// skip unexported field
				continue
			}
			tag := f.Tag.Get("firestore")
			parts := strings.Split(tag, ",")
			tag = parts[0]
			if v.Fields[tag] == nil {
				// skip fields that have no value
				continue
			}
			if err := setReflect(crv.Field(i), v.Fields[tag], tag); err != nil {
				return err
			}
		}
	}
	return nil
}
