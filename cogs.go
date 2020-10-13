package cogs

import (
	"fmt"

	"go.mozilla.org/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

func example() {
	yamlMap := make(map[string]interface{})
	sec, err := decrypt.File("./test_files/secret.enc.yaml", "yaml")
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(sec, &yamlMap)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", yamlMap)
}
