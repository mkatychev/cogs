package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Bestowinc/cogs"
	"github.com/docopt/docopt-go"
	"github.com/joho/godotenv"
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

func upperKeys(cfgMap map[string]string) map[string]string {
	newCfgMap := make(map[string]string)
	for k, v := range cfgMap {
		newCfgMap[strings.ToUpper(k)] = v
	}
	return newCfgMap
}

func main() {
	usage := `
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [--out=<type>] [--keys=<key,>] [-n] [-e]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Return specific keys from cog manifest.
  --out=<type>     Configuration output type [default: json].
                   Valid types: json, toml, yaml, dotenv, raw.`

	opts, _ := docopt.ParseArgs(usage, os.Args[1:], "0.3.4")
	var conf struct {
		Gen      bool
		Ctx      string
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
					encHint = "\n--no-enc was called: was it an encrypted value? "
				}
				return nil, fmt.Errorf("--key: [%s] missing from generated config%s", key, encHint)
			}
		}
		return newCfgMap, nil
	}

	switch {
	case conf.Gen:
		cfgMap, err := cogs.Generate(conf.Ctx, conf.File)
		ifErr(err)
		cfgMap, err = filterCfgMap(cfgMap)
		ifErr(err)

		var b []byte
		var output string
		switch conf.Output {
		case "json":
			b, err = json.MarshalIndent(cfgMap, "", "  ")
			output = string(b)
		case "yaml":
			b, err = yaml.Marshal(cfgMap)
			output = string(b)
		case "toml":
			b, err = toml.Marshal(cfgMap)
			output = string(b)
		case "dotenv":
			// convert all key values to uppercase
			output, err = godotenv.Marshal(upperKeys(cfgMap))
		case "raw":
			output = getRawValue(cfgMap)
		default:
			err = fmt.Errorf("invalid arg: --out=" + conf.Output)
		}
		fmt.Fprintln(os.Stdout, string(output))
	}
}
