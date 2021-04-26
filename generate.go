package cogs

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// NoEnc decides whether to handle encrypted variables
var NoEnc bool = false

// NoDecrypt decides whether to decrypt encrypted values, not compatible with NoEnc
var NoDecrypt bool = false

// EnvSubst decides whether to use environmental substitution or not
var EnvSubst bool = false

// RecursionLimit is the limit used to define when to abort successive traversals of gears
var RecursionLimit int = 12

// distinctPath is used to separate k/v pairs that share the same URL path but
// with differing bodies/headers/methods
type distinctPath struct {
	path   string
	header string
	method string
	body   string
}

// Link holds all the data needed to resolve one string key value pair
type Link struct {
	KeyName    string      // the key name defined in the context file
	SearchName string      // same as keyName unless redefined using the `name` key: var.name="other_name"
	Value      interface{} // Holds a complex or simple value for the given Link
	Path       string      // filepath string where Link can be resolved
	SubPath    string      // object traversal string used to resolve Link if not at top level of document (yq syntax)
	encrypted  bool        // indicates if decryption is needed to resolve Link.Value
	remote     bool        // indicates if an HTTP request is needed to return the given document
	header     http.Header // HTTP request headers
	method     string      // HTTP request method
	body       string      // HTTP request body
	keys       []string    // key filter for Gear read types
	readType   ReadType
}

// distinctPath returns the Link properties needed to differentiate Links with identical paths
// but differing HTTP properties
func (c Link) distinctPath() distinctPath {
	header := ""
	// NOTE starting with Go 1.12, the fmt package prints maps in key-sorted order to ease testing.
	// https://golang.org/doc/go1.12#fmt
	if c.header != nil {
		header = fmt.Sprintf("%v", c.header)
	}

	return distinctPath{
		path:   c.Path,
		header: header,
		method: c.method,
		body:   c.body,
	}
}

// String holds the string representation of a Link struct
func (c Link) String() string {
	return fmt.Sprintf(`Link{
	KeyName: %s
	SearchName: %s
	Value: %s
	Path: %s
	SubPath: %s
	encrypted: %t
}`, c.KeyName, c.SearchName, c.Value, c.Path, c.SubPath, c.encrypted)
}

// LinkMap is used by Resolver to output the final k/v associative array
type LinkMap map[string]*Link

// CfgMap is meant to represent a map with values of one or more unknown types
type CfgMap map[string]interface{}

// LinkFilter if a function meant to filter a LinkMap
type LinkFilter func(LinkMap) (LinkMap, error)

// Resolver is meant to define an object that returns the final string map to be used in a configuration
// resolving any paths and sub paths defined in the underling config map
type Resolver interface {
	ResolveMap(baseContext) (CfgMap, error)
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
	filter     LinkFilter
}

// SetName sets the gear name to the provided string
func (g *Gear) SetName(name string) {
	g.Name = name
}

