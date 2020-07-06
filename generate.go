package cogs

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

// Cfg holds all the data needed to generate one string key value pair
type Cfg struct {
	// Defaults to key name unless explicitly declared
	Name  string
	Value string
	Path  string
	// default should be Cfg key name
	SubPath   string
	encrypted bool
}

// GenerateValue returns the value corresponding to a Cfg struct
// if Path resolves to a valid file the file byte value
// is passed to a file reader object, attempting to serialize the contents of
// the file if type is supported
func (c Cfg) GenerateValue() string {
	// if Path is empty or Value is non empty
	if c.Path == "" || c.Value != "" {
		return c.Value
	}

	if c.encrypted {
		// TODO COGS-1657
		// decrypt.File(c.Path, c.SubPath)
		return "|enc|" + c.Path
	}
	// TODO COGS-1659
	// cogs.File(c.Path, c.SubPath)
	return "|path|" + c.Path + "|subpath|" + c.SubPath

}

// String holds the string representation of a Cfg struct
func (c Cfg) String() string {
	return fmt.Sprintf(`Cfg{
	Name: %s
	Value: %s
	Path: %s
	SubPath: %s
	encrypted: %t
}`, c.Name, c.Value, c.Path, c.SubPath, c.encrypted)
}

type configMap map[string]Cfg

// Gear represents one of the envs in a cog manifest.
// The term "gear" is used to refer to the operating state of a machine (similar
// to how a microservice can operate locally or in a remote environment)
// rather than a gear object. The term "switching gears" is an apt representation
// of how one Cog manifest file can have many environments
type Gear struct {
	Name   string
	cfgMap configMap
}

// GenerateMap outputs the flat associative string, resolving potential filepath pointers
// held by Cfg objects by calling the .GenerateValue() method
func (g *Gear) GenerateMap() map[string]string {
	cfgMap := make(map[string]string)
	for k, cfg := range g.cfgMap {
		cfgMap[k] = cfg.GenerateValue()
	}
	return cfgMap

}

type rawManifest struct {
	table map[string]rawEnv
}

type rawEnv map[string]interface{}

// Generate is a top level command that takes an env argument and cogfilepath to return a string map
func Generate(env, cogFile string) (map[string]string, error) {

	tree, err := toml.LoadFile(cogFile)
	if err != nil {
		return nil, err
	}
	return generate(env, tree)

}

func generate(envName string, tree *toml.Tree) (map[string]string, error) {
	var gear Gear
	var ok bool
	var err error

	// grab manifest name
	gear.Name, ok = tree.Get("name").(string)
	if !ok || gear.Name == "" {
		return nil, fmt.Errorf("manifest.name string value must be present as a string")
	}
	tree.Delete("name")

	var manifest rawManifest
	if err = tree.Unmarshal(&manifest.table); err != nil {
		return nil, err
	}

	env, ok := manifest.table[envName]
	if !ok {
		return nil, fmt.Errorf("%s environment missing from cog file", envName)
	}

	err = gear.parseEnv(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", envName, err)
	}

	return gear.GenerateMap(), nil
}

// parseEnv traverses an map interface to populate a gear's configMap
func (g *Gear) parseEnv(env rawEnv) (err error) {

	g.cfgMap = make(configMap)

	// treat enc key as a nested g.config
	if enc, ok := env["enc"]; ok {
		rawEnc, ok := enc.(map[string]interface{})
		if !ok {
			return fmt.Errorf(".enc must map to a table")
		}

		g.cfgMap, err = parseEnv(g.cfgMap, rawEnc)
		if err != nil {
			return err
		}
		for key, cfg := range g.cfgMap {
			cfg.encrypted = true
			g.cfgMap[key] = cfg
		}
		// remove env map now that it is parsed
		delete(env, "enc")
	}
	g.cfgMap, err = parseEnv(g.cfgMap, env)
	if err != nil {
		return err
	}
	return nil
}

func parseEnv(cfgMap configMap, env rawEnv) (configMap, error) {
	var err error

	for k, rawCfg := range env {
		if _, ok := cfgMap[k]; ok {
			return nil, fmt.Errorf("%s: duplicate key present in env and env.enc", k)
		}
		switch t := rawCfg.(type) {
		case string:
			cfgMap[k] = Cfg{
				Name:  k,
				Value: t,
			}
		case map[string]interface{}:
			cfgMap[k], err = parseCfg(t)
			if err != nil {
				return nil, fmt.Errorf("%s: %s", k, err)
			}
		default:
			return nil, fmt.Errorf("%s: %s is an unsupported type", k, t)
		}
	}
	return cfgMap, nil
}

// parseCfg handles the cases when a config key maps to a non string object type
func parseCfg(cfgVal map[string]interface{}) (Cfg, error) {
	var cfg Cfg
	var ok bool

	for k, v := range cfgVal {
		switch k {
		case "name":
			cfg.Name, ok = v.(string)
			if !ok {
				return cfg, fmt.Errorf(".name must be a string")
			}
		case "path":
			// a path key can map to two valid types:
			// 1. path value is a single string mapping to filepath
			// 2. path value  is a two index slice mapping to [filepath, subpath] respectively

			// singular filepath string
			cfg.Path, ok = v.(string)
			if ok {
				continue
			}
			// cast to interface slice first since v.([]string) fails in one pass
			pathSlice, ok := v.([]interface{})
			if !ok {
				return cfg, fmt.Errorf("path must be a string or array of strings")
			}
			if len(pathSlice) != 2 {
				return cfg, fmt.Errorf("path array must only contain two values mapping to path and subpath respectively")
			}
			// filepath string
			cfg.Path, ok = pathSlice[0].(string)
			if !ok {
				return cfg, fmt.Errorf("path must be a string or array of strings")
			}

			// subpath string index used to traverse the data object once deserialized
			cfg.SubPath, ok = pathSlice[1].(string)
			if !ok {
				return cfg, fmt.Errorf("path must be a string or array of strings")
			}

		default:
			return cfg, fmt.Errorf("%s is an unsupported key name", k)
		}
	}
	return cfg, nil
}
