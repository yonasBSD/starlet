// Package random defines functions that generate random values for various distributions, it's intended to be a drop-in subset of Python's random module for Starlark.
package random

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"sort"
	"sync"

	tps "github.com/1set/starlet/dataconv/types"
	guuid "github.com/google/uuid"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the expected name for this Module when used in Starlark's load() function, eg: load('random', 'choice')
const ModuleName = "random"

var (
	once   sync.Once
	module starlark.StringDict
)

// LoadModule loads the random module. It is concurrency-safe and idempotent.
func LoadModule() (starlark.StringDict, error) {
	once.Do(func() {
		module = starlark.StringDict{
			"random": &starlarkstruct.Module{
				Name: "random",
				Members: starlark.StringDict{
					"randbytes": starlark.NewBuiltin("random.randbytes", randbytes),
					"randstr":   starlark.NewBuiltin("random.randstr", randstr),
					"randb32":   starlark.NewBuiltin("random.randb32", randb32),
					"randint":   starlark.NewBuiltin("random.randint", randint),
					"choice":    starlark.NewBuiltin("random.choice", choice),
					"choices":   starlark.NewBuiltin("random.choices", choices),
					"shuffle":   starlark.NewBuiltin("random.shuffle", shuffle),
					"random":    starlark.NewBuiltin("random.random", random),
					"uniform":   starlark.NewBuiltin("random.uniform", uniform),
					"uuid":      starlark.NewBuiltin("random.uuid", uuid),
				},
			},
		}
	})
	return module, nil
}

// for convenience
var (
	emptyStr    string
	none        = starlark.None
	defaultLenN = big.NewInt(10)
)

// randbytes(n) returns a random byte string of length n.
func randbytes(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var n starlark.Int
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "n?", &n); err != nil {
		return nil, err
	}
	// set default value if n is not provided correctly
	nInt := n.BigInt()
	if nInt.Sign() <= 0 {
		nInt = defaultLenN
	}
	// get random bytes
	buf := make([]byte, nInt.Int64())
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return starlark.Bytes(buf), nil
}

// randstr(chars, n) returns a random string of given length from given characters.
func randstr(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var (
		ab starlark.String
		n  starlark.Int
	)
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "chars", &ab, "n?", &n); err != nil {
		return nil, err
	}
	// set default value if n is not provided correctly
	nInt := n.BigInt()
	if nInt.Sign() <= 0 {
		nInt = defaultLenN
	}
	// get random strings
	s, err := getRandStr(ab.GoString(), nInt.Int64())
	if err != nil {
		return nil, err
	}
	return starlark.String(s), nil
}

// randb32(n, sep) returns a random base32 string of length n with optional separator dash for every sep characters.
func randb32(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var n, sep starlark.Int
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "n?", &n, "sep?", &sep); err != nil {
		return nil, err
	}
	// set default value if n is not provided correctly
	nInt := n.BigInt()
	if nInt.Sign() <= 0 {
		nInt = defaultLenN
	}
	nSep := sep.BigInt()
	if nSep.Sign() <= 0 {
		nSep = big.NewInt(0)
	}
	// get random strings
	const ab = `ABCDEFGHIJKLMNOPQRSTUVWXYZ234567` // standard base32 encoding chars, as defined in RFC 4648.
	s, err := getRandStr(ab, nInt.Int64())
	if err != nil {
		return nil, err
	}
	// add separator
	if n := int(nSep.Int64()); n > 0 && n < len(s) {
		// add separator every n chars
		var buf []rune
		for i, r := range s {
			if i > 0 && i%n == 0 {
				buf = append(buf, '-', r)
			} else {
				buf = append(buf, r)
			}
		}
		s = string(buf)
	}
	return starlark.String(s), nil
}

