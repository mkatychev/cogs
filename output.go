package cogs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
)

// Format represents the final marshalled k/v output type from a resolved Gear
type Format string

// Formats for respective object notation
const (
	JSON   Format = "json"
	YAML   Format = "yaml"
	TOML   Format = "toml"
	Dotenv Format = "dotenv"
	Raw    Format = "raw"
)

// Validate ensures that a string maps to a valid Format
func (t Format) Validate() error {
	switch t {
	case JSON, YAML, TOML, Dotenv, Raw:
		return nil
	default: // deferred readType should not be validated
		return fmt.Errorf("%s is an invalid Format", string(t))
	}
}

// OutputCfg returns the corresponding value for a given Link struct
func OutputCfg(link *Link, outputType Format) (interface{}, error) {
	if outputType == Dotenv || outputType == Raw {
		// don't try to marshal simple primitive types
		if IsSimpleValue(link.Value) {
			return SimpleValueToString(link.Value)
		}
		return marshalComplexValue(link.Value, FormatLinkInput(link))
	}
	return link.Value, nil
}

func marshalComplexValue(v interface{}, inputType Format) (output string, err error) {
	var b []byte
	switch inputType {
	case JSON:
		b, err = json.Marshal(v)
		output = string(b)
	case YAML:
		b, err = yaml.Marshal(v)
		output = string(b)
	case TOML:
		b, err = toml.Marshal(v)
		output = string(b)
	case Dotenv, Raw:
		output = fmt.Sprintf("%s", v)
	}
	return output, err
}

// FormatForPath returns the correct format given the path to a file
func FormatForPath(path string) Format {
	format := Raw
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

// SimpleValueToString converts an undelying type to a string, returning an error if it is not a simple value
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

// Exclude produces a laundered map with exclusionList values missing
func Exclude(exclusionList []string, linkMap LinkMap) LinkMap {
	newLinkMap := make(LinkMap)

	for k := range linkMap {
		if InList(k, exclusionList) {
			continue
		}
		newLinkMap[k] = linkMap[k]
	}
	return newLinkMap
}

// InList verifies that a given string is in a string slice
func InList(s string, ss []string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
