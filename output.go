package cogs

import (
	"encoding/json"
	"fmt"
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
func OutputCfg(link *Link, format Format) (interface{}, error) {
	if link.Value != "" && link.ComplexValue != nil {
		return nil, fmt.Errorf("Link.Name[%s]: Link.Value and Link.ComplexValue are both non-empty", link.Name)
	}
	if link.ComplexValue == nil {
		return link.Value, nil
	}
	if format == Dotenv || format == Raw {
		strValue, err := marshalComplexValue(link.ComplexValue, FormatForLink(link))
		if err != nil {
			return nil, err
		}
		return strValue, nil
	}
	return link.ComplexValue, nil
}

func marshalComplexValue(v interface{}, format Format) (output string, err error) {
	var b []byte
	switch format {
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

// FormatForLink returns the correct format given the readType
func FormatForLink(link *Link) (format Format) {
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

// Exclude produces a laundered map with exclusionList values missing
func Exclude(exclusionList []string, cfgMap CfgMap) CfgMap {
	newCfgMap := make(map[string]interface{})

	for k := range cfgMap {
		if InList(k, exclusionList) {
			continue
		}
		newCfgMap[k] = cfgMap[k]
	}
	return newCfgMap
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
