package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bestowinc/cogs"
	"github.com/docopt/docopt-go"
	"github.com/pelletier/go-toml"
	logging "gopkg.in/op/go-logging.v1"
	"gopkg.in/yaml.v3"
)

func main() {
	usage := `
COGS

Usage:
  cogs generate <env> <cog-file> [--out=<type>]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --output=<type>  Configuration output type [default: json].
`

	opts, _ := docopt.ParseArgs(usage, os.Args[1:], "0.1")
	var conf struct {
		Generate bool
		Env      string
		File     string `docopt:"<cog-file>"`
		Output   string `docopt:"--out"`
	}

	opts.Bind(&conf)

	if conf.Output == "" {
		conf.Output = "json"
	}

	logging.SetLevel(logging.WARNING, "yq")
	switch {
	case conf.Generate:
		cfgMap, err := cogs.Generate(conf.Env, conf.File)
		if err != nil {
			panic(err)
		}
		var output []byte
		switch conf.Output {
		case "json":
			output, err = json.MarshalIndent(cfgMap, "", "  ")
		case "yaml":
			output, err = yaml.Marshal(cfgMap)
		case "toml":
			output, err = toml.Marshal(cfgMap)
		default:
			panic("invalid arg: --out=%" + conf.Output)
		}
		if err != nil {
			panic(err)
		}
		fmt.Println(string(output))
	}
}
