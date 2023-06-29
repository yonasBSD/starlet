package internal

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	startime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

func TestIsEmptyString(t *testing.T) {
	if !IsEmptyString(starlark.String("")) {
		t.Error("empty string should equal true")
	}

	if IsEmptyString(".") {
		t.Error("non-empty string shouldn't be empty")
	}
}

func TestAsString(t *testing.T) {
	cases := []struct {
		in       starlark.Value
		got, err string
	}{
		{starlark.String("foo"), "foo", ""},
		{starlark.String("\"foo'"), "\"foo'", ""},
		{starlark.Bool(true), "", "invalid syntax"},
	}

	for i, c := range cases {
		got, err := asString(c.in)
		if !(err == nil && c.err == "" || err != nil && err.Error() == c.err) {
			t.Errorf("case %d error mismatch. expected: '%s', got: '%s'", i, c.err, err)
			continue
		}

		if c.got != got {
			t.Errorf("case %d. expected: '%s', got: '%s'", i, c.got, got)
		}
	}
}

func TestMarshal(t *testing.T) {
	expectedStringDict := starlark.NewDict(1)
	if err := expectedStringDict.SetKey(starlark.String("foo"), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}

	expectedIntDict := starlark.NewDict(1)
	if err := expectedIntDict.SetKey(starlark.MakeInt(42*2), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}

	ct, _ := (&customType{42}).MarshalStarlark()
	expectedStrDictCustomType := starlark.NewDict(2)
	if err := expectedStrDictCustomType.SetKey(starlark.String("foo"), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}
	if err := expectedStrDictCustomType.SetKey(starlark.String("bar"), ct); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		in   interface{}
		want starlark.Value
		err  string
	}{
		{nil, starlark.None, ""},
		{true, starlark.True, ""},
		{"foo", starlark.String("foo"), ""},
		{42, starlark.MakeInt(42), ""},
		{int8(42), starlark.MakeInt(42), ""},
		{int16(42), starlark.MakeInt(42), ""},
		{int32(42), starlark.MakeInt(42), ""},
		{int64(42), starlark.MakeInt(42), ""},
		{int64(1 << 42), starlark.MakeInt(1 << 42), ""},
		{uint(42), starlark.MakeUint(42), ""},
		{uint8(42), starlark.MakeUint(42), ""},
		{uint16(42), starlark.MakeUint(42), ""},
		{uint32(42), starlark.MakeUint(42), ""},
		{uint64(42), starlark.MakeUint64(42), ""},
		{uint64(1 << 42), starlark.MakeUint64(1 << 42), ""},
		{float32(42), starlark.Float(42), ""},
		{42., starlark.Float(42), ""},
		{time.Unix(1588540633, 0), startime.Time(time.Unix(1588540633, 0)), ""},
		{[]interface{}{42}, starlark.NewList([]starlark.Value{starlark.MakeInt(42)}), ""},
		{map[string]interface{}{"foo": 42}, expectedStringDict, ""},
		{map[interface{}]interface{}{"foo": 42}, expectedStringDict, ""},
		{map[interface{}]interface{}{42 * 2: 42}, expectedIntDict, ""},
		{&customType{42}, ct, ""},
		{map[string]interface{}{"foo": 42, "bar": &customType{42}}, expectedStrDictCustomType, ""},
		{map[interface{}]interface{}{"foo": 42, "bar": &customType{42}}, expectedStrDictCustomType, ""},
		{[]interface{}{42, &customType{42}}, starlark.NewList([]starlark.Value{starlark.MakeInt(42), ct}), ""},
		{&invalidCustomType{42}, starlark.None, "unrecognized type: &internal.invalidCustomType{Foo:42}"},
	}

	for i, c := range cases {
		got, err := Marshal(c.in)
		if !(err == nil && c.err == "" || err != nil && err.Error() == c.err) {
			t.Errorf("case %d. error mismatch. expected: %q, got: %q (%T -> %T)", i, c.err, err, c.in, c.want)
			continue
		}

		compareResult, err := starlark.Equal(c.want, got)
		if err != nil {
			t.Errorf("case %d. error comparing results: %q", i, err)
			continue
		}
		if !compareResult {
			t.Errorf("case %d. expected: %#v, got: %#v (%T -> %T)", i, c.want, got, c.in, c.want)
		}
	}
}

