package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bestowinc/cogs"
	"github.com/docopt/docopt-go"
	"github.com/pelletier/go-toml"
	logging "gopkg.in/op/go-logging.v1"
	"gopkg.in/yaml.v3"
)

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

func main() {
	usage := `
COGS COnfiguration manaGement S

Usage:
  cogs generate <env> <cog-file> [--out=<type>] [--keys=<key,>] [--no-enc] [--envsubst]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental subsitution on the given cog file.
  --keys=<key,>    Return specific keys from cog manifest.
  --out=<type>     Configuration output type [default: json].
                   Valid types: json, toml, yaml, raw.`

	opts, _ := docopt.ParseArgs(usage, os.Args[1:], "0.3.1")
	var conf struct {
		Generate bool
		Env      string
		File     string `docopt:"<cog-file>"`
		Output   string `docopt:"--out"`
		Keys     string
		NoEnc    bool
		Raw      bool
		EnvSubst bool `docopt:"--envsubst"`
	}

	err := opts.Bind(&conf)
	ifErr(err)
	logging.SetLevel(logging.WARNING, "yq")
	cogs.NoEnc = conf.NoEnc
	cogs.EnvSubst = conf.EnvSubst

	// filterCfgMap retains only key names passed to --keys
	filterCfgMap := func(cfgMap map[string]string) (map[string]string, error) {

		if conf.Keys == "" {
			return cfgMap, nil
		}
		keyList := strings.Split(conf.Keys, ",")
		newCfgMap := make(map[string]string)
		for _, key := range keyList {
			var ok bool
			newCfgMap[key], ok = cfgMap[key]
			if !ok {
				encHint := ""
				if conf.NoEnc {
					encHint = "\n--no-enc was called: was it an ecrypted value? "
				}
				return nil, fmt.Errorf("--key: [%s] missing from generated config%s", key, encHint)
			}
		}
		return newCfgMap, nil
	}

	switch {
	case conf.Generate:
		cfgMap, err := cogs.Generate(conf.Env, conf.File)
		ifErr(err)
		cfgMap, err = filterCfgMap(cfgMap)
		ifErr(err)

		var output []byte
		switch conf.Output {
		case "json":
			output, err = json.MarshalIndent(cfgMap, "", "  ")
		case "yaml":
			output, err = yaml.Marshal(cfgMap)
		case "toml":
			output, err = toml.Marshal(cfgMap)
		case "raw":
			fmt.Fprintln(os.Stdout, getRawValue(cfgMap))
			os.Exit(0)
		default:
			err = fmt.Errorf("invalid arg: --out=" + conf.Output)
		}
		fmt.Fprintln(os.Stdout, string(output))
	}
}
