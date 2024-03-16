package fsevent

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/type/latlng"
)

func setReflect(v reflect.Value, field map[string]interface{}, tag string) error {
	switch v.Type() {
	case reflect.TypeOf(time.Time{}):
		t, err := reflectTimestamp(field, tag)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(*t))
		return nil
	case reflect.TypeOf([]byte{}):
		b, err := reflectBytes(field, tag)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(b))
		return nil
	case reflect.TypeOf(latlng.LatLng{}):
		return fmt.Errorf("fsevent: LatLng must be pointer")
	case reflect.TypeOf(&latlng.LatLng{}):
		ll, err := reflectGeo(field, tag)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(ll))
		return nil
	}

	switch v.Kind() {
	case reflect.Bool:
		bv := field["booleanValue"]
		if bv == nil {
			return nil
		}
		fv, ok := bv.(bool)
		if !ok {
			return fmt.Errorf("fsevent: %s is not bool", tag)
		}
		v.SetBool(fv)
	case reflect.Int64:
		iv := field["integerValue"]
		if iv == nil {
			return nil
		}
		fv, ok := iv.(string)
		if !ok {
			return fmt.Errorf("fsevent: %s is not int string", tag)
		}
		ifv, err := strconv.ParseInt(fv, 10, 64)
		if err != nil {
			return fmt.Errorf("fsevent: failed to parse int64 on %s. %v", tag, err)
		}
		v.SetInt(ifv)
	case reflect.Float64:
		dv := field["doubleValue"]
		iv := field["integerValue"]
		var fv float64
		var ok bool
		if dv != nil {
			fv, ok = dv.(float64)
		} else if iv != nil {
			fv, ok = iv.(float64)
		} else {
			return nil
		}
		if !ok {
			return fmt.Errorf("fsevent: %s is not float64", tag)
		}
		v.SetFloat(fv)
	case reflect.String:
		sv := field["stringValue"]
		// TODO: support reflect to firestore.DocumentRef
		// for now reflect to string
		rv := field["referenceValue"]
		var fv string
		var ok bool
		if sv != nil {
			fv, ok = sv.(string)
		} else if rv != nil {
			fv, ok = rv.(string)
		} else {
			return nil
		}
		if !ok {
			return fmt.Errorf("fsevent: %s is not string", tag)
		}
		v.SetString(fv)
	case reflect.Slice:
		vs, err := reflectArray(field, tag)
		if err != nil {
			return err
		}
		vslen := len(vs)
		vlen := v.Len()
		switch {
		case vlen < vslen:
			v.Set(reflect.MakeSlice(v.Type(), vslen, vslen))
		case vlen > vslen:
			v.SetLen(vslen)
		}
		for i := 0; i < vslen; i++ {
			f, ok := vs[i].(map[string]interface{})
			if !ok {
				return fmt.Errorf("fsevent: %s is not array", tag)
			}
			if err := setReflect(v.Index(i), f, tag); err != nil {
				return err
			}
		}
	case reflect.Array:
		vs, err := reflectArray(field, tag)
		if err != nil {
			return err
		}
		vslen := len(vs)
		vlen := v.Len()
		if vlen > vslen {
			for i := vslen; i < vlen; i++ {
				v.Index(i).Set(reflect.Zero(v.Type().Elem()))
			}
		}
		for i := 0; i < vslen; i++ {
			f, ok := vs[i].(map[string]interface{})
			if !ok {
				return fmt.Errorf("fsevent: %s is not array", tag)
			}
			if err := setReflect(v.Index(i), f, tag); err != nil {
				return err
			}
		}
	case reflect.Struct:
		fsm, err := reflectMap(field, tag)
		if err != nil {
			return err
		}
		vt := v.Type()
		for i := 0; i < vt.NumField(); i++ {
			f := vt.Field(i)
			if f.PkgPath != "" {
				return nil
			}
			ftag := f.Tag.Get("firestore")
			parts := strings.Split(ftag, ",")
			ftag = parts[0]
			if fsm[ftag] == nil {
				// skip fields that have no value
				continue
			}
			fm, ok := fsm[ftag].(map[string]interface{})
			if !ok {
				return fmt.Errorf("fsevent: %s is not map", tag)
			}
			if err := setReflect(v.Field(i), fm, ftag); err != nil {
				return err
			}
		}
	case reflect.Map:
		fsm, err := reflectMap(field, tag)
		if err != nil {
			return err
		}
		vt := v.Type()
		if vt.Key().Kind() != reflect.String {
			return fmt.Errorf("fsevent: %s map key type is not string", tag)
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(vt))
		}
		evt := vt.Elem()
		for k, fv := range fsm {
			fm, ok := fv.(map[string]interface{})
			if !ok {
				return fmt.Errorf("fsevent: %s is not map", tag)
			}
			ne := reflect.New(evt).Elem()
			if err := setReflect(ne, fm, tag); err != nil {
				return err
			}
			v.SetMapIndex(reflect.ValueOf(k), ne)
		}
	case reflect.Ptr:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setReflect(v.Elem(), field, tag)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			if !v.IsNil() && v.Elem().Kind() == reflect.Ptr {
				return setReflect(v.Elem(), field, tag)
			}
			iv, err := reflectInterface(field, tag)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(iv))
			return nil
		}
		fallthrough
	default:
		return fmt.Errorf("fsevent: tag:%s type %v is not supported", tag, v.Type())
	}
	return nil
}

