package main

import (
	"encoding/json"
	"fmt"
	"github.com/Bestowinc/cogs"
	"github.com/docopt/docopt-go"
	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml"
	"github.com/stoewer/go-strcase"
	logging "gopkg.in/op/go-logging.v1"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

const cogsVersion = "0.9.1"
const usage string = `
COGS COnfiguration manaGement S

Usage:
  cogs gen <cog-file> <ctx>... [options]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --no-decrypt	   Skips decrypting encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Include specific keys, comma separated.
  --not=<key,>     Exclude specific keys, comma separated.
  --out=<type>     Configuration output type [default: json].
                   <type>: json, toml, yaml, dotenv, raw.
  
  --export, -x     If --out=dotenv: Prepends "export " to each line.
  --preserve, -p   If --out=dotenv: Preserves variable casing.
  --sep=<sep>      If --out=raw:    Delimits values with a <sep>arator.
 `

// Conf is used to bind CLI arguments and options
type Conf struct {
	Gen       bool
	Ctx       []string
	File      string `docopt:"<cog-file>"`
	Output    string `docopt:"--out"`
	Keys      string
	Not       string
	NoEnc     bool
	NoDecrypt bool
	Raw       bool
	EnvSubst  bool `docopt:"--envsubst"`
	Export    bool
	Preserve  bool
	Delimiter string `docopt:"--sep"`
}

var conf Conf

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// run handles the main logic in parsing the CLI arguments
func run() error {

	opts, err := docopt.ParseArgs(usage, os.Args[1:], cogsVersion)
	if err != nil {
		return err
	}

	if err = opts.Bind(&conf); err != nil {
		return err
	}
	// this is the logger used by yq, set it to warning to hide trace and debug data
	logging.SetLevel(logging.WARNING, "")
	cogs.NoEnc = conf.NoEnc
	cogs.EnvSubst = conf.EnvSubst
	cogs.NoDecrypt = conf.NoDecrypt

	if cogs.NoDecrypt && cogs.NoEnc {
		return cogs.ErrNoEncAndNoDecrypt
	}

	switch {
	case conf.Gen:
		var b []byte
		var output string
		var cfgs []*cogs.CfgMap
		var cfgMap cogs.CfgMap

		format, err := conf.validate()
		if err != nil {
			return err
		}

		for _, ctx := range conf.Ctx {
			cfg, err := cogs.Generate(ctx, conf.File, format, conf.filterLinks)
			if err != nil {
				return err
			}
			cfgs = append(cfgs, &cfg)
		}
		// Dotenv Join should be done once modFn changes key names so that
		// keyValue and key_value can be marked as duplicates of KEY_VALUE
		if format != cogs.Dotenv {
			if cfgMap, err = cogs.Join(cfgs...); err != nil {
				return err
			}
		}

		switch format {
		case cogs.JSON:
			b, err = json.MarshalIndent(cfgMap, "", "  ")
			output = string(b) + "\n"
		case cogs.YAML:
			b, err = yaml.Marshal(cfgMap)
			output = string(b)
		case cogs.TOML:
			b, err = toml.Marshal(cfgMap)
			output = string(b)
		case cogs.Dotenv:
			var modFn []func(string) string
			// if --preserve was called, do not convert variable names to uppercase
			if !conf.Preserve {
				modFn = append(modFn, strcase.UpperSnakeCase)
			}
			// if --export was called, prepend "export " to key name
			if conf.Export {
				modFn = append(modFn, func(k string) string { return "export " + k })
			}
			for _, cfg := range cfgs {
				*cfg = modKeys(*cfg, modFn...)
			}
			if cfgMap, err = cogs.Join(cfgs...); err != nil {
				return err
			}

			output, err = godotenv.Marshal(toStringMap(cfgMap))
			output = output + "\n"
		case cogs.Raw:
			keyList := []string{}
			if conf.Keys != "" {
				keyList = strings.Split(conf.Keys, ",")
			}
			output, err = getRawValue(cfgMap, keyList, conf.Delimiter)
		}
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stdout, output)
	}

	return nil
}
