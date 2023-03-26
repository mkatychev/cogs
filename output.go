package cogs

import (
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
)

// OutputCfg returns the corresponding value for a given Link struct
func OutputCfg(link *Link, outputType Format) (interface{}, error) {
	if outputType == Dotenv || outputType == List {
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
	case Dotenv, List:
		output = fmt.Sprintf("%s", v)
	}
	return output, err
}

func Keys[K comparable, V any](m map[K]V) []K {
	var keys []K
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Exclude produces a laundered map with exclusionList values missing
func Exclude[K comparable, V any](exclusionList []K, linkMap map[K]V) map[K]V {
	newLinkMap := make(map[K]V)

	for k := range linkMap {
		if InList(k, exclusionList) {
			continue
		}
		newLinkMap[k] = linkMap[k]
	}
	return newLinkMap
}

// InList verifies that a given string is in a string slice
func InList[T comparable](s T, ss []T) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