// ResolveMap outputs the flat associative string, resolving potential filepath pointers
// held by Link objects by calling the .SetValue() method
func (g *Gear) ResolveMap(ctx baseContext) (CfgMap, error) {
	var err error

	if g.linkMap, err = parseCtx(ctx); err != nil {
		return nil, err
	}
	if g.linkMap, err = g.filter(g.linkMap); err != nil {
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

	pathGroups := make(map[distinctPath]*PathGroup)

	// 1. sort Links by Path
	for _, link := range g.linkMap {
		if link.Path == "" {
			continue
		}

		if _, ok := pathGroups[link.distinctPath()]; !ok {
			// read plaintext file into bytes
			loadFile := readFile
			switch {
			case link.remote:
				// must explicitly define variables
				// or previous link values will bleed into loadFile func
				header := link.header
				method := link.method
				body := link.body

				if link.encrypted {
					loadFile = func(path string) ([]byte, error) {
						return decryptHTTPFile(path, header, method, body)
					}
				} else {
					loadFile = func(path string) ([]byte, error) {
						return getHTTPFile(path, header, method, body)
					}
				}
			case link.encrypted:
				loadFile = decryptFile
			}
			pathGroups[link.distinctPath()] = &PathGroup{loadFile: loadFile, links: []*Link{}}
		}
		pathGroups[link.distinctPath()].links = append(pathGroups[link.distinctPath()].links, link)
	}

	var errs error
	for p, pGroup := range pathGroups {
		var fileBuf []byte
		// 2. for each distinct Path: generate a Reader object
		linkFilePath := g.getLinkFilePath(p.path)
		// if link.Path references the cog file, return the already read (and envsubst applied) value
		if p.path == selfPath {
			fileBuf = g.fileValue
		} else if fileBuf, err = pGroup.loadFile(linkFilePath); err != nil {
			if os.IsNotExist(err) {
				errs = multierr.Append(errs, err)
				continue
			}
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
			if err := visitor.SetValue(link); err != nil {
				return nil, errors.Wrap(err, link.KeyName)
			}

		}

		// 5. add missing links to errs
		if viErrs := visitor.Errors(); viErrs != nil {
			errs = multierr.Append(errs, multierr.Combine(viErrs...))
		}
	}

	// The returned error formats into a readable multi-line error message if formatted with %+v.
	if errs != nil {
		return nil, fmt.Errorf("%+v", errs)
	}

	// final output
	cfgOut := make(CfgMap)
	for key, link := range g.linkMap {
		cfgOut[key], err = OutputCfg(link, g.outputType)
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
	if path.IsAbs(linkPath) || isValidURL(linkPath) {
		return linkPath
	}
	dir, err := os.Getwd()
	if err != nil {
		dir = path.Dir(g.filePath)
	}
	return path.Join(dir, linkPath)
}

// Generate is a top level command that takes an context name argument and cog file path to return a string map
func Generate(ctxName, cogPath string, outputType Format, filter LinkFilter) (CfgMap, error) {
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
	var ctx baseContext

	name, ok := tree.Get("name").(string)
	if !ok {
		return nil, fmt.Errorf("manifest.name string value must be present as a non-empty string")
	}
	gear.SetName(name)

	ctxTree, ok := tree.Get(ctxName).(*toml.Tree)
	if !ok {
		// TODO  ErrMissingContext = errorW{fmt:"%s: %s context missing from cog file"}
		errMsg := fmt.Sprintf("%s context missing from cog file", ctxName)
		if g, ok := gear.(*Gear); ok {
			errMsg = g.filePath + ": " + errMsg
		}
		return nil, errors.New(errMsg)
	}

	var ctxMap map[string]interface{}
	if err := ctxTree.Unmarshal(&ctxMap); err != nil {
		return nil, err
	}

	if err = mapstructure.Decode(ctxMap, &ctx); err != nil {
		return nil, fmt.Errorf("generate context: %w", err)
	}

	genOut, err := gear.ResolveMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ctxName, err)
	}

	return genOut, nil
}

// parseCtx traverses an map interface to populate a gear's configMap
func parseCtx(ctx baseContext) (linkMap LinkMap, err error) {
	linkMap = make(map[string]*Link)

	// skip fetching encrypted vars if flag is toggled
	if !NoEnc {
		err = decodeEncVars(linkMap, ctx.Enc)
		if err != nil {
			return nil, err
		}
	}

	err = decodeVars(linkMap, ctx.toContext())
	if err != nil {
		return nil, err
	}
	return linkMap, nil
}

// baseContext is the struct that maps to the TOML table's ctx name
type baseContext struct {
	Path     interface{} `mapstructure:",omitempty"`
	ReadType string      `mapstructure:"type,omitempty"`
	Name     string      `mapstructure:",omitempty"`
	Vars     CfgMap      `mapstructure:",omitempty"`
	Enc      context     `mapstructure:",omitempty"`
	Header   interface{} `mapstructure:",omitempty"`
	Method   string      `mapstructure:",omitempty"`
	Body     string      `mapstructure:",omitempty"`
}

// toContext returns the unencrypted context properties ignoring baseContext.Enc
func (b baseContext) toContext() context {
	return context{
		Path:     b.Path,
		ReadType: b.ReadType,
		Name:     b.Name,
		Vars:     b.Vars,
		Header:   b.Header,
		Method:   b.Method,
		Body:     b.Body,
	}
}

// context is a struct meant to represent both encrypted and plaintext sections of a baseContext
type context struct {
	Path     interface{} `mapstructure:",omitempty"`
	ReadType string      `mapstructure:"type,omitempty"`
	Name     string      `mapstructure:",omitempty"`
	Vars     CfgMap      `mapstructure:",omitempty"`
	Header   interface{} `mapstructure:",omitempty"`
	Method   string      `mapstructure:",omitempty"`
	Body     string      `mapstructure:",omitempty"`
}

