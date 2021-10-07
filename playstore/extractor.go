package playstore

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/markphelps/optional"
)

type extractor struct {
	dataMap             map[string]interface{}
	serviceRequestIdMap map[string]string
	errors              []error
}

func newExtractor(dataMap map[string]interface{}, serviceRequestIdMap map[string]string) extractor {
	return extractor{
		dataMap:             dataMap,
		serviceRequestIdMap: serviceRequestIdMap,
		errors:              nil,
	}
}

func (e *extractor) Error(err error) {
	e.errors = append(e.errors, err)
}

func (e *extractor) Errors() []error {
	return e.errors
}

func (e *extractor) Block(key string) *blockExtractor {
	block, exists := e.dataMap[key]
	if !exists {
		e.Error(fmt.Errorf("Block(%s): no such block", key))

		// This chucks away any errors from extracting paths on the returned blockExtractor
		return &blockExtractor{
			data:   nil,
			errors: &[]error{},
		}
	}
	return &blockExtractor{
		data:   block,
		key:    key,
		errors: &e.errors,
	}
}

func (e *extractor) BlockWithServiceRequestId(serviceRequestId string) *blockExtractor {
	key, ok := e.serviceRequestIdMap[serviceRequestId]
	if !ok {
		e.Error(fmt.Errorf("BlockWithServiceRequestId(%s): no such service request ID", key))
	}

	return e.Block(key)
}

type blockExtractor struct {
	data   interface{}
	key    string
	errors *[]error
}

func (e *blockExtractor) error(funcName string, errorMsg string, path []int) {
	var sb strings.Builder
	for i, p := range path {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatInt(int64(p), 10))
	}
	commaSeparatedPath := sb.String()

	var err error
	if e.key == "" {
		err = fmt.Errorf("%s(%s): %s", funcName, commaSeparatedPath, errorMsg)
	} else {
		err = fmt.Errorf("%s(%s, %s): %s", funcName, e.key, commaSeparatedPath, errorMsg)
	}

	*e.errors = append(*e.errors, err)
}

func (e *blockExtractor) Errors() []error {
	return *e.errors
}

func (e *blockExtractor) Json(path ...int) interface{} {
	ret, err := pluck(e.data, path...)
	if err != nil {
		return nil
	}
	return ret
}

func (e *blockExtractor) Bool(path ...int) bool {
	val := e.Json(path...)

	switch val := val.(type) {
	case nil:
		return false
	case bool:
		return val
	case json.Number:
		floating, err := val.Float64()
		if err == nil {
			return floating != 0
		}

		integer, err := val.Int64()
		if err == nil {
			return integer != 0
		}
		e.error("Bool", "cannot convert json.Number to float64 or int64", path)
		return false
	case float64, int64:
		return val != 0
	case string:
		return val != ""
	default:
		e.error("Bool", "wrong type", path)
		return false
	}
}

func (e *blockExtractor) Number(path ...int) json.Number {
	val := e.Json(path...)

	number, ok := val.(json.Number)
	if !ok {
		e.error("Number", "wrong type", path)
	}
	return number
}

func (e *blockExtractor) Int64(path ...int) int64 {
	val := e.Json(path...)

	switch val := val.(type) {
	case int64:
		return val
	case float64:
		if val == math.Trunc(val) {
			if val > math.MaxInt64 {
				e.error("Int64", "float64 is too large", path)
				return 0
			}
			return int64(val)
		} else {
			e.error("Int64", "float64 is not an integer", path)
			return 0
		}
	case json.Number:
		integer, err := val.Int64()
		if err != nil {
			e.error("Int64", "cannot convert json.Number to int64", path)
		}
		return integer
	default:
		e.error("Int64", "wrong type", path)
		return 0
	}
}

func (e *blockExtractor) String(path ...int) string {
	val := e.Json(path...)

	switch val := val.(type) {
	case string:
		return val
	default:
		e.error("String", "wrong type", path)
		return ""
	}
}

func (e *blockExtractor) OptionalString(path ...int) optional.String {
	val := e.Json(path...)

	switch val := val.(type) {
	case nil:
		return optional.String{}
	case string:
		return optional.NewString(val)
	default:
		e.error("OptionalString", "wrong type", path)
		return optional.String{}
	}
}

func (e *blockExtractor) OptionalInt64(path ...int) optional.Int64 {
	val := e.Json(path...)

	if val == nil {
		return optional.Int64{}
	}
	return optional.NewInt64(e.Int64(path...))
}

func (e *blockExtractor) OptionalFloat64(path ...int) optional.Float64 {
	val := e.Json(path...)

	switch val := val.(type) {
	case nil:
		return optional.Float64{}
	case json.Number:
		floating, err := val.Float64()
		if err != nil {
			e.error("OptionalFloat64", "cannot convert json.Number to float64", path)
			return optional.Float64{}
		}
		return optional.NewFloat64(floating)
	case float64:
		return optional.NewFloat64(val)
	case int64:
		return optional.NewFloat64(float64(val))
	default:
		e.error("OptionalFloat64", "wrong type", path)
		return optional.Float64{}
	}
}
