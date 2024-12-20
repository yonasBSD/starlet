package dataconv

// Based on https://github.com/qri-io/starlib/tree/master/util with some modifications and additions

import (
	"errors"
	"fmt"
	"time"

	"github.com/1set/starlight/convert"
	startime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Marshal converts Go values into Starlark types, like ToValue() of package starlight does.
// It only supports common Go types, won't wrap any custom types like Starlight does.
func Marshal(data interface{}) (v starlark.Value, err error) {
	switch x := data.(type) {
	case nil:
		v = starlark.None
	case bool:
		v = starlark.Bool(x)
	case string:
		v = starlark.String(x)
	case int:
		v = starlark.MakeInt(x)
	case int8:
		v = starlark.MakeInt(int(x))
	case int16:
		v = starlark.MakeInt(int(x))
	case int32:
		v = starlark.MakeInt(int(x))
	case int64:
		v = starlark.MakeInt64(x)
	case uint:
		v = starlark.MakeUint(x)
	case uint8:
		v = starlark.MakeUint(uint(x))
	case uint16:
		v = starlark.MakeUint(uint(x))
	case uint32:
		v = starlark.MakeUint(uint(x))
	case uint64:
		v = starlark.MakeUint64(x)
	case float32:
		v = starlark.Float(x)
	case float64:
		v = starlark.Float(x)
	case time.Time:
		v = startime.Time(x)
	case []interface{}:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []string:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i] = starlark.String(val)
		}
		v = starlark.NewList(elems)
	case []byte:
		v = starlark.Bytes(x)
	case map[interface{}]interface{}:
		dict := &starlark.Dict{}
		var elem starlark.Value
		for ki, val := range x {
			var key starlark.Value
			key, err = Marshal(ki)
			if err != nil {
				return
			}

			elem, err = Marshal(val)
			if err != nil {
				return
			}
			if err = dict.SetKey(key, elem); err != nil {
				return
			}
		}
		v = dict
	case map[string]interface{}:
		dict := &starlark.Dict{}
		var elem starlark.Value
		for key, val := range x {
			elem, err = Marshal(val)
			if err != nil {
				return
			}
			if err = dict.SetKey(starlark.String(key), elem); err != nil {
				return
			}
		}
		v = dict
	case map[string]string:
		dict := &starlark.Dict{}
		for key, val := range x {
			if err = dict.SetKey(starlark.String(key), starlark.String(val)); err != nil {
				return
			}
		}
		v = dict
	case Marshaler:
		v, err = x.MarshalStarlark()
	default:
		return starlark.None, fmt.Errorf("unrecognized type: %#v", x)
	}
	return
}

// Unmarshal converts a starlark.Value into it's Golang counterpart, like FromValue() of package starlight does.
// It's the opposite of Marshal().
func Unmarshal(x starlark.Value) (val interface{}, err error) {
	iterAttrs := func(v starlark.HasAttrs) (map[string]interface{}, error) {
		jo := make(map[string]interface{})
		for _, name := range v.AttrNames() {
			sv, err := v.Attr(name)
			if err != nil {
				return nil, err
			}
			jo[name], err = Unmarshal(sv)
			if err != nil {
				return nil, err
			}
		}
		return jo, nil
	}

	// for typed nil or nil
	if IsInterfaceNil(x) {
		if x == nil {
			return nil, errors.New("nil value")
		}
		return nil, fmt.Errorf("typed nil value: %T", x)
	}

	// switch on the type of the value (common types)
	switch v := x.(type) {
	case starlark.NoneType:
		val = nil
	case starlark.Bool:
		val = v.Truth() == starlark.True
	case starlark.Int:
		var tmp int
		err = starlark.AsInt(x, &tmp)
		val = tmp
	case starlark.Float:
		if f, ok := starlark.AsFloat(x); !ok {
			err = fmt.Errorf("couldn't parse float")
		} else {
			val = f
		}
	case starlark.String:
		val = v.GoString()
	case starlark.Bytes:
		val = string(v)
	case startime.Time:
		val = time.Time(v)
	case *starlark.Dict:
		var (
			dictVal starlark.Value
			pval    interface{}
			kval    interface{}
			keys    []interface{}
			vals    []interface{}
			// use interface{} as key type if found one key is not a string
			keyIf bool
		)
		for _, k := range v.Keys() {
			dictVal, _, err = v.Get(k)
			if err != nil {
				return
			}

			// check for cyclic reference
			if dictVal == x {
				err = fmt.Errorf("cyclic reference found")
				return
			}

			pval, err = Unmarshal(dictVal)
			if err != nil {
				err = fmt.Errorf("unmarshaling starlark value: %w", err)
				return
			}

			kval, err = Unmarshal(k)
			if err != nil {
				err = fmt.Errorf("unmarshaling starlark key: %w", err)
				return
			}

			if _, ok := kval.(string); !ok {
				// found key as not a string
				keyIf = true
			}

			keys = append(keys, kval)
			vals = append(vals, pval)
		}

		// prepare result
		rs := map[string]interface{}{}
		ri := map[interface{}]interface{}{}
		for i, key := range keys {
			// key as interface
			if keyIf {
				ri[key] = vals[i]
			} else {
				rs[key.(string)] = vals[i]
			}
		}

		if keyIf {
			val = ri // map[interface{}]interface{}
		} else {
			val = rs // map[string]interface{}
		}
	case *starlark.List:
		var (
			i       int
			listVal starlark.Value
			iter    = v.Iterate()
			value   = make([]interface{}, v.Len())
		)

		defer iter.Done()
		for iter.Next(&listVal) {
			if listVal == x {
				err = fmt.Errorf("cyclic reference found")
				return
			}
			value[i], err = Unmarshal(listVal)
			if err != nil {
				return
			}
			i++
		}
		val = value
	case starlark.Tuple:
		var (
			i        int
			tupleVal starlark.Value
			iter     = v.Iterate()
			value    = make([]interface{}, v.Len())
		)

		defer iter.Done()
		for iter.Next(&tupleVal) {
			value[i], err = Unmarshal(tupleVal)
			if err != nil {
				return
			}
			i++
		}
		val = value
	case *starlark.Set:
		var (
			i      int
			setVal starlark.Value
			iter   = v.Iterate()
			value  = make([]interface{}, v.Len())
		)

		defer iter.Done()
		for iter.Next(&setVal) {
			value[i], err = Unmarshal(setVal)
			if err != nil {
				return
			}
			i++
		}
		val = value
	case *starlarkstruct.Struct:
		if _var, ok := v.Constructor().(Unmarshaler); ok {
			err = _var.UnmarshalStarlark(x)
			if err != nil {
				err = fmt.Errorf("failed to marshal %T to Starlark object: %w", v.Constructor(), err)
				return
			}
			val = _var
		} else {
			am, err := iterAttrs(v)
			if err != nil {
				return nil, err
			}
			val = am
		}
	case *starlarkstruct.Module:
		am, err := iterAttrs(v)
		if err != nil {
			return nil, err
		}
		val = am
	case *convert.GoSlice:
		val = v.Value().Interface()
	case *convert.GoMap:
		val = v.Value().Interface()
	case *convert.GoStruct:
		val = v.Value().Interface()
	case *convert.GoInterface:
		val = v.Value().Interface()
	default:
		err = fmt.Errorf("unrecognized starlark type: %T", x)
	}
	return
}
