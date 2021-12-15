package cogs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
)

// ReadType represents the logic used to derive the deserliazied value for a given Link
type ReadType string

const (
	// read format overrides
	rDotenv ReadType = "dotenv"
	rJSON   ReadType = "json"
	rYAML   ReadType = "yaml"
	rTOML   ReadType = "toml"
	// complex values of a given markup type are appended with "{}"
	rJSONComplex ReadType = "json{}" // complex JSON key value pair: {"k":{"v1":[],"v2":[]}}
	rYAMLComplex ReadType = "yaml{}" // complex YAML key value pair: {k: {v1: [], v2: []}}
	rTOMLComplex ReadType = "toml{}" // complex TOML key value pair: k = {v1 = [], v2 = []}

	deferred ReadType = ""      // defer file config type to filename suffix
	rWhole   ReadType = "whole" // indicates to associate the entirety of a file to the given key name
	rRaw     ReadType = "raw"   // indicates to associate the entirety of a file to the given key name without serialization
	rGear    ReadType = "gear"  // treat TOML table as a nested gear object
)

// Validate ensures that a string is a valid readType enum
func (t ReadType) Validate() error {
	switch t {
	case rDotenv, rJSON, rYAML, rTOML,
		rJSONComplex, rYAMLComplex, rTOMLComplex, rWhole,
		rRaw,
		deferred:
		return nil
	default: // deferred readType should not be validated
		return fmt.Errorf("%s is an invalid linkType", t)
	}
}

// isComplex returns true if the readType is complex
func (t ReadType) isComplex() bool {
	switch t {
	case rJSONComplex, rYAMLComplex, rTOMLComplex, rWhole:
		return true
	}
	return false
}

type unmarshalFn func([]byte, interface{}) error

// getUnmarshal returns the corresponding function to unmarshal a given read type
func (t ReadType) getUnmarshal() (unmarshalFn, error) {
	switch t {
	case rJSON, rJSONComplex:
		return json.Unmarshal, nil
	case rTOML, rTOMLComplex:
		return toml.Unmarshal, nil
	case rYAML, rYAMLComplex:
		return yaml.Unmarshal, nil
	}
	return nil, fmt.Errorf("unsupported type for GetUnmarshal: %s", t)
}

func (t ReadType) String() string {
	switch t {
	case rDotenv:
		return string(rDotenv)
	case rJSON:
		return "flat json"
	case rYAML:
		return "flat yaml"
	case rTOML:
		return "flat toml"
	case rJSONComplex:
		return "complex json"
	case rYAMLComplex:
		return "complex yaml"
	case rTOMLComplex:
		return "complex toml"
	case rWhole:
		return "whole file"
	case rRaw:
		return "whole unserialized file"
	case rGear:
		return "gear object"
	case deferred:
		return "deferred"
	default:
		return "unknown"
	}
}

// Format represents the final marshalled k/v output type from a resolved Gear
// TODO reconcile readType and Format patterns
type Format string

// Formats for respective object notation
const (
	JSON   Format = "json"
	YAML   Format = "yaml"
	TOML   Format = "toml"
	Dotenv Format = "dotenv"
	List   Format = "list" // omit keys
)

// Validate ensures that a string maps to a valid Format
func (t Format) Validate() error {
	switch t {
	case JSON, YAML, TOML, Dotenv, List:
		return nil
	default: // deferred readType should not be validated
		return fmt.Errorf("%s is an invalid Format", string(t))
	}
}

// FormatForPath returns the correct format given the path to a file
func FormatForPath(path string) Format {
	format := List
	switch {
	case IsYAMLFile(path):
		format = YAML
	case IsTOMLFile(path):
		format = TOML
	case IsJSONFile(path):
		format = JSON
	case IsEnvFile(path):
		format = Dotenv
	}
	return format
}

// FormatLinkInput returns the correct format given the readType
func FormatLinkInput(link *Link) (format Format) {
	if link.header.Get("accept") == "application/json" {
		return JSON
	}
	switch link.readType {
	case rJSON, rJSONComplex:
		format = JSON
	case rDotenv:
		format = Dotenv
	// grab Format from filepath suffix if there are no explicit type overrides
	default:
		format = FormatForPath(link.Path)
	}
	return format
}

// IsYAMLFile returns true if a given file path corresponds to a YAML file
func IsYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}

// IsTOMLFile returns true if a given file path corresponds to a TOML file
func IsTOMLFile(path string) bool {
	return strings.HasSuffix(path, ".toml") || strings.HasSuffix(path, ".tml")
}

// IsJSONFile returns true if a given file path corresponds to a JSON file
func IsJSONFile(path string) bool {
	return strings.HasSuffix(path, ".json")
}

// IsEnvFile returns true if a given file path corresponds to a .env file
func IsEnvFile(path string) bool {
	return strings.HasSuffix(path, ".env")
}

// IsSimpleValue is intended to see if the underlying value allows a flat map to be retained
func IsSimpleValue(i interface{}) bool {
	switch i.(type) {
	case string,
		float32, float64,
		uint, uint8, uint16, uint32, uint64,
		int, int8, int16, int32, int64,
		bool:
		return true
	}
	return false
}

// TODO  ErrNotASimpleValue = errorW{fmt:"%s of type %T is not a simple value"}

// SimpleValueToString converts an underlying type to a string, returning an error if it is not a simple value
func SimpleValueToString(i interface{}) (str string, err error) {
	switch t := i.(type) {
	case string:
		str = t
	case int:
		str = strconv.Itoa(t)
	case int8:
		str = strconv.FormatInt(int64(t), 10)
	case int16:
		str = strconv.FormatInt(int64(t), 10)
	case int32:
		str = strconv.FormatInt(int64(t), 10)
	case int64:
		str = strconv.FormatInt(int64(t), 10)
	case uint:
		str = strconv.FormatUint(uint64(t), 10)
	case uint8:
		str = strconv.FormatUint(uint64(t), 10)
	case uint16:
		str = strconv.FormatUint(uint64(t), 10)
	case uint32:
		str = strconv.FormatUint(uint64(t), 10)
	case uint64:
		str = strconv.FormatUint(uint64(t), 10)
	case bool:
		str = strconv.FormatBool(t)
	case float32:
		str = strconv.FormatFloat(float64(t), 'E', -1, 64)
	case float64:
		str = strconv.FormatFloat(t, 'E', -1, 32)
	default:
		err = fmt.Errorf("%s of type %T is not a simple value", t, t)
	}
	return str, err
}
