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

func main() {
	usage := `
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [--out=<type>] [--keys=<key,>] [--not=<key,>] [-n] [-e]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Include specific keys, comma separated.
  --not=<key,>     Exclude specific keys, comma separated.
  --out=<type>     Configuration output type [default: json].
                   Valid types: json, toml, yaml, dotenv, raw.`

	opts, _ := docopt.ParseArgs(usage, os.Args[1:], "0.4.1")
	var conf struct {
		Gen      bool
		Ctx      string
		File     string `docopt:"<cog-file>"`
		Output   string `docopt:"--out"`
		Keys     string
		Not      string
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

		// --not runs before --keys!
		// make sure to avoid --not=key_name --key=key_name, ya dingus!
		var notList []string
		if conf.Not != "" {
			notList = strings.Split(conf.Not, ",")
			cfgMap = exclude(notList, cfgMap)
		}
		if conf.Keys == "" {
			return cfgMap, nil
		}

		keyList := strings.Split(conf.Keys, ",")
		newCfgMap := make(map[string]string)
		for _, key := range keyList {
			var ok bool
			newCfgMap[key], ok = cfgMap[key]
			if !ok {
				notHint := ""
				if inList(key, notList) {
					notHint = fmt.Sprintf("\n\n--not=%s and --keys=%s were called\n"+
						"avoid trying to include and exclude the same value, ya dingus!", key, key)
				}
				encHint := ""
				if conf.NoEnc {
					encHint = "\n\n--no-enc was called: was it an encrypted value?\n"
				}
				return nil, fmt.Errorf("--key: [%s] missing from generated config%s%s", key, encHint, notHint)
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
		ifErr(err)
		fmt.Fprintln(os.Stdout, string(output))
	}
}
