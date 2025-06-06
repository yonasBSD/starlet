// Package string defines functions that manipulate strings, it's intended to be a drop-in subset of Python's string module for Starlark.
// See https://docs.python.org/3/library/string.html and https://github.com/python/cpython/blob/main/Lib/string.py for reference.
package string

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the expected name for this Module when used
// in starlark's load() function, eg: load('string', 'length')
const ModuleName = "string"

var (
	once      sync.Once
	strModule starlark.StringDict
)

const (
	whitespace     = " \t\n\r\v\f"
	asciiLowerCase = `abcdefghijklmnopqrstuvwxyz`
	asciiUpperCase = `ABCDEFGHIJKLMNOPQRSTUVWXYZ`
	asciiLetters   = asciiLowerCase + asciiUpperCase
	decDigits      = `0123456789`
	hexDigits      = decDigits + `abcdefABCDEF`
	octDigits      = `01234567`
	punctuation    = `!"#$%&'()*+,-./:;<=>?@[\]^_` + "{|}~`"
	printable      = decDigits + asciiLetters + punctuation + whitespace
)

// LoadModule loads the string module. It is concurrency-safe and idempotent.
func LoadModule() (starlark.StringDict, error) {
	once.Do(func() {
		strModule = starlark.StringDict{
			ModuleName: &starlarkstruct.Module{
				Name: ModuleName,
				Members: starlark.StringDict{
					// constants
					"ascii_lowercase": starlark.String(asciiLowerCase),
					"ascii_uppercase": starlark.String(asciiUpperCase),
					"ascii_letters":   starlark.String(asciiLetters),
					"digits":          starlark.String(decDigits),
					"hexdigits":       starlark.String(hexDigits),
					"octdigits":       starlark.String(octDigits),
					"punctuation":     starlark.String(punctuation),
					"whitespace":      starlark.String(whitespace),
					"printable":       starlark.String(printable),

					// functions
					"length":    starlark.NewBuiltin(ModuleName+".length", length),
					"reverse":   starlark.NewBuiltin(ModuleName+".reverse", reverse),
					"escape":    genStarStrBuiltin("escape", html.EscapeString),
					"unescape":  genStarStrBuiltin("unescape", html.UnescapeString),
					"quote":     genStarStrBuiltin("quote", strconv.Quote),
					"unquote":   genStarStrBuiltin("unquote", robustUnquote),
					"index":     starlark.NewBuiltin(ModuleName+".index", createIndexFunc("index", false, false)),
					"find":      starlark.NewBuiltin(ModuleName+".find", createIndexFunc("find", false, true)),
					"rindex":    starlark.NewBuiltin(ModuleName+".rindex", createIndexFunc("rindex", true, false)),
					"rfind":     starlark.NewBuiltin(ModuleName+".rfind", createIndexFunc("rfind", true, true)),
					"substring": starlark.NewBuiltin(ModuleName+".substring", substring),
					"codepoint": starlark.NewBuiltin(ModuleName+".codepoint", codepoint),
				},
			},
		}
	})
	return strModule, nil
}

// for convenience
var (
	emptyStr string
	none     = starlark.None
)

type (
	starFn func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)
)

// length returns the length of the given value.
func length(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if l := len(args); l != 1 {
		return none, fmt.Errorf(`length() takes exactly one argument (%d given)`, l)
	}

	switch r := args[0]; v := r.(type) {
	case starlark.String:
		return starlark.MakeInt(utf8.RuneCountInString(v.GoString())), nil
	case starlark.Bytes:
		return starlark.MakeInt(len(v)), nil
	default:
		if sv, ok := v.(starlark.Sequence); ok {
			return starlark.MakeInt(sv.Len()), nil
		}
		return none, fmt.Errorf(`length() function isn't supported for '%s' type object`, v.Type())
	}
}