func TestUnmarshal(t *testing.T) {
	strDict := starlark.NewDict(1)
	if err := strDict.SetKey(starlark.String("foo"), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}

	intDict := starlark.NewDict(1)
	if err := intDict.SetKey(starlark.MakeInt(42*2), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}

	ct, _ := (&customType{42}).MarshalStarlark()
	strDictCT := starlark.NewDict(2)
	if err := strDictCT.SetKey(starlark.String("foo"), starlark.MakeInt(42)); err != nil {
		t.Fatal(err)
	}
	if err := strDictCT.SetKey(starlark.String("bar"), ct); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		in   starlark.Value
		want interface{}
		err  string
	}{
		{starlark.None, nil, ""},
		{starlark.True, true, ""},
		{starlark.String("foo"), "foo", ""},
		{starlark.MakeInt(42), 42, ""},
		{starlark.MakeInt(42), int8(42), ""},
		{starlark.MakeInt(42), int16(42), ""},
		{starlark.MakeInt(42), int32(42), ""},
		{starlark.MakeInt(42), int64(42), ""},
		{starlark.MakeInt(1 << 42), int64(1 << 42), ""},
		{starlark.MakeUint(42), uint(42), ""},
		{starlark.MakeUint(42), uint8(42), ""},
		{starlark.MakeUint(42), uint16(42), ""},
		{starlark.MakeUint(42), uint32(42), ""},
		{starlark.MakeUint64(42), uint64(42), ""},
		{starlark.MakeUint64(1 << 42), uint64(1 << 42), ""},
		{starlark.Float(42), float32(42), ""},
		{starlark.Float(42), 42., ""},
		{startime.Time(time.Unix(1588540633, 0)), time.Unix(1588540633, 0), ""},
		{starlark.NewList([]starlark.Value{starlark.MakeInt(42)}), []interface{}{42}, ""},
		{strDict, map[string]interface{}{"foo": 42}, ""},
		{intDict, map[interface{}]interface{}{42 * 2: 42}, ""},
		{ct, &customType{42}, ""},
		{strDictCT, map[string]interface{}{"foo": 42, "bar": &customType{42}}, ""},
		{starlark.NewList([]starlark.Value{starlark.MakeInt(42), ct}), []interface{}{42, &customType{42}}, ""},
		{starlark.Tuple{starlark.String("foo"), starlark.MakeInt(42)}, []interface{}{"foo", 42}, ""},
	}

	for i, c := range cases {
		got, err := Unmarshal(c.in)
		if !(err == nil && c.err == "" || err != nil && err.Error() == c.err) {
			t.Errorf("case %d. error mismatch. expected: %q, got: %q, %T -> %T", i, c.err, err, c.in, c.want)
			continue
		}

		// convert to the same type as expected
		var act interface{}
		act = got
		switch c.want.(type) {
		case int8:
			act = int8(got.(int))
		case int16:
			act = int16(got.(int))
		case int32:
			act = int32(got.(int))
		case int64:
			act = int64(got.(int))
		case uint:
			act = uint(got.(int))
		case uint8:
			act = uint8(got.(int))
		case uint16:
			act = uint16(got.(int))
		case uint32:
			act = uint32(got.(int))
		case uint64:
			act = uint64(got.(int))
		case float32:
			act = float32(got.(float64))
		}

		// compare
		if !reflect.DeepEqual(c.want, act) {
			t.Errorf("case %d. expected: %#v (%T), got: %#v (%T), %T -> %T", i, c.want, c.want, got, got, c.in, c.want)
		}
	}
}

type invalidCustomType struct {
	Foo int64
}

type customType invalidCustomType

var (
	_ Unmarshaler    = (*customType)(nil)
	_ Marshaler      = (*customType)(nil)
	_ starlark.Value = (*customType)(nil)
)

func (c *customType) CompareSameType(op syntax.Token, v starlark.Value, depth int) (bool, error) {
	if op != syntax.EQL {
		return false, fmt.Errorf("not expected operator %q", op)
	}
	other := v.(*customType)
	return c.Foo == other.Foo, nil
}

func (c *customType) UnmarshalStarlark(v starlark.Value) error {
	// asserts
	if v.Type() != "struct" {
		return fmt.Errorf("not expected top level type, want struct, got %q", v.Type())
	}
	if _, ok := v.(*starlarkstruct.Struct).Constructor().(*customType); !ok {
		return fmt.Errorf("not expected construct type got %T, want %T", v.(*starlarkstruct.Struct).Constructor(), c)
	}

	// TODO: refactoring transform data

	mustInt64 := func(sv starlark.Value) int64 {
		i, _ := sv.(starlark.Int).Int64()
		return i
	}

	data := starlark.StringDict{}
	v.(*starlarkstruct.Struct).ToStringDict(data)

	*c = customType{
		Foo: mustInt64(data["foo"]),
	}
	return nil
}

func (c *customType) MarshalStarlark() (starlark.Value, error) {
	v := starlarkstruct.FromStringDict(&customType{}, starlark.StringDict{
		"foo": starlark.MakeInt64(c.Foo),
	})
	return v, nil
}

func (c customType) String() string {
	return "customType"
}

func (c customType) Type() string { return "test.customType" }

func (customType) Freeze() {}

func (c customType) Truth() starlark.Bool {
	return starlark.True
}

func (c customType) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", c.Type())
}