func decodeVars(linkMap LinkMap, ctx context) error {
	var err error
	var baseLink Link // any readType or Path declarations to be inherited by Links

	// global path
	if ctx.Path != nil {
		if err = decodePath(ctx.Path, &baseLink, nil); err != nil {
			return err
		}
	}

	// baseContext globals
	// -------------------
	// name
	baseLink.SearchName = ctx.Name
	// type
	baseLink.readType = ReadType(ctx.ReadType)
	if err := baseLink.readType.Validate(); err != nil {
		return err
	}
	// HTTP header
	if ctx.Header != nil {
		if baseLink.header, err = parseHeader(ctx.Header); err != nil {
			return errors.Wrap(err, ctx.Name)
		}
	}
	// HTTP method
	baseLink.method = ctx.Method
	// HTTP body
	baseLink.body = ctx.Body
	// -------------------

	// check for duplicate keys for ctx.vars and ctx.enc.vars
	for k, v := range ctx.Vars {
		if _, ok := linkMap[k]; ok {
			return fmt.Errorf("%s: duplicate key present in ctx and ctx.enc", k)
		} else if IsSimpleValue(v) {
			linkMap[k] = &Link{
				KeyName: k,
				Value:   v,
			}
		} else if cfgMap, ok := v.(map[string]interface{}); ok {
			if linkMap[k], err = parseLinkMap(k, &baseLink, cfgMap); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		} else {
			return fmt.Errorf("%s: %T is an unsupported type", k, v)
		}
	}
	return nil
}

// convenience function for passing ctx.enc variables to decodeEnv
func decodeEncVars(linkMap LinkMap, ctx context) error {
	err := decodeVars(linkMap, ctx)
	if err != nil {
		return fmt.Errorf("decodeEncVars: %w", err)
	}
	// since ctx.enc should always be called first, mark all output Links as encrypted
	if !NoDecrypt {
		for key, link := range linkMap {
			link.encrypted = true
			linkMap[key] = link
		}
	}

	return nil
}

// parseLink handles the cases when a config value maps to a non string object type
func parseLinkMap(varName string, baseLink *Link, cfgMap CfgMap) (*Link, error) {
	var link Link
	var ok bool
	var err error

	for k, v := range cfgMap {
		switch k {
		case "name":
			if link.SearchName, ok = v.(string); !ok {
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

			link.readType = ReadType(rType)
			if err := link.readType.Validate(); err != nil {
				return nil, fmt.Errorf("%s.type: %w", varName, err)
			}
		case "gear_keys":
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
			panic("rGear unsupported at this time")
		case "header": // "net/http".Header is of type Header map[string][]string
			if link.header, err = parseHeader(v); err != nil {
				return nil, errors.Wrapf(err, "%s.header", varName)
			}
		case "method":
			method, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s.method must be a string", varName)
			}
			link.method = method
		case "body":
			link.body, ok = v.(string)
			if !ok {
				return nil, errors.Errorf("%s.body must be a string: %T", varName, v)
			}
		default:
			return nil, fmt.Errorf("%s.%s is an unsupported key name", varName, k)
		}

	}

	// TODO simplify implicit inheritance, this is janky
	// if Path is empty string
	if link.Path == "" {
		return nil, fmt.Errorf("%s does not have a value assigned or %s.path defined", varName, varName)
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
	link.KeyName = varName
	if _, ok := cfgMap["name"]; !ok {
		link.SearchName = varName
		// if ctx.name was set then and var.name was not defined then inherit SearchName from baseLink
		if baseLink.SearchName != "" {
			link.SearchName = baseLink.SearchName
		}
	}

	link.remote = isValidURL(link.Path)
	// implicit header and method inheritance
	// if path is a URL & baseLink is non-nil
	if link.remote && baseLink != nil {
		if _, ok := cfgMap["header"]; !ok && baseLink.header != nil {
			link.header = baseLink.header
		}
		if _, ok := cfgMap["method"]; !ok {
			link.method = baseLink.method
		}
		if _, ok := cfgMap["body"]; !ok {
			link.body = baseLink.body
		}
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
