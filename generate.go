package cogs

import (
	"fmt"

	"path"

	"github.com/pelletier/go-toml"
)

// used to represent Cfg k/v pair at the top level of a file
const noSubPath = ""

// Cfg holds all the data needed to generate one string key value pair
type Cfg struct {
	// Defaults to key name unless explicitly declared
	Name  string
	Value string
	Path  string
	// default should be Cfg key name
	SubPath   string
	encrypted bool
	readType  readType
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

type configMap map[string]*Cfg

// Resolver is meant to define an object that returns the final string map to be used in a configuration
// resolving any paths and sub paths defined in the underling config map
type Resolver interface {
	ResolveMap(RawEnv) (map[string]string, error)
	SetName(string)
}

// Gear represents one of the envs in a cog manifest.
// The term "gear" is used to refer to the operating state of a machine (similar
// to how a microservice can operate locally or in a remote environment)
// rather than a gear object. The term "switching gears" is an apt representation
// of how one Cog manifest file can have many environments
type Gear struct {
	Name   string
	cfgMap configMap
	// filepath of file.cog.toml
	filePath string
}

// SetName sets the gear name to the provided string
func (g *Gear) SetName(name string) {
	g.Name = name
}

// ResolveMap outputs the flat associative string, resolving potential filepath pointers
// held by Cfg objects by calling the .ResolveValue() method
func (g *Gear) ResolveMap(env RawEnv) (map[string]string, error) {
	var err error

	g.cfgMap, err = parseEnv(env)
	if err != nil {
		return nil, err
	}

	// includes Cfg objects with a direct file and an empty SubPath:
	// ex: var.path = "./path"
	// ---
	// as well as Cfg objects with SubPaths present:
	// ex: var.path = ["./path", ".subpath"]
	// ---

	type PathGroup struct{
		loadFile func(filePath string) ([]byte, error)
		cfgs []*Cfg
	}
	pathGroups := make(map[string]*PathGroup)

	// 1. sort Cfgs by Path
	for _, cfg := range g.cfgMap {
		if cfg.Path != "" {
			if _, ok := pathGroups[cfg.Path]; !ok {
				loadFileFunc := readFile
				if cfg.encrypted {
					loadFileFunc = decryptFile
				}
				pathGroups[cfg.Path] = &PathGroup{loadFile: loadFileFunc, cfgs: []*Cfg{}}
			}
			pathGroups[cfg.Path].cfgs = append(pathGroups[cfg.Path].cfgs, cfg)
		}
	}

	for path, pathGroup := range pathGroups {
		// 2. for each distinct Path: generate a Reader object
		cfgFilePath := g.getCfgFilePath(path)
		fileBuf, err := pathGroup.loadFile(cfgFilePath)
		if err != nil {
			return nil, err
		}

		// 3. create yaml visitor to handle SubPath strings
		visitor, err := NewYamlVisitor(fileBuf)
		if err != nil {
			return nil, err
		}

		// 4. traverse every Path and possible SubPath retrieving the Cfg.Values associated with it
		for _, cfg := range pathGroup.cfgs {
			err := visitor.SetValue(cfg)
			if err != nil {
				return nil, err
			}

		}
	}

	// final output
	cfgOut := make(map[string]string)
	for cogName, cfg := range g.cfgMap {
		cfgOut[cogName] = cfg.Value
	}

	return cfgOut, nil

}

func (g *Gear) getCfgFilePath(cfgPath string) string {

	if path.IsAbs(cfgPath) {
		return cfgPath
	}
	return path.Join(path.Dir(g.filePath), cfgPath)
}

// RawEnv is meant to represent the topmost untraversed level of a cog environment
type RawEnv map[string]interface{}

// Generate is a top level command that takes an env argument and cogfilepath to return a string map
func Generate(envName, cogFile string) (map[string]string, error) {

	tree, err := toml.LoadFile(cogFile)
	if err != nil {
		return nil, err
	}
	return generate(envName, tree, &Gear{filePath: cogFile})

}

func generate(envName string, tree *toml.Tree, gear Resolver) (map[string]string, error) {
	var ok bool
	var err error

	type rawManifest struct {
		table map[string]RawEnv
	}

	// grab manifest name
	name, ok := tree.Get("name").(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("manifest.name string value must be present as a string")
	}
	tree.Delete("name")

	gear.SetName(name)

	var manifest rawManifest
	if err = tree.Unmarshal(&manifest.table); err != nil {
		return nil, err
	}

	env, ok := manifest.table[envName]
	if !ok {
		return nil, fmt.Errorf("%s environment missing from cog file", envName)
	}

	genOut, err := gear.ResolveMap(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", envName, err)
	}

	return genOut, nil
}

// parseEnv traverses an map interface to populate a gear's configMap
func parseEnv(env RawEnv) (cfgMap configMap, err error) {
	cfgMap = make(configMap)

	err = decodeEncrypted(cfgMap, env)
	if err != nil {
		return nil, err
	}

	err = decodeEnv(cfgMap, env)
	if err != nil {
		return nil, err
	}
	return cfgMap, nil
}

func decodeEnv(cfgMap configMap, env RawEnv) error {
	var err error
	var baseCfg Cfg

	// global path
	if pathValue, ok := env["path"]; ok {
		if err = decodePath(pathValue, &baseCfg, nil); err != nil {
			return err
		}
	}

	// global type
	if rawValue, ok := env["type"]; ok {
		strValue, ok := rawValue.(string)
		if !ok {
			return fmt.Errorf("env.type must be a string value")
		}
		baseCfg.readType = readType(strValue)
		if err := baseCfg.readType.Validate(); err != nil {
			return err
		}
	}

	rawVars, ok := env["vars"]
	if !ok {
		return nil
	}
	vars, ok := rawVars.(map[string]interface{})
	if !ok {
		return fmt.Errorf(".vars must map to a table")
	}

	// check for dubplicate keys for env.vars and env.enc.vars
	for varName, rawVar := range vars {
		if _, ok := cfgMap[varName]; ok {
			return fmt.Errorf("%s: duplicate key present in env and env.enc", varName)
		}
		switch cfgType := rawVar.(type) {
		case string:
			cfgMap[varName] = &Cfg{
				Name:  varName,
				Value: cfgType,
			}
		case map[string]interface{}:
			cfgMap[varName], err = parseCfgMap(varName, &baseCfg, cfgType)
			if err != nil {
				return fmt.Errorf("%s: %s", varName, err)
			}
		default:
			return fmt.Errorf("%s: %s is an unsupported type", varName, cfgType)
		}
	}
	return nil
}

// convenience function for passing env.enc variables to decodeEnv
func decodeEncrypted(cfgMap configMap, env RawEnv) error {
	// treat enc key as a nested configMap
	enc, ok := env["enc"]
	if !ok {
		return nil
	}
	rawEnc, ok := enc.(map[string]interface{})
	if !ok {
		return fmt.Errorf(".enc must map to a table")
	}

	// parse through encrypted variables first
	err := decodeEnv(cfgMap, rawEnc)
	if err != nil {
		return err
	}
	// since env.enc is always called first, mark all output Cfgs as encrypted
	for key, cfg := range cfgMap {
		cfg.encrypted = true
		cfgMap[key] = cfg
	}

	return nil
}

// parseCfg handles the cases when a config value maps to a non string object type
func parseCfgMap(varName string, baseCfg *Cfg, cfgVal map[string]interface{}) (*Cfg, error) {
	var cfg Cfg
	var ok bool

	for k, v := range cfgVal {
		switch k {
		case "name":
			cfg.Name, ok = v.(string)
			if !ok {
				return &cfg, fmt.Errorf(".name must be a string")
			}
		case "path":
			if err := decodePath(v, &cfg, baseCfg); err != nil {
				return nil, fmt.Errorf("%s.path: %s", varName, err)
			}
		case "type":
			cfg.readType = readType(k)
			if err := cfg.readType.Validate(); err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("%s is an unsupported key name", k)
		}

	}
	// if readType was not specified:
	if _, ok := cfgVal["type"]; !ok {
		if baseCfg != nil {
			cfg.readType = baseCfg.readType
		} else {
			cfg.readType = deferred
		}
	}
	// if name is not defined: `var = "value"`
	// then set cfg.Name to the key name, "var" in this case
	if _, ok := cfgVal["name"]; !ok {
		cfg.Name = varName
	}

	return &cfg, nil
}

// decodePath decodes a value of v into a given Cfg pointer
// a path key can map to four valid types:
// 1. path value is a single string mapping to filepath
// 2. path value  is an empty slice, thus baseCfg values will be inherited
// 3. path value  is a two index slice with either index possibly holding an empty slice or string value:
// -  [[], subpath] - path will be inherited from baseCfg if present
// -  [path, []] - subpath will be inherited from baseCfg if present
// -  [path, subpath] - nothing will be inherited as both indices hold strings
func decodePath(v interface{}, cfg *Cfg, baseCfg *Cfg) error {
	var ok bool
	var baseCfgSlice []string
	// map path indices to respective Cfg struct
	if baseCfg != nil {
		baseCfgSlice = []string{baseCfg.Path, baseCfg.SubPath}
	} else {
		baseCfgSlice = []string{"", ""}
	}

	// singular filepath string
	cfg.Path, ok = v.(string)
	if ok {
		return nil
	}
	// cast to interface slice first since v.([]string) fails in one pass
	pathSlice, ok := v.([]interface{})
	if !ok {
		return fmt.Errorf("path must be an array, slice of strings/empty arrays, or an empty array")
	}
	// if path maps to an empty slice: var.path = []
	if len(pathSlice) == 0 && baseCfg != nil {
		cfg.Path = baseCfg.Path
		cfg.SubPath = baseCfg.SubPath
		return nil
	}
	if len(pathSlice) != 2 {
		return fmt.Errorf("path array must have a length of two, providing path and subpath respectively")
	}

	decodedSlice := []string{"", ""}
	for i, v := range pathSlice {
		str, ok := v.(string)
		if ok {
			decodedSlice[i] = str
			continue
		}
		slice, ok := v.([]interface{})
		if !ok {
			return fmt.Errorf("path must be a string or array of strings: %T", slice)
		}
		if len(slice) != 0 {
			return fmt.Errorf("array in path[%d] must be empty", i)
		}
		// inherit the respective path attribute or assign empty string
		decodedSlice[i] = baseCfgSlice[i]

	}
	cfg.Path = decodedSlice[0]
	cfg.SubPath = decodedSlice[1]
	return nil
}
