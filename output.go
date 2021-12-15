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

// Exclude produces a laundered map with exclusionList values missing
func Exclude(exclusionList []string, linkMap map[string]*Link) map[string]*Link {
	newLinkMap := make(map[string]*Link)

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
