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

const CogsVersion = "0.5.0"

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

	opts, err := docopt.ParseArgs(usage, os.Args[1:], CogsVersion)
	ifErr(err)
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

	err = opts.Bind(&conf)
	ifErr(err)
	logging.SetLevel(logging.WARNING, "yq")
	cogs.NoEnc = conf.NoEnc
	cogs.EnvSubst = conf.EnvSubst

	// filterCfgMap retains only key names passed to --keys
	filterCfgMap := func(cfgMap map[string]interface{}) (map[string]interface{}, error) {

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
		newCfgMap := make(map[string]interface{})
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
		var format cogs.Format
		var b []byte
		var output string

		if format = cogs.Format(conf.Output); format.Validate() != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("invalid arg: --out="+conf.Output))
			os.Exit(1)
		}

		cfgMap, err := cogs.Generate(conf.Ctx, conf.File, format)
		ifErr(err)

		cfgMap, err = filterCfgMap(cfgMap)
		ifErr(err)

		switch format {
		case cogs.JSON:
			b, err = json.MarshalIndent(cfgMap, "", "  ")
			output = string(b)
		case cogs.YAML:
			b, err = yaml.Marshal(cfgMap)
			output = string(b)
		case cogs.TOML:
			b, err = toml.Marshal(cfgMap)
			output = string(b)
		case cogs.Dotenv:
			// convert all key values to uppercase
			output, err = godotenv.Marshal(upperKeys(cfgMap))
		case cogs.Raw:
			output = getRawValue(cfgMap)
		}
		ifErr(err)

		fmt.Fprintln(os.Stdout, output)
	}
}
