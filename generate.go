package cogs

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/pelletier/go-toml"
)

// NoEnc decides whether to output encrypted variables or now
var NoEnc bool = false

// EnvSubst decides whether to use environmental substitution or not
var EnvSubst bool = false

// RecursionLimit is the limit used to define when to abort successive traversals of gears
var RecursionLimit int = 12

// Link holds all the data needed to resolve one string key value pair
type Link struct {
	Name         string      // defaults to key name in cog file unless var.name="other_name" is used
	Value        string      // Link.ComplexValue should be nil if Link.Value is a non-empty string("")
	ComplexValue interface{} // Link.Value should be empty string("") if Link.ComplexValue is non-nil
	Path         string      // filepath string where Link can be resolved
	SubPath      string      // object traversal string used to resolve Link if not at top level of document (yq syntax)
	encrypted    bool        // indicates if decryption is needed to resolve Link.Value
	remote       bool        // indicates if an HTTP request is needed to return the given document
	header       http.Header // HTTP request headers
	keys         []string    // key filter for Gear read types
	readType     readType
}

// String holds the string representation of a Link struct
func (c Link) String() string {
	return fmt.Sprintf(`Link{
	Name: %s
	Value: %s
	Path: %s
	SubPath: %s
	encrypted: %t
}`, c.Name, c.Value, c.Path, c.SubPath, c.encrypted)
}

// LinkMap is used by Resolver to output the final k/v associative array
type LinkMap map[string]*Link

// CfgMap is meant to represent a map with values of one or more unknown types
type CfgMap map[string]interface{}

// EnvFilter if a function meant to filter a CfgMap
type EnvFilter func(CfgMap) (CfgMap, error)

// Resolver is meant to define an object that returns the final string map to be used in a configuration
// resolving any paths and sub paths defined in the underling config map
type Resolver interface {
	ResolveMap(CfgMap) (CfgMap, error)
	SetName(string)
}

// Gear represents one of the contexts in a cog manifest.
// The term "gear" is used to refer to the operating state of a machine (similar
// to how a microservice can operate locally or in a remote environment)
// rather than a gear object. The term "switching gears" is an apt representation
// of how one Cog manifest file can have many contexts/environments
type Gear struct {
	Name       string
	linkMap    LinkMap
	filePath   string     // filepath of file.cog.toml
	fileValue  []byte     // byte representation of TOML file
	tree       *toml.Tree // TOML object tree
	outputType Format     // desired output type of the marshalled Gear
	recursions uint       // the amount of recursions for the current Gear
	filter     EnvFilter
}

// SetName sets the gear name to the provided string
func (g *Gear) SetName(name string) {
	g.Name = name
}

// ResolveMap outputs the flat associative string, resolving potential filepath pointers
// held by Link objects by calling the .ResolveValue() method
func (g *Gear) ResolveMap(env CfgMap) (CfgMap, error) {
	var err error

	if g.filter != nil {
		if env, err = g.filter(env); err != nil {
			return nil, err
		}
	}

	g.linkMap, err = parseEnv(env)
	if err != nil {
		return nil, err
	}

	// includes Link objects with a direct file and an empty SubPath:
	// ex: var.path = "./path"
	// ---
	// as well as Link objects with SubPaths present:
	// ex: var.path = ["./path", ".subpath"]
	// ---

	type PathGroup struct {
		loadFile func(filePath string) ([]byte, error)
		links    []*Link
	}
	pathGroups := make(map[string]*PathGroup)

	// 1. sort Links by Path
	for _, link := range g.linkMap {
		if link.Path != "" {
			if _, ok := pathGroups[link.Path]; !ok {
				// read plaintext file into bytes
				loadFileFunc := readFile
				// check the path string is a valid URL
				if link.remote = isValidUrl(link.Path); link.remote {
					// cheat to fulfill PathGroup interface
					loadFileFunc = func(path string) ([]byte, error) {
						return getHTTPFile(path, link.header)
					}
				}
				if link.encrypted && link.remote {
					panic("remote encrypted files not supported at this time")
				}
				// read encrypted file into bytes
				if link.encrypted {
					loadFileFunc = decryptFile
				}
				pathGroups[link.Path] = &PathGroup{loadFile: loadFileFunc, links: []*Link{}}
			}
			pathGroups[link.Path].links = append(pathGroups[link.Path].links, link)
		}
	}

	for p, pGroup := range pathGroups {
		var fileBuf []byte
		// 2. for each distinct Path: generate a Reader object
		linkFilePath := g.getLinkFilePath(p)
		// if link.Path references the cog file, return the already read (and envsubst applied) value
		if p == selfPath {
			fileBuf = g.fileValue
		} else if fileBuf, err = pGroup.loadFile(linkFilePath); err != nil {
			return nil, err
		}

		newVisitor := NewYAMLVisitor
		// 3. create visitor to handle SubPath strings
		// all read files should resolve to a yaml.Node, this includes JSON, TOML, and dotenv
		switch FormatForPath(linkFilePath) {
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

		// 4. traverse every Path and possible SubPath retrieving the Link.Values associated with it
		for _, link := range pGroup.links {
			err := visitor.SetValue(link)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", link.Name, err)
			}

		}
	}

	// final output
	cfgOut := make(CfgMap)
	for cogName, link := range g.linkMap {
		cfgOut[cogName], err = OutputCfg(link, g.outputType)
		if err != nil {
			return nil, err
		}
	}

	return cfgOut, nil

}

