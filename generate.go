package cogs

import (
	"fmt"
	"os"
	"path"

	"github.com/pelletier/go-toml"
)

// NoEnc decides whether to output encrypted variables or now
var NoEnc bool = false

// EnvSubst decides whether to use environmental substitution or not
var EnvSubst bool = false

// Cfg holds all the data needed to generate one string key value pair
type Cfg struct {
	Name         string      // defaults to key name in cog file unless var.name="other_name" is used
	Value        string      // Cfg.ComplexValue should be nil if Cfg.Value is a non-empty string("")
	ComplexValue interface{} // Cfg.Value should be empty string("") if Cfg.ComplexValue is non-nil
	Path         string      // filepath string where Cfg can be resolved
	SubPath      string      // object traversal string used to resolve Cfg if not at top level of document (yq syntax)
	encrypted    bool        // indicates if decryption is needed to resolve Cfg.Value
	readType     readType
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

// configMap is used by Resolver to output the final k/v associative array
type configMap map[string]*Cfg

// Resolver is meant to define an object that returns the final string map to be used in a configuration
// resolving any paths and sub paths defined in the underling config map
type Resolver interface {
	ResolveMap(RawEnv) (map[string]interface{}, error)
	SetName(string)
}

// Gear represents one of the contexts in a cog manifest.
// The term "gear" is used to refer to the operating state of a machine (similar
// to how a microservice can operate locally or in a remote environment)
// rather than a gear object. The term "switching gears" is an apt representation
// of how one Cog manifest file can have many contexts/environments
type Gear struct {
	Name       string
	cfgMap     configMap
	filePath   string // filepath of file.cog.toml
	outputType Format // desired output type of the marshalled Gear
}

// SetName sets the gear name to the provided string
func (g *Gear) SetName(name string) {
	g.Name = name
}

// ResolveMap outputs the flat associative string, resolving potential filepath pointers
// held by Cfg objects by calling the .ResolveValue() method
func (g *Gear) ResolveMap(env RawEnv) (map[string]interface{}, error) {
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

	type PathGroup struct {
		loadFile func(filePath string) ([]byte, error)
		cfgs     []*Cfg
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

	for p, pGroup := range pathGroups {
		// 2. for each distinct Path: generate a Reader object
		cfgFilePath := g.getCfgFilePath(p)
		fileBuf, err := pGroup.loadFile(cfgFilePath)
		if err != nil {
			return nil, err
		}

		newVisitor := NewYAMLVisitor
		// 3. create visitor to handle SubPath strings
		// all read files should resolve to a yaml.Node, this includes JSON, TOML, and dotenv
		switch FormatForPath(cfgFilePath) {
		case JSON:
			newVisitor = NewJSONVisitor
		case YAML:
			newVisitor = NewYAMLVisitor
		case TOML:
			newVisitor = NewTOMLVisitor
		case Dotenv:
			newVisitor = NewDotenvVisitor
		}
		visitor, err := newVisitor(fileBuf)
		if err != nil {
			return nil, err
		}

		// 4. traverse every Path and possible SubPath retrieving the Cfg.Values associated with it
		for _, cfg := range pGroup.cfgs {
			err := visitor.SetValue(cfg)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", cfg.Name, err)
			}

		}
	}

	// final output
	cfgOut := make(map[string]interface{})
	for cogName, cfg := range g.cfgMap {
		cfgOut[cogName], err = OutputCfg(cfg, g.outputType)
		if err != nil {
			return nil, err
		}
	}

	return cfgOut, nil

}

func (g *Gear) getCfgFilePath(cfgPath string) string {
	if cfgPath == "." {
		return g.filePath
	}
	if path.IsAbs(cfgPath) {
		return cfgPath
	}
	dir, err := os.Getwd()
	if err != nil {
		dir = path.Dir(g.filePath)
	}
	return path.Join(dir, cfgPath)
}

// RawEnv is meant to represent the topmost untraversed level of a cog environment
type RawEnv map[string]interface{}

// Generate is a top level command that takes an env argument and cogfilepath to return a string map
func Generate(envName, cogFile string, outputType Format) (map[string]interface{}, error) {
	var tree *toml.Tree
	var err error

	if err = outputType.Validate(); err != nil {
		return nil, err
	}

	if EnvSubst {
		substFile, err := envSubFile(cogFile)
		if err != nil {
			return nil, err
		}
		tree, err = toml.Load(substFile)
		if err != nil {
			return nil, err
		}
	} else {
		tree, err = toml.LoadFile(cogFile)
		if err != nil {
			return nil, err
		}
	}
	return generate(envName, tree, &Gear{filePath: cogFile, outputType: outputType})

}

func generate(envName string, tree *toml.Tree, gear Resolver) (map[string]interface{}, error) {
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
	if err = tree.Delete("name"); err != nil {
		return nil, err
	}

	gear.SetName(name)

	var manifest rawManifest
	if err = tree.Unmarshal(&manifest.table); err != nil {
		return nil, err
	}

	env, ok := manifest.table[envName]
	if !ok {
		return nil, fmt.Errorf("%s context missing from cog file", envName)
	}

	genOut, err := gear.ResolveMap(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", envName, err)
	}

	return genOut, nil
}

// parseEnv traverses an map interface to populate a gear's configMap
func parseEnv(env RawEnv) (cfgMap configMap, err error) {
	cfgMap = make(configMap)

	// skip fetching encrypted vars if flag is toggled
	if !NoEnc {
		err = decodeEncrypted(cfgMap, env)
		if err != nil {
			return nil, err
		}
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

	// check for duplicate keys for env.vars and env.enc.vars
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
				return fmt.Errorf("%s: %w", varName, err)
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
				return nil, fmt.Errorf(".name must be a string")
			}
		case "path":
			if err := decodePath(v, &cfg, baseCfg); err != nil {
				return nil, fmt.Errorf("%s.path: %w", varName, err)
			}
		case "type":
			rType, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf(".type must be a string")
			}

			cfg.readType = readType(rType)
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
// 4. [path, subpath] - nothing will be inherited as both indices hold strings
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
