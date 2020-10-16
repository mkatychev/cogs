package main

import (
	"fmt"
	"os"
	"strings"
)

// ----------------------
// CLI optparse functions
// ----------------------

func ifErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getRawValue(cfgMap map[string]string) string {
	output := ""
	// for now, no delimiter
	for _, v := range cfgMap {
		output = output + v
	}
	return output

}

func upperKeys(cfgMap map[string]string) map[string]string {
	newCfgMap := make(map[string]string)
	for k, v := range cfgMap {
		newCfgMap[strings.ToUpper(k)] = v
	}
	return newCfgMap
}

// exclude produces a laundered map with exclusionList values misssing
func exclude(exclusionList []string, cfgMap map[string]string) map[string]string {
	newCfgMap := make(map[string]string)
	for k, _ := range cfgMap {
		if inList(k, exclusionList) {
			continue
		}
		newCfgMap[k] = cfgMap[k]
	}
	return newCfgMap
}

func inList(s string, ss []string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
