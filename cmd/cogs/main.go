package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bestowinc/cogs"
	"github.com/docopt/docopt-go"
	logging "gopkg.in/op/go-logging.v1"
)

func main() {
	usage := `Usage:
  cogs generate <env> <cog-file>`

	opts, _ := docopt.ParseArgs(usage, os.Args[1:], "0.1")
	var conf struct {
		Generate bool
		Env      string
		File     string `docopt:"<cog-file>"`
	}

	logging.SetLevel(logging.WARNING, "yq")

	opts.Bind(&conf)
	switch {
	case conf.Generate:
		cfgMap, err := cogs.Generate(conf.Env, conf.File)
		if err != nil {
			panic(err)
		}
		output, _ := json.MarshalIndent(cfgMap, "", "  ")
		fmt.Println(string(output))
	}
}
