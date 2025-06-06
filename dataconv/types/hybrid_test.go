package types

import (
	"errors"
	"math"
	"reflect"
	"runtime"
	"testing"

	"go.starlark.net/starlark"
)

func TestFloatOrInt_Unpack(t *testing.T) {
	var p *FloatOrInt
	if err := p.Unpack(nil); err == nil {
		t.Error("expected error on nil receiver")
	}

	tests := []struct {
		name    string
		v       starlark.Value
		wantNum FloatOrInt
		wantErr bool
	}{
		{
			name:    "int",
			v:       starlark.MakeInt(1),
			wantNum: 1,
		},
		{
			name:    "float",
			v:       starlark.Float(1.2),
			wantNum: 1.2,
		},
		{
			name:    "string",
			v:       starlark.String("1"),
			wantErr: true,
		},
		{
			name:    "none",
			v:       starlark.None,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p FloatOrInt
			if err := p.Unpack(tt.v); (err != nil) != tt.wantErr {
				t.Errorf("FloatOrInt.Unpack() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && p != tt.wantNum {
				t.Errorf("FloatOrInt.Unpack() got = %v, want %v", p, tt.wantNum)
			}
		})
	}
}

func TestOverflow(t *testing.T) {
	i := math.MaxInt64
	t.Logf("Int: %d", i)
	f := FloatOrInt(i)
	t.Logf("Float: %f", f)
	f2 := FloatOrInt(i) + 1
	t.Logf("Float+1: %f", f2)
	t.Logf("Float <=>: %t %t %t", f2 < f, f2 == f, f2 > f)

	t.Logf("GoInt: %d", f2.GoInt())
	t.Logf("GoInt32: %d", f2.GoInt32())
	t.Logf("GoInt64: %d", f2.GoInt64())
}

func TestFloatOrInt_Value(t *testing.T) {
	t.Logf("Testing on %s", runtime.GOARCH)
	t.Logf("MinInt: %d, MaxInt: %d", math.MinInt, math.MaxInt)
	t.Logf("MinInt32: %d, MaxInt32: %d", math.MinInt32, math.MaxInt32)
	t.Logf("MinInt64: %d, MaxInt64: %d", math.MinInt64, math.MaxInt64)
	t.Logf("MinFloat32: %f, MaxFloat32: %f", math.SmallestNonzeroFloat32, math.MaxFloat32)
	t.Logf("MinFloat64: %f, MaxFloat64: %f", math.SmallestNonzeroFloat64, math.MaxFloat64)

	tests := []struct {
		name      string
		v         FloatOrInt
		wantInt   int
		wantInt32 int32
		wantInt64 int64
		wantFlt   float64
	}{
		{
			name:      "zero",
			v:         0,
			wantInt:   0,
			wantInt32: 0,
			wantInt64: 0,
			wantFlt:   0,
		},
		{
			name:      "int",
			v:         1,
			wantInt:   1,
			wantInt32: 1,
			wantInt64: 1,
			wantFlt:   1,
		},
		{
			name:      "float",
			v:         1.2,
			wantInt:   1,
			wantInt32: 1,
			wantInt64: 1,
			wantFlt:   1.2,
		},
		{
			name:      "large",
			v:         1e12 + 1,
			wantInt:   1000000000001,
			wantInt32: 2147483647,
			wantInt64: 1000000000001,
			wantFlt:   1e12 + 1,
		},
		{
			name:      "underflow",
			v:         -1e12 - 1,
			wantInt:   -1000000000001,
			wantInt32: -2147483648,
			wantInt64: -1000000000001,
			wantFlt:   -1e12 - 1,
		},
		{
			name:      "int32_max",
			v:         FloatOrInt(math.MaxInt32),
			wantInt:   math.MaxInt32,
			wantInt32: math.MaxInt32,
			wantInt64: math.MaxInt32,
			wantFlt:   float64(math.MaxInt32),
		},
		{
			name:      "int32_overflow",
			v:         FloatOrInt(math.MaxInt32) + 1,
			wantInt:   int(math.MaxInt32) + 1,
			wantInt32: math.MaxInt32,
			wantInt64: int64(math.MaxInt32) + 1,
			wantFlt:   float64(math.MaxInt32) + 1,
		},
		{
			name:      "negative_int32_min",
			v:         FloatOrInt(math.MinInt32),
			wantInt:   math.MinInt32,
			wantInt32: math.MinInt32,
			wantInt64: int64(math.MinInt32),
			wantFlt:   float64(math.MinInt32),
		},
		{
			name:      "negative_int32_underflow",
			v:         FloatOrInt(math.MinInt32) - 1,
			wantInt:   int(math.MinInt32) - 1,
			wantInt32: math.MinInt32,
			wantInt64: int64(math.MinInt32) - 1,
			wantFlt:   float64(math.MinInt32) - 1,
		},
		//{
		//	name:      "int64_max",
		//	v:         FloatOrInt(math.MaxInt64),
		//	wantInt:   math.MaxInt,
		//	wantInt32: math.MaxInt32,
		//	wantInt64: math.MaxInt64,
		//	wantFlt:   float64(math.MaxInt64),
		//},
		//{
		//	name:      "int64_overflow",
		//	v:         FloatOrInt(math.MaxInt64) + 1000,
		//	wantInt:   math.MaxInt,
		//	wantInt32: math.MaxInt32,
		//	wantInt64: math.MaxInt64,
		//	wantFlt:   float64(math.MaxInt64) + 1000,
		//},
		{
			name:      "negative_int64_underflow",
			v:         FloatOrInt(math.MinInt64) - 1000,
			wantInt:   math.MinInt,
			wantInt32: math.MinInt32,
			wantInt64: math.MinInt64,
			wantFlt:   float64(math.MinInt64) - 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.GoInt(); got != tt.wantInt {
				t.Errorf("FloatOrInt.GoInt() = %v, want %v", got, tt.wantInt)
			}
			if got := tt.v.GoInt32(); got != tt.wantInt32 {
				t.Errorf("FloatOrInt.GoInt32() = %v, want %v", got, tt.wantInt32)
			}
			if got := tt.v.GoInt64(); got != tt.wantInt64 {
				t.Errorf("FloatOrInt.GoInt64() = %v, want %v", got, tt.wantInt64)
			}

			if got := tt.v.GoFloat(); got != tt.wantFlt {
				t.Errorf("FloatOrInt.GoFloat() = %v, want %v", got, tt.wantFlt)
			}
			if got := tt.v.GoFloat32(); got != float32(tt.wantFlt) {
				t.Errorf("FloatOrInt.GoFloat32() = %v, want %v", got, float32(tt.wantFlt))
			}
			if got := tt.v.GoFloat64(); got != tt.wantFlt {
				t.Errorf("FloatOrInt.GoFloat64() = %v, want %v", got, tt.wantFlt)
			}
		})
	}
}

