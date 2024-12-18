// Package base64 defines base64 encoding & decoding functions for Starlark.
//
// Migrated from: https://github.com/qri-io/starlib/tree/master/encoding/base64
package base64

import (
	gobase64 "encoding/base64"
	"fmt"
	"sync"

	tps "github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the expected name for this Module when used in starlark's load() function, eg: load('base64', 'encode')
const ModuleName = "base64"

var (
	once         sync.Once
	base64Module starlark.StringDict
)

// Encodings is a map of strings to encoding formats. It is used to select the encoding format for the base64 module.
// You can add your own encoding formats to this map.
var Encodings = map[string]*gobase64.Encoding{
	// StdEncoding is the standard base64 encoding, as defined in RFC 4648.
	"standard": gobase64.StdEncoding,
	// RawStdEncoding is the standard raw, unpadded base64 encoding,
	// as defined in RFC 4648 section 3.2.
	// This is the same as StdEncoding but omits padding characters.
	"standard_raw": gobase64.RawStdEncoding,
	// URLEncoding is the alternate base64 encoding defined in RFC 4648.
	// It is typically used in URLs and file names.
	"url": gobase64.URLEncoding,
	// RawURLEncoding is the unpadded alternate base64 encoding defined in RFC 4648.
	// It is typically used in URLs and file names.
	// This is the same as URLEncoding but omits padding characters.
	"url_raw": gobase64.RawURLEncoding,
}

// LoadModule loads the base64 module.
// It is concurrency-safe and idempotent.
func LoadModule() (starlark.StringDict, error) {
	once.Do(func() {
		base64Module = starlark.StringDict{
			"base64": &starlarkstruct.Module{
				Name: "base64",
				Members: starlark.StringDict{
					"encode": starlark.NewBuiltin("base64.encode", encodeString),
					"decode": starlark.NewBuiltin("base64.decode", decodeString),
				},
			},
		}
	})
	return base64Module, nil
}

func selectEncoder(encoding starlark.String) (encoder *gobase64.Encoding, err error) {
	if encoding == "" {
		encoding = "standard"
	}
	encoder = Encodings[string(encoding)]
	if encoder == nil {
		err = fmt.Errorf("unsupported encoding format: %s", encoding)
	}
	return
}

func encodeString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data     tps.StringOrBytes
		encoding starlark.String
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "data", &data, "encoding?", &encoding); err != nil {
		return starlark.None, err
	}

	encoder, err := selectEncoder(encoding)
	if err != nil {
		return starlark.None, err
	}

	enc := encoder.EncodeToString([]byte(data))
	return starlark.String(enc), nil
}

func decodeString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data     tps.StringOrBytes
		encoding starlark.String
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "data", &data, "encoding?", &encoding); err != nil {
		return starlark.None, err
	}

	encoder, err := selectEncoder(encoding)
	if err != nil {
		return starlark.None, err
	}

	dec, err := encoder.DecodeString(string(data))
	if err != nil {
		return starlark.None, err
	}
	return starlark.String(dec), nil
}