func reflectTimestamp(field map[string]interface{}, tag string) (*time.Time, error) {
	tv := field["timestampValue"]
	if tv == nil {
		return nil, nil
	}
	ts, ok := tv.(string)
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not timestamp string", tag)
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmt.Errorf("fsevent: failed to parse time on %s. %v", tag, err)
	}
	return &t, nil
}

func reflectBytes(field map[string]interface{}, tag string) ([]byte, error) {
	bv := field["bytesValue"]
	if bv == nil {
		return nil, nil
	}
	bs, ok := bv.(string)
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not bytes string", tag)
	}
	b, err := base64.StdEncoding.DecodeString(bs)
	if err != nil {
		return nil, fmt.Errorf("fsevent: failed to decode bytes string on %s. %v", tag, err)
	}
	return b, nil
}

func reflectGeo(field map[string]interface{}, tag string) (*latlng.LatLng, error) {
	geo := field["geoPointValue"]
	if geo == nil {
		return nil, nil
	}
	gm, ok := geo.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not geoPoint map", tag)
	}
	lat, ok := gm["latitude"].(float64)
	if !ok {
		return nil, fmt.Errorf("fsevent: %s.latitude is not float64", tag)
	}
	lng, ok := gm["longitude"].(float64)
	if !ok {
		return nil, fmt.Errorf("fsevent: %s.longitude is not float64", tag)
	}
	ll := latlng.LatLng{
		Latitude:  lat,
		Longitude: lng,
	}
	return &ll, nil
}

func reflectArray(field map[string]interface{}, tag string) ([]interface{}, error) {
	am, ok := field["arrayValue"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not array", tag)
	}
	iv := am["values"]
	if iv == nil {
		return nil, nil
	}
	vs, ok := iv.([]interface{})
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not array", tag)
	}
	return vs, nil
}

func reflectMap(field map[string]interface{}, tag string) (map[string]interface{}, error) {
	mv, ok := field["mapValue"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not map", tag)
	}
	fs := mv["fields"]
	if fs == nil {
		return nil, nil
	}
	fsm, ok := fs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fsevent: %s is not map", tag)
	}
	return fsm, nil
}

func reflectInterface(field map[string]interface{}, tag string) (interface{}, error) {
	if field["nullValue"] != nil {
		return nil, nil
	}
	if field["booleanValue"] != nil {
		return field["booleanValue"], nil
	}
	if field["integerValue"] != nil {
		fv, ok := field["integerValue"].(string)
		if !ok {
			return nil, fmt.Errorf("fsevent: %s is not int string", tag)
		}
		ifv, err := strconv.Atoi(fv)
		if err != nil {
			return nil, fmt.Errorf("fsevent: failed to atoi on %s. %v", tag, err)
		}
		return ifv, nil
	}
	if field["doubleValue"] != nil {
		return field["doubleValue"], nil
	}
	if field["timestampValue"] != nil {
		return reflectTimestamp(field, tag)
	}
	if field["stringValue"] != nil {
		return field["stringValue"], nil
	}
	if field["bytesValue"] != nil {
		return reflectBytes(field, tag)
	}
	if field["referenceValue"] != nil {
		// TODO: support reflect to firestore.DocumentRef
		// for now reflect to string
		return field["referenceValue"], nil
	}
	if field["geoPointValue"] != nil {
		return reflectGeo(field, tag)
	}
	if field["arrayValue"] != nil {
		fv, err := reflectArray(field, tag)
		if err != nil {
			return nil, err
		}
		av := make([]interface{}, len(fv))
		for i, v := range fv {
			mv, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("fsevent: %s is not array", tag)
			}
			iv, err := reflectInterface(mv, tag)
			if err != nil {
				return nil, err
			}
			av[i] = iv
		}
		return av, nil
	}
	if field["mapValue"] != nil {
		fsm, err := reflectMap(field, tag)
		if err != nil {
			return nil, err
		}
		mp := make(map[string]interface{}, len(fsm))
		for k, v := range fsm {
			mv, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("fsevent: %s is not map", tag)
			}
			iv, err := reflectInterface(mv, tag)
			if err != nil {
				return nil, err
			}
			mp[k] = iv
		}
		return mp, nil
	}
	return nil, fmt.Errorf("fsevent: unknown value type %+v", field)
}