func TestFloatOrIntList_Unpack(t *testing.T) {
	tests := []struct {
		name    string
		input   starlark.Value
		want    FloatOrIntList
		wantErr bool
	}{
		{
			name:    "nil",
			input:   nil,
			wantErr: true,
		},
		{
			name:  "empty list",
			input: starlark.NewList(nil),
			want:  FloatOrIntList{},
		},
		{
			name:  "valid list of ints",
			input: starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.MakeInt(2), starlark.MakeInt(3)}),
			want:  FloatOrIntList{1, 2, 3},
		},
		{
			name:  "valid list of floats",
			input: starlark.NewList([]starlark.Value{starlark.Float(1.1), starlark.Float(2.2), starlark.Float(3.3)}),
			want:  FloatOrIntList{1.1, 2.2, 3.3},
		},
		{
			name:  "mixed list of ints and floats",
			input: starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.Float(2.2), starlark.MakeInt(3)}),
			want:  FloatOrIntList{1, 2.2, 3},
		},
		{
			name:    "invalid input type",
			input:   starlark.String("not a list"),
			wantErr: true,
		},
		{
			name:    "list with invalid type",
			input:   starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.String("invalid"), starlark.MakeInt(3)}),
			wantErr: true,
		},
		{
			name: "invalid nested list",
			input: starlark.NewList([]starlark.Value{
				starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.MakeInt(2)}),
				starlark.NewList([]starlark.Value{starlark.Float(3.3), starlark.Float(4.4)}),
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FloatOrIntList
			err := got.Unpack(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("FloatOrIntList.Unpack() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FloatOrIntList.Unpack() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test for nil receiver
	t.Run("nil receiver", func(t *testing.T) {
		var l *FloatOrIntList
		err := l.Unpack(starlark.NewList(nil))
		if !errors.Is(err, errNilReceiver) {
			t.Errorf("FloatOrIntList.Unpack() error = %v, want %v", err, errNilReceiver)
		}
	})
}

func TestFloatOrIntList_GoSlice(t *testing.T) {
	tests := []struct {
		name string
		l    FloatOrIntList
		want []float64
	}{
		{
			name: "non-empty list",
			l:    FloatOrIntList{1.1, 2.2, 3.3},
			want: []float64{1.1, 2.2, 3.3},
		},
		{
			name: "empty list",
			l:    FloatOrIntList{},
			want: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.GoSlice(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FloatOrIntList.GoSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloatOrIntList_StarlarkList(t *testing.T) {
	tests := []struct {
		name string
		l    FloatOrIntList
		want *starlark.List
	}{
		{
			name: "non-empty list",
			l:    FloatOrIntList{1.1, 2.2, 3.3},
			want: starlark.NewList([]starlark.Value{starlark.Float(1.1), starlark.Float(2.2), starlark.Float(3.3)}),
		},
		{
			name: "empty list",
			l:    FloatOrIntList{},
			want: starlark.NewList(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.l.StarlarkList()
			if got.Len() != tt.want.Len() {
				t.Errorf("FloatOrIntList.StarlarkList() length = %v, want %v", got.Len(), tt.want.Len())
			}
			for i := 0; i < got.Len(); i++ {
				gotItem, _ := got.Index(i).(starlark.Float)
				wantItem, _ := tt.want.Index(i).(starlark.Float)
				if gotItem != wantItem {
					t.Errorf("FloatOrIntList.StarlarkList() item at index %d = %v, want %v", i, gotItem, wantItem)
				}
			}
		})
	}
}

func TestFloatOrIntList_Len(t *testing.T) {
	tests := []struct {
		name string
		l    FloatOrIntList
		want int
	}{
		{
			name: "non-empty list",
			l:    FloatOrIntList{1.1, 2.2, 3.3},
			want: 3,
		},
		{
			name: "empty list",
			l:    FloatOrIntList{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Len(); got != tt.want {
				t.Errorf("FloatOrIntList.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloatOrIntList_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		l    FloatOrIntList
		want bool
	}{
		{
			name: "non-empty list",
			l:    FloatOrIntList{1.1, 2.2, 3.3},
			want: false,
		},
		{
			name: "empty list",
			l:    FloatOrIntList{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.IsEmpty(); got != tt.want {
				t.Errorf("FloatOrIntList.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumericValue(t *testing.T) {
	integer := func(n int) starlark.Value { return starlark.MakeInt(n) }
	double := func(n float64) starlark.Value { return starlark.Float(n) }
	tests := []struct {
		name    string
		values  []starlark.Value
		wantVal starlark.Value
		wantErr bool
	}{
		{
			name:    "empty",
			values:  []starlark.Value{},
			wantVal: integer(0),
		},
		{
			name:    "single int",
			values:  []starlark.Value{integer(100)},
			wantVal: integer(100),
		},
		{
			name:    "single float",
			values:  []starlark.Value{double(2)},
			wantVal: double(2),
		},
		{
			name:    "int and float",
			values:  []starlark.Value{integer(100), double(2)},
			wantVal: double(102),
		},
		{
			name:    "float and int",
			values:  []starlark.Value{double(4), integer(100)},
			wantVal: double(104),
		},
		{
			name:    "string",
			values:  []starlark.Value{starlark.String("100")},
			wantErr: true,
		},
		{
			name:    "int and string",
			values:  []starlark.Value{integer(100), starlark.String("2")},
			wantVal: integer(100),
			wantErr: true,
		},
		{
			name:    "string and int",
			values:  []starlark.Value{starlark.String("2"), integer(100)},
			wantVal: integer(0),
			wantErr: true,
		},
		{
			name:    "float and string",
			values:  []starlark.Value{double(4), starlark.String("2")},
			wantVal: double(4),
			wantErr: true,
		},
		{
			name:    "string and float",
			values:  []starlark.Value{starlark.String("2"), double(4)},
			wantVal: integer(0),
			wantErr: true,
		},
		{
			name:    "more int",
			values:  []starlark.Value{integer(100), integer(2), integer(3)},
			wantVal: integer(105),
		},
		{
			name:    "more float",
			values:  []starlark.Value{double(4), double(2), double(3)},
			wantVal: double(9),
		},
		{
			name:    "int and nil",
			values:  []starlark.Value{integer(100), nil},
			wantVal: integer(100),
		},
		{
			name:    "float and None",
			values:  []starlark.Value{double(6), starlark.None},
			wantVal: double(6),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNumericValue()
			var err error
			for _, v := range tt.values {
				if err = n.Add(v); err != nil {
					break
				}
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
			}
			gotVal := n.Value()
			switch expVal := tt.wantVal.(type) {
			case starlark.Int:
				if actVal, ok := gotVal.(starlark.Int); !ok || actVal != expVal {
					t.Errorf("Add() gotVal = %v, want int %v", gotVal, tt.wantVal)
				}
			case starlark.Float:
				if actVal, ok := gotVal.(starlark.Float); !ok || actVal != expVal {
					t.Errorf("Add() gotVal = %v, want float %v", gotVal, tt.wantVal)
				}
			}
		})
	}
}

func TestNumericValue_Unpack(t *testing.T) {
	var p *NumericValue
	if err := p.Unpack(nil); err == nil {
		t.Error("expected error on nil receiver")
	}

	tests := []struct {
		name    string
		v       starlark.Value
		wantInt starlark.Int
		wantFlt float64
		hasFlt  bool
		wantErr bool
	}{
		{
			name:    "int",
			v:       starlark.MakeInt(42),
			wantInt: starlark.MakeInt(42),
			hasFlt:  false,
		},
		{
			name:    "float",
			v:       starlark.Float(3.14),
			wantFlt: 3.14,
			hasFlt:  true,
		},
		{
			name:    "none error",
			v:       starlark.None,
			wantErr: true,
		},
		{
			name:    "string error",
			v:       starlark.String("not a number"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNumericValue()
			err := n.Unpack(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("NumericValue.Unpack() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if tt.hasFlt {
					if float64(n.floatValue) != tt.wantFlt {
						t.Errorf("NumericValue.Unpack() got = %v, want %v", n.floatValue, tt.wantFlt)
					}
					if n.hasFloat != tt.hasFlt {
						t.Errorf("NumericValue.Unpack() got hasFloat = %v, want %v", n.hasFloat, tt.hasFlt)
					}
				} else {
					if n.intValue != tt.wantInt {
						t.Errorf("NumericValue.Unpack() got = %v, want %v", n.intValue, tt.wantInt)
					}
				}
			}
		})
	}
}

func TestNumericValue_Value(t *testing.T) {
	tests := []struct {
		name    string
		n       *NumericValue
		wantVal starlark.Value
	}{
		{
			name:    "int value",
			n:       &NumericValue{intValue: starlark.MakeInt(42)},
			wantVal: starlark.MakeInt(42),
		},
		{
			name:    "float value",
			n:       &NumericValue{floatValue: starlark.Float(3.14), hasFloat: true},
			wantVal: starlark.Float(3.14),
		},
		{
			name:    "int and float value",
			n:       &NumericValue{intValue: starlark.MakeInt(100), floatValue: starlark.Float(3.14), hasFloat: true},
			wantVal: starlark.Float(103.14),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotVal := tt.n.Value(); gotVal != tt.wantVal {
				t.Errorf("NumericValue.Value() = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

func TestStringOrBytes_Unpack(t *testing.T) {
	var p *StringOrBytes
	if err := p.Unpack(nil); err == nil {
		t.Error("expected error on nil receiver")
	}

	tests := []struct {
		name    string
		v       starlark.Value
		wantStr StringOrBytes
		wantErr bool
	}{
		{
			name:    "string",
			v:       starlark.String("foo"),
			wantStr: "foo",
		},
		{
			name:    "bytes",
			v:       starlark.Bytes("foo"),
			wantStr: "foo",
		},
		{
			name:    "int",
			v:       starlark.MakeInt(1),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p StringOrBytes
			if err := p.Unpack(tt.v); (err != nil) != tt.wantErr {
				t.Errorf("StringOrBytes.Unpack() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && p != tt.wantStr {
				t.Errorf("StringOrBytes.Unpack() got = %v, want %v", p, tt.wantStr)
			}
		})
	}
}

func TestStringOrBytes_Stringer(t *testing.T) {
	tests := []struct {
		name     string
		v        StringOrBytes
		wantGo   string
		wantStar starlark.String
		empty    bool
	}{
		{
			name:     "empty",
			v:        "",
			wantGo:   "",
			wantStar: starlark.String(""),
			empty:    true,
		},
		{
			name:     "string",
			v:        "foo",
			wantGo:   "foo",
			wantStar: starlark.String("foo"),
		},
		{
			name:     "bytes",
			v:        "bar",
			wantGo:   "bar",
			wantStar: starlark.String("bar"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.GoString(); got != tt.wantGo {
				t.Errorf("StringOrBytes.GoString() = %v, want %v", got, tt.wantGo)
			}
			if got := tt.v.GoBytes(); string(got) != tt.wantGo {
				t.Errorf("StringOrBytes.GoBytes() = %v, want %v", got, []byte(tt.wantGo))
			}
			if got := tt.v.StarlarkString(); got != tt.wantStar {
				t.Errorf("StringOrBytes.StarlarkString() = %v, want %v", got, tt.wantStar)
			}
			if got := tt.v.StarlarkBytes(); got != starlark.Bytes(tt.wantStar) {
				t.Errorf("StringOrBytes.StarlarkBytes() = %v, want %v", got, starlark.Bytes(tt.wantStar))
			}
			if got := tt.v.IsEmpty(); got != tt.empty {
				t.Errorf("StringOrBytes.IsEmpty() = %v, want %v", got, tt.empty)
			}
		})
	}
}

func TestNullableStringOrBytes_Unpack(t *testing.T) {
	var p *NullableStringOrBytes
	if err := p.Unpack(nil); err == nil {
		t.Error("expected error on nil receiver")
	}

	tests := []struct {
		name    string
		v       starlark.Value
		wantStr string
		wantErr bool
	}{
		{
			name:    "string",
			v:       starlark.String("foo"),
			wantStr: "foo",
		},
		{
			name:    "bytes",
			v:       starlark.Bytes("bar"),
			wantStr: "bar",
		},
		{
			name:    "none",
			v:       starlark.None,
			wantStr: "",
		},
		{
			name:    "int",
			v:       starlark.MakeInt(1),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p NullableStringOrBytes
			if err := p.Unpack(tt.v); (err != nil) != tt.wantErr {
				t.Errorf("NullableStringOrBytes.Unpack() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && p.GoString() != tt.wantStr {
				t.Errorf("NullableStringOrBytes.Unpack() got = %v, want %v", p.GoString(), tt.wantStr)
			}
		})
	}
}

func TestNullableStringOrBytes_Methods(t *testing.T) {
	tests := []struct {
		name        string
		str         *NullableStringOrBytes
		wantGoStr   string
		wantGoBytes []byte
		wantStarStr starlark.String
		wantIsNull  bool
		wantIsEmpty bool
	}{
		{
			name:        "nil",
			str:         nil,
			wantGoStr:   "",
			wantGoBytes: nil,
			wantStarStr: starlark.String(""),
			wantIsNull:  true,
			wantIsEmpty: true,
		},
		{
			name:        "nil value",
			str:         &NullableStringOrBytes{},
			wantGoStr:   "",
			wantGoBytes: nil,
			wantStarStr: starlark.String(""),
			wantIsNull:  true,
			wantIsEmpty: true,
		},
		{
			name:        "no default",
			str:         NewNullableStringOrBytesNoDefault(),
			wantGoStr:   "",
			wantGoBytes: nil,
			wantStarStr: starlark.String(""),
			wantIsNull:  true,
			wantIsEmpty: true,
		},
		{
			name:        "empty string",
			str:         NewNullableStringOrBytes(""),
			wantGoStr:   "",
			wantGoBytes: []byte{},
			wantStarStr: starlark.String(""),
			wantIsNull:  false,
			wantIsEmpty: true,
		},
		{
			name:        "non-empty string",
			str:         NewNullableStringOrBytes("hello"),
			wantGoStr:   "hello",
			wantGoBytes: []byte("hello"),
			wantStarStr: starlark.String("hello"),
			wantIsNull:  false,
			wantIsEmpty: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotStr := tt.str.GoString(); gotStr != tt.wantGoStr {
				t.Errorf("NullableStringOrBytes.GoString() = %v, want %v", gotStr, tt.wantGoStr)
			}
			if gotBytes := tt.str.GoBytes(); !reflect.DeepEqual(gotBytes, tt.wantGoBytes) {
				t.Errorf("NullableStringOrBytes.GoBytes() = %v, want %v", gotBytes, tt.wantGoBytes)
			}
			if gotStr := tt.str.StarlarkString(); gotStr != tt.wantStarStr {
				t.Errorf("NullableStringOrBytes.StarlarkString() = %v, want %v", gotStr, tt.wantStarStr)
			}
			if gotStr := tt.str.StarlarkBytes(); gotStr != starlark.Bytes(tt.wantStarStr) {
				t.Errorf("NullableStringOrBytes.StarlarkBytes() = %v, want %v", gotStr, starlark.Bytes(tt.wantStarStr))
			}
			if gotIsNull := tt.str.IsNull(); gotIsNull != tt.wantIsNull {
				t.Errorf("NullableStringOrBytes.IsNull() = %v, want %v", gotIsNull, tt.wantIsNull)
			}
			if gotIsEmpty := tt.str.IsNullOrEmpty(); gotIsEmpty != tt.wantIsEmpty {
				t.Errorf("NullableStringOrBytes.IsNullOrEmpty() = %v, want %v", gotIsEmpty, tt.wantIsEmpty)
			}
		})
	}
}