// randint(a, b) returns a random integer N such that a <= N <= b. Alias for randrange(a, b+1).
func randint(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var a, b starlark.Int
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "a", &a, "b", &b); err != nil {
		return nil, err
	}
	// a <= b, then a - b <= 0
	if cmp := a.Sub(b).BigInt(); cmp.Sign() > 0 {
		return nil, errors.New(`a must be less than or equal to b`)
	}
	// get random diff
	var (
		aInt = a.BigInt()
		bInt = b.BigInt()
	)
	diff := new(big.Int).Sub(bInt, aInt)
	diff.Add(diff, big.NewInt(1)) // make it inclusive
	n, err := rand.Int(rand.Reader, diff)
	if err != nil {
		return nil, err
	}
	// rand big int is low + diff
	n.Add(n, aInt)
	return starlark.MakeBigInt(n), nil
}

// choice returns a random element from the non-empty sequence seq. If seq is empty, raises a ValueError.
func choice(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var seq starlark.Indexable
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "seq", &seq); err != nil {
		return nil, err
	}
	l := seq.Len()
	if l == 0 {
		return nil, errors.New(`cannot choose from an empty sequence`)
	}
	// get random index
	i, err := getRandomInt(l)
	if err != nil {
		return nil, err
	}
	// return element at index
	return seq.Index(i), nil
}

// choices returns a k sized list of elements chosen from the population with replacement.
func choices(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		population  starlark.Indexable
		weights     *starlark.List
		cumWeights  *starlark.List
		numOfResult = 1
	)

	if err := starlark.UnpackArgs(bn.Name(), args, kwargs,
		"population", &population,
		"weights?", &weights,
		"cum_weights?", &cumWeights,
		"k?", &numOfResult); err != nil {
		return nil, err
	}

	// population must be non-empty
	n := population.Len()
	if n == 0 {
		return nil, errors.New("population is empty")
	}
	// k should be positive, otherwise return an empty list
	if numOfResult <= 0 {
		l := starlark.NewList([]starlark.Value{})
		return l, nil
	}

	// get or calculate cumulative weights
	var (
		cumulativeWeights []float64
		err               error
	)
	if cumWeights != nil {
		if weights != nil {
			return nil, errors.New("cannot specify both weights and cumulative weights")
		}
		cumulativeWeights, err = listToFloat64Slice(cumWeights)
		if err != nil {
			return nil, err
		}
		if len(cumulativeWeights) != n {
			return nil, errors.New("the number of weights does not match the population")
		}
		lastWeight := cumulativeWeights[0]
		for i := 1; i < n; i++ {
			if cumulativeWeights[i] < lastWeight {
				return nil, errors.New("cumulative weights must be non-decreasing")
			}
			lastWeight = cumulativeWeights[i]
		}
	} else if weights != nil {
		relativeWeights, err := listToFloat64Slice(weights)
		if err != nil {
			return nil, err
		}
		if len(relativeWeights) != n {
			return nil, errors.New("the number of weights does not match the population")
		}
		cumulativeWeights = make([]float64, n)
		sum := 0.0
		for i, w := range relativeWeights {
			sum += w
			cumulativeWeights[i] = sum
		}
	}

	// create the result list
	result := make([]starlark.Value, numOfResult)
	if cumulativeWeights == nil {
		// Equal probability selection
		for i := 0; i < numOfResult; i++ {
			index, err := getRandomInt(n)
			if err != nil {
				return nil, err
			}
			result[i] = population.Index(index)
		}
	} else {
		// Weighted selection
		total := cumulativeWeights[n-1]
		if total <= 0 {
			return nil, errors.New("total of weights must be greater than zero")
		}
		if math.IsInf(total, 0) || math.IsNaN(total) {
			return nil, errors.New("total of weights must be finite")
		}

		for i := 0; i < numOfResult; i++ {
			r, err := getRandomFloat(1 << 53)
			if err != nil {
				return nil, err
			}
			target := r * total
			index := sort.SearchFloat64s(cumulativeWeights, target)
			result[i] = population.Index(index)
		}
	}

	// return the result list
	return starlark.NewList(result), nil
}