func (g *Gear) getLinkFilePath(linkPath string) string {
	if linkPath == selfPath {
		return g.filePath
	}
	if path.IsAbs(linkPath) || isValidUrl(linkPath) {
		return linkPath
	}
	dir, err := os.Getwd()
	if err != nil {
		dir = path.Dir(g.filePath)
	}
	return path.Join(dir, linkPath)
}

// Generate is a top level command that takes an env argument and cogfilepath to return a string map
func Generate(ctxName, cogPath string, outputType Format, filter EnvFilter) (CfgMap, error) {
	var tree *toml.Tree
	var err error

	if err = outputType.Validate(); err != nil {
		return nil, err
	}

	b, err := readFile(cogPath)
	if err != nil {
		return nil, err
	}

	if EnvSubst {
		if b, err = envSubBytes(b); err != nil {
			return nil, err
		}
	}
	if tree, err = toml.LoadBytes(b); err != nil {
		return nil, err
	}
	gear := &Gear{
		filePath:   cogPath,
		fileValue:  b,
		tree:       tree,
		outputType: outputType,
		recursions: 0,
		filter:     filter,
	}
	return generate(ctxName, tree, gear)

}

func generate(ctxName string, tree *toml.Tree, gear Resolver) (CfgMap, error) {
	var err error

	type rawManifest struct {
		table map[string]CfgMap
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

	env, ok := manifest.table[ctxName]
	if !ok {
		return nil, fmt.Errorf("%s context missing from cog file", ctxName)
	}

	genOut, err := gear.ResolveMap(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ctxName, err)
	}

	return genOut, nil
}

// parseEnv traverses an map interface to populate a gear's configMap
func parseEnv(env CfgMap) (linkMap LinkMap, err error) {
	linkMap = make(map[string]*Link)

	// skip fetching encrypted vars if flag is toggled
	if !NoEnc {
		err = decodeEncrypted(linkMap, env)
		if err != nil {
			return nil, err
		}
	}

	err = decodeEnv(linkMap, env)
	if err != nil {
		return nil, err
	}
	return linkMap, nil
}

