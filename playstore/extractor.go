package playstore

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"gopkg.in/guregu/null.v4"
)

type extractor struct {
	payload string
	errors  []error
}

func NewExtractor(payload string) *extractor {
	return &extractor{
		payload: payload,
		errors:  nil,
	}
}

func (e *extractor) Errors() []error {
	return e.errors
}

func (e *extractor) Error(err error) {
	e.errors = append(e.errors, err)
}

func (e *extractor) error(path string, msg string) {
	e.errors = append(e.errors, fmt.Errorf("%s: %s", path, msg))
}

func (e *extractor) IsNull(path string) bool {
	result := gjson.Get(e.payload, path)
	return result.Type == gjson.Null
}

func (e *extractor) Bool(path string) bool {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.False:
		return false
	case gjson.True:
		return true
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return false
	}
}

func (e *extractor) int(path string) (int64, bool) {
	result := gjson.Get(e.payload, path)

	if result.Type != gjson.Number {
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return 0, false
	}

	num := result.Num
	if num != math.Trunc(num) {
		e.error(path, "not an integer")
		return 0, false
	}

	if num < math.MinInt64 || num > math.MaxInt64 {
		e.error(path, "float cannot be converted to int64")
		return 0, false
	}

	return int64(num), true
}

func (e *extractor) Int(path string) int64 {
	i, ok := e.int(path)
	if !ok {
		return i
	} else {
		return i
	}
}

func (e *extractor) Float(path string) float64 {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.Number:
		return result.Num
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return 0.0
	}
}

func (e *extractor) String(path string) string {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.String:
		return strings.ReplaceAll(result.Str, "\x00", "")
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return ""
	}
}

func (e *extractor) StringSlice(path string) []string {
	result := gjson.Get(e.payload, path)
	if !result.IsArray() {
		e.error(path, "is not array")
		return nil
	}
	results := result.Array()
	slice := make([]string, 0, len(results))

	for i, r := range results {
		switch r.Type {
		case gjson.String:
			slice = append(slice, strings.ReplaceAll(r.Str, "\x00", ""))
		default:
			e.error(fmt.Sprintf("%s.%d", path, i), "wrong type")
			return nil
		}
	}

	return slice
}

func (e *extractor) FloatSlice(path string) []float64 {
	result := gjson.Get(e.payload, path)
	if !result.IsArray() {
		e.error(path, "is not array")
		return nil
	}
	results := result.Array()
	slice := make([]float64, 0, len(results))

	for i, r := range results {
		switch r.Type {
		case gjson.Number:
			slice = append(slice, r.Num)
		default:
			e.error(fmt.Sprintf("%s.%d", path, i), "wrong type")
			return nil
		}
	}

	return slice
}

func (e *extractor) OptionalFloatSlice(path string) []null.Float {
	result := gjson.Get(e.payload, path)
	if !result.IsArray() {
		e.error(path, "is not array")
		return nil
	}
	results := result.Array()
	slice := make([]null.Float, 0, len(results))

	for i, r := range results {
		switch r.Type {
		case gjson.Null:
			slice = append(slice, null.Float{})
		case gjson.Number:
			slice = append(slice, null.FloatFrom(r.Num))
		default:
			e.error(fmt.Sprintf("%s.%d", path, i), "wrong type")
			return nil
		}
	}

	return slice
}

func (e *extractor) Time(path string) time.Time {
	i, ok := e.int(path)
	if !ok {
		return time.Time{}
	}

	return time.Unix(i, 0).UTC()
}

func (e *extractor) Json(path string) gjson.Result {
	return gjson.Get(e.payload, path)
}

func (e *extractor) OptionalBool(path string) null.Bool {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.Null:
		return null.Bool{}
	case gjson.False:
		return null.BoolFrom(false)
	case gjson.True:
		return null.BoolFrom(true)
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return null.Bool{}
	}
}

func (e *extractor) OptionalInt(path string) null.Int {
	result := gjson.Get(e.payload, path)

	if result.Type == gjson.Null {
		return null.Int{}
	}

	i, ok := e.int(path)
	if !ok {
		return null.Int{}
	}

	return null.IntFrom(i)
}

func (e *extractor) OptionalFloat(path string) null.Float {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.Null:
		return null.Float{}
	case gjson.Number:
		return null.FloatFrom(result.Num)
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return null.Float{}
	}
}

func (e *extractor) OptionalString(path string) null.String {
	result := gjson.Get(e.payload, path)

	switch result.Type {
	case gjson.Null:
		return null.String{}
	case gjson.String:
		return null.StringFrom(strings.ReplaceAll(result.Str, "\x00", ""))
	default:
		e.error(path, fmt.Sprintf("wrong type '%s'", result.Type))
		return null.String{}
	}
}

func (e *extractor) OptionalStringSlice(path string) []string {
	result := gjson.Get(e.payload, path)

	if result.Type == gjson.Null {
		return nil
	}

	if !result.IsArray() {
		e.error(path, "is not array")
		return nil
	}

	results := result.Array()
	slice := make([]string, 0, len(results))

	for i, r := range results {
		switch r.Type {
		case gjson.String:
			slice = append(slice, strings.ReplaceAll(r.Str, "\x00", ""))
		case gjson.Null:
			continue
		default:
			e.error(fmt.Sprintf("%s.%d", path, i), "wrong type")
			return nil
		}
	}

	return slice
}

func (e *extractor) OptionalTime(path string) null.Time {
	result := gjson.Get(e.payload, path)
	if result.Type == gjson.Null {
		return null.Time{}
	}

	i, ok := e.int(path)
	if !ok {
		return null.Time{}
	}

	return null.TimeFrom(time.Unix(i, 0).UTC())
}