// shuffle(x) shuffles the sequence x in place.
func shuffle(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var seq starlark.HasSetIndex
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "seq", &seq); err != nil {
		return nil, err
	}
	// nothing to do if seq is empty or has only one element
	l := seq.Len()
	if l <= 1 {
		return none, nil
	}
	// The shuffle algorithm is the Fisher-Yates Shuffle and its complexity is O(n).
	var (
		randBig   = new(big.Int)
		randBytes = make([]byte, 8)
		swap      = func(i, j int) error {
			x := seq.Index(i)
			y := seq.Index(j)

			e1 := seq.SetIndex(i, y)
			e2 := seq.SetIndex(j, x)

			if e1 != nil {
				return e1
			} else if e2 != nil {
				return e2
			}
			return nil
		}
	)
	for i := uint64(l - 1); i > 0; {
		if _, err := rand.Read(randBytes); err != nil {
			return nil, err
		}
		randBig.SetBytes(randBytes)
		for num := randBig.Uint64(); num > i && i > 0; i-- {
			max := i + 1
			j := int(num % max)
			num /= max
			if e := swap(int(i), j); e != nil {
				return nil, e
			}
		}
	}
	// done
	return none, nil
}

// random() returns a random floating point number in the range 0.0 <= X < 1.0.
func random(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// check the arguments: no arguments
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	// get random float
	f, err := getRandomFloat(1 << 53)
	if err != nil {
		return nil, err
	}
	return starlark.Float(f), nil
}

// uuid() returns a random UUID (RFC 4122 version 4).
func uuid(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// check the arguments: no arguments
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	// get random UUID
	u := guuid.New()
	return starlark.String(u.String()), nil
}

// uniform(a, b) returns a random floating point number N such that a <= N <= b for a <= b and b <= N <= a for b < a. The end-point value b may or may not be included in the range depending on floating-point rounding in the equation a + (b-a) * random().
func uniform(thread *starlark.Thread, bn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// precondition checks
	var a, b tps.FloatOrInt
	if err := starlark.UnpackArgs(bn.Name(), args, kwargs, "a", &a, "b", &b); err != nil {
		return nil, err
	}
	// get random float
	f, err := getRandomFloat(1 << 53)
	if err != nil {
		return nil, err
	}
	// a + (b-a) * random()
	diff := float64(b - a)
	return starlark.Float(float64(a) + diff*f), nil
}

// the following functions are not exposed to Starlark directly, but can be used in other Starlark builtins.

// getRandomInt returns a random integer in the range [0, max).
func getRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New(`max must be > 0`)
	}
	maxBig := new(big.Int).SetUint64(uint64(max))
	n, err := rand.Int(rand.Reader, maxBig)
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// getRandomFloat returns a random floating point number in the range [0.0, 1.0).
func getRandomFloat(prec int64) (n float64, err error) {
	if prec <= 0 {
		return 0, errors.New(`prec must be > 0`)
	}
	maxBig := new(big.Int).SetUint64(uint64(prec))
	nBig, err := rand.Int(rand.Reader, maxBig)
	if err != nil {
		return 0, err
	}
	return float64(nBig.Int64()) / float64(prec), nil
}

// getRandStr returns a random string of given length from given characters.
func getRandStr(chars string, length int64) (string, error) {
	// basic checks
	if length <= 0 {
		return emptyStr, errors.New(`length must be > 0`)
	}
	if chars == emptyStr {
		return emptyStr, errors.New(`chars must not be empty`)
	}

	// split chars into runes
	runes := []rune(chars)
	rc := len(runes)

	// get random runes
	buf := make([]rune, length)
	for i := range buf {
		idx, err := getRandomInt(rc)
		if err != nil {
			return emptyStr, err
		}
		buf[i] = runes[idx]
	}

	// convert to string
	return string(buf), nil
}

// listToFloat64Slice is a helper function to convert a Starlark list of weights to a []float64.
func listToFloat64Slice(list *starlark.List) ([]float64, error) {
	result := make([]float64, list.Len())
	iter := list.Iterate()
	defer iter.Done()
	var x starlark.Value
	for i := 0; iter.Next(&x); i++ {
		if num, ok := x.(starlark.Float); ok {
			result[i] = float64(num)
		} else if num, ok := x.(starlark.Int); ok {
			val := num.Float()
			result[i] = float64(val)
		} else {
			return nil, errors.New("weights must be numeric")
		}
	}
	return result, nil
}
