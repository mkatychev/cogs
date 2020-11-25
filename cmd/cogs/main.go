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

const cogsVersion = "0.5.0"
const usage string = `
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [options]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Include specific keys, comma separated.
  --not=<key,>     Exclude specific keys, comma separated.
  --out=<type>     Configuration output type [default: json].
                   <type>: json, toml, yaml, dotenv, raw.
  
  --export, -x     If --out=dotenv: Prepends "export " to each line.
  --preserve, -p   If --out=dotenv: Preserves variable casing.
  --sep=<sep>      If --out=raw:    Delimits values with a <sep>arator.
 `

// Conf is used to bind CLI agruments and options
type Conf struct {
	Gen       bool
	Ctx       string
	File      string `docopt:"<cog-file>"`
	Output    string `docopt:"--out"`
	Keys      string
	Not       string
	NoEnc     bool
	Raw       bool
	EnvSubst  bool `docopt:"--envsubst"`
	Export    bool
	Preserve  bool
	Delimiter string `docopt:"--sep"`
}

var conf Conf

func main() {

	opts, err := docopt.ParseArgs(usage, os.Args[1:], cogsVersion)
	ifErr(err)

	err = opts.Bind(&conf)
	ifErr(err)
	logging.SetLevel(logging.WARNING, "yq")
	cogs.NoEnc = conf.NoEnc
	cogs.EnvSubst = conf.EnvSubst

	switch {
	case conf.Gen:
		var b []byte
		var output string

		format, err := conf.validate()
		ifErr(err)

		cfgMap, err := cogs.Generate(conf.Ctx, conf.File, format)
		ifErr(err)

		cfgMap, err = conf.filterCfgMap(cfgMap)
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
			var modFuncs []func(string) string
			// if --preserve was called, do not convert variable names to uppercase
			if !conf.Preserve {
				modFuncs = append(modFuncs, strings.ToUpper)
			}
			// if --export was called, prepend "export " to key name
			if conf.Export {
				modFuncs = append(modFuncs, func(k string) string { return "export " + k })
			}
			// convert all key values to uppercase
			output, err = godotenv.Marshal(modKeys(cfgMap, modFuncs...))
		case cogs.Raw:
			output = getRawValue(cfgMap, conf.Delimiter)
		}
		ifErr(err)

		fmt.Fprintln(os.Stdout, output)
	}
}