func decodeEnv(linkMap LinkMap, env CfgMap) error {
	var err error
	var baseLink Link

	// global path
	if pathValue, ok := env["path"]; ok {
		if err = decodePath(pathValue, &baseLink, nil); err != nil {
			return err
		}
	}

	// global type
	if rawValue, ok := env["type"]; ok {
		strValue, ok := rawValue.(string)
		if !ok {
			return fmt.Errorf("env.type must be a string value")
		}
		baseLink.readType = readType(strValue)
		if err := baseLink.readType.Validate(); err != nil {
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

	// check for duplicate keys for ctx.vars and ctx.enc.vars
	for varName, rawCfg := range vars {
		if _, ok := linkMap[varName]; ok {
			return fmt.Errorf("%s: duplicate key present in ctx and ctx.enc", varName)
		}
		switch cfgMap := rawCfg.(type) {
		case string:
			linkMap[varName] = &Link{
				Name:  varName,
				Value: cfgMap,
			}
		case map[string]interface{}:
			linkMap[varName], err = parseLinkMap(varName, &baseLink, cfgMap)
			if err != nil {
				return fmt.Errorf("%s: %w", varName, err)
			}
		default:
			return fmt.Errorf("%s: %s is an unsupported type", varName, cfgMap)
		}
	}
	return nil
}

// convenience function for passing env.enc variables to decodeEnv
func decodeEncrypted(linkMap LinkMap, env CfgMap) error {
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
	err := decodeEnv(linkMap, rawEnc)
	if err != nil {
		return err
	}
	// since env.enc is always called first, mark all output Links as encrypted
	for key, link := range linkMap {
		link.encrypted = true
		linkMap[key] = link
	}

	return nil
}

// parseLink handles the cases when a config value maps to a non string object type
func parseLinkMap(varName string, baseLink *Link, cfgMap CfgMap) (*Link, error) {
	var link Link
	var ok bool

	for k, v := range cfgMap {
		switch k {
		case "name":
			if link.Name, ok = v.(string); !ok {
				return nil, fmt.Errorf("%s.name must be a string", varName)
			}
		case "path":
			if err := decodePath(v, &link, baseLink); err != nil {
				return nil, fmt.Errorf("%s.path: %w", varName, err)
			}
		case "type":
			rType, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s.type must be a string", varName)
			}

			link.readType = readType(rType)
			if err := link.readType.Validate(); err != nil {
				return nil, fmt.Errorf("%s.type: %w", varName, err)
			}
		case "gear_keys":
			panic("rGear unsupported at this time")
			keysErr := fmt.Errorf("%s.keys must be a string or array of strings", varName)
			link.keys = []string{}
			slice, ok := v.([]interface{})
			if !ok {
				return nil, keysErr
			}
			for _, v := range slice {
				str, ok := v.(string)
				if !ok {
					return nil, keysErr
				}
				link.keys = append(link.keys, str)

			}
		case "header": // "net/http".Header is of type Header map[string][]string
			link.header = make(http.Header)
			headerErr := fmt.Errorf("%s.header must map to a string or array of strings", varName)
			header, ok := v.(map[string]interface{}) // handle single string value header
			if !ok {
				return nil, headerErr
			}
			for headerK, headerV := range header {
				switch vType := headerV.(type) {
				case string:
					link.header[headerK] = append(link.header[headerK], vType)
				case []interface{}: // go is unable to check for headerV.([]string)
					for _, el := range vType {
						vStr, ok := el.(string)
						if !ok {
							return nil, headerErr
						}
						link.header[headerK] = append(link.header[headerK], vStr)

					}
				}
			}
		default:
			return nil, fmt.Errorf("%s.%s is an unsupported key name", varName, k)
		}

	}
	// if readType was not specified:
	if _, ok := cfgMap["type"]; !ok {
		if baseLink != nil {
			link.readType = baseLink.readType
		} else {
			link.readType = deferred
		}
	}
	// if name is not defined: `var = "value"`
	// then set link.Name to the key name, "var" in this case
	if _, ok := cfgMap["name"]; !ok {
		link.Name = varName
	}

	return &link, nil
}

// decodePath decodes a value of v into a given Link pointer
// a path key can map to four valid types:
// 1. path value is a single string mapping to filepath
// 2. path value  is an empty slice, thus baseLink values will be inherited
// 3. path value  is a two index slice with either index possibly holding an empty slice or string value:
// -  [[], subpath] - path will be inherited from baseLink if present
// -  [path, []] - subpath will be inherited from baseLink if present
// 4. [path, subpath] - nothing will be inherited as both indices hold strings
func decodePath(v interface{}, link *Link, baseLink *Link) error {
	var ok bool
	var baseLinkSlice []string
	// map path indices to respective Link struct
	if baseLink != nil {
		baseLinkSlice = []string{baseLink.Path, baseLink.SubPath}
	} else {
		baseLinkSlice = []string{"", ""}
	}

	// singular filepath string
	link.Path, ok = v.(string)
	if ok {
		return nil
	}
	// cast to interface slice first since v.([]string) fails in one pass
	pathSlice, ok := v.([]interface{})
	if !ok {
		return fmt.Errorf("path must be a string, array of strings/empty arrays, or an empty array")
	}
	// if path maps to an empty slice: var.path = []
	if len(pathSlice) == 0 && baseLink != nil {
		link.Path = baseLink.Path
		link.SubPath = baseLink.SubPath
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
		decodedSlice[i] = baseLinkSlice[i]

	}
	link.Path = decodedSlice[0]
	link.SubPath = decodedSlice[1]
	return nil
}