// reverse returns the reversed string of the given value.
func reverse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if l := len(args); l != 1 {
		return none, fmt.Errorf(`reverse() takes exactly one argument (%d given)`, l)
	}

	switch r := args[0]; v := r.(type) {
	case starlark.String:
		rs := []rune(v.GoString())
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		return starlark.String(rs), nil
	case starlark.Bytes:
		bs := []byte(v)
		for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
			bs[i], bs[j] = bs[j], bs[i]
		}
		return starlark.Bytes(bs), nil
	default:
		return none, fmt.Errorf(`reverse() function isn't supported for '%s' type object`, v.Type())
	}
}

// genStarStrBuiltin generates the string operation builtin for Starlark.
func genStarStrBuiltin(fn string, opFn func(string) string) *starlark.Builtin {
	sf := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if l := len(args); l != 1 {
			return none, fmt.Errorf(`%s() takes exactly one argument (%d given)`, fn, l)
		}

		switch r := args[0]; v := r.(type) {
		case starlark.String:
			return starlark.String(opFn(v.GoString())), nil
		case starlark.Bytes:
			return starlark.Bytes(opFn(string(v))), nil
		default:
			return none, fmt.Errorf(`%s() function isn't supported for '%s' type object`, fn, v.Type())
		}
	}
	return starlark.NewBuiltin(ModuleName+"."+fn, sf)
}

// robustUnquote unquotes a string, even if it's not quoted.
func robustUnquote(s string) string {
	if len(s) < 2 {
		return s
	}

	// if it's not quoted, quote it
	old := s
	if !(s[0] == '"' && s[len(s)-1] == '"') {
		s = `"` + s + `"`
	}

	// try to unquote
	ns, err := strconv.Unquote(s)
	if err != nil {
		// if failed, return original string
		return old
	}
	// return unmodified string
	return ns
}

// createIndexFunc generates a Starlark function for finding the index of a substring.
// If reverse is true, it searches from the end of the string.
func createIndexFunc(name string, reverse, returnNegative bool) func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var s, sub string
		if err := starlark.UnpackArgs(name, args, kwargs, "s", &s, "sub", &sub); err != nil {
			return none, err
		}

		// find the substring
		var pos int
		if reverse {
			pos = strings.LastIndex(s, sub)
		} else {
			pos = strings.Index(s, sub)
		}

		// failed to find the substring
		if pos < 0 {
			if returnNegative {
				return starlark.MakeInt(-1), nil
			}
			return none, fmt.Errorf(`%s: substring not found`, name)
		}

		// convert to rune count index
		pos = utf8.RuneCountInString(s[:pos])
		return starlark.MakeInt(pos), nil
	}
}

// substring returns a substring from start to end (exclusive) indices.
func substring(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		s          string
		start, end int
	)
	if err := starlark.UnpackArgs("substring", args, kwargs, "s", &s, "start", &start, "end?", &end); err != nil {
		return none, err
	}

	// convert to rune count
	rs := []rune(s)
	n := len(rs)

	// Handle negative indices
	if start < 0 {
		start += n
	}
	if end < 0 {
		end += n
	}
	if end == 0 { // if end is not provided, use the length of the string
		end = n
	}

	// Check for out of range indices
	if start < 0 || end < 0 || start > n || end > n || start > end {
		return starlark.None, fmt.Errorf(`substring: indices are out of range`)
	}

	return starlark.String(rs[start:end]), nil
}

// codepoint returns the Unicode codepoint of the character at the given index.
func codepoint(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		s   string
		idx int
	)
	if err := starlark.UnpackArgs("codepoint", args, kwargs, "s", &s, "index", &idx); err != nil {
		return none, err
	}

	// convert to rune count
	rs := []rune(s)
	n := len(rs)

	// Handle negative index
	if idx < 0 {
		idx += n
	}

	// Check for out of range index
	if idx < 0 || idx >= n {
		return starlark.None, fmt.Errorf(`codepoint: index out of range`)
	}

	// return the codepoint
	return starlark.String(rs[idx]), nil
}
