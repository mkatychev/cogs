package cogs

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
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
	// defaultValue interface{} // default value if key is missing
	Path      string      // filepath string where Link can be resolved
	SubPath   string      // object traversal string used to resolve Link if not at top level of document (yq syntax)
	encrypted bool        // indicates if decryption is needed to resolve Link.Value
	remote    bool        // indicates if an HTTP request is needed to return the given document
	header    http.Header // HTTP request headers
	method    string      // HTTP request method
	body      string      // HTTP request body
	aliases   []string    // additional key names that map to the same value
	readType  ReadType
	// keys       []string    // key filter for Gear read types
}

// GearFilter is used to filter link maps when read type is gear
func (c *Link) GearFilter(linkMap map[string]*Link) (map[string]*Link, error) {
	var ok bool
	filteredMap := make(map[string]*Link)

	if filteredMap[c.SearchName], ok = linkMap[c.SearchName]; !ok {
		return nil, errors.Errorf("Link.name: %q is not present in the provided gear map", c.SearchName)
	}
	// if keys == nil || len(keys) == 0 {
	//     return linkMap, nil
	// }
	// for k, _ := range linkMap {
	//     if !InList(k, keys) {
	//         delete(linkMap, k)
	//     }
	// }
	return linkMap, nil
}

// ensure Link.aliases strings do not exist as preexisting key entries
func (c *Link) validateAliases(linkMap map[string]*Link) error {
	for i, alias := range c.aliases {
		if _, ok := linkMap[alias]; ok {
			return fmt.Errorf("%s.aliases[%d]: key %q already present in ctx", c.KeyName, i, alias)
		}
		aliasLink := *c
		aliasLink.KeyName = alias
		linkMap[alias] = &aliasLink
	}
	return nil
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

// CfgMap is meant to represent a map with values of one or more unknown types
type CfgMap map[string]interface{}

// Join merges any number of CfgMap elements into a single CfgMap,
// elements with a duplicate key name return an error.
func Join(cfgs ...*CfgMap) (CfgMap, error) {
	c := CfgMap{}
	for _, cfg := range cfgs {
		for k, v := range *cfg {
			if _, ok := c[k]; ok {
				return nil, fmt.Errorf("duplicate key found in two contexts: %q", k)
			}
			c[k] = v
		}
	}
	return c, nil
}

// LinkFilter if a function meant to filter a LinkMap
type LinkFilter func(map[string]*Link) (map[string]*Link, error)

// Resolver is meant to define an object that returns the final string map to be used in a configuration
// resolving any paths and sub paths defined in the underling config map
type Resolver interface {
	ResolveMap(baseContext) (CfgMap, error)
	SetName(string)
	GetTree() *toml.Tree
}

// Generate is a top level command that takes an context name argument and cog file path to return a string map
func Generate(ctxName, cogPath string, outputType Format, filter LinkFilter) (CfgMap, error) {
	var err error

	if err = outputType.Validate(); err != nil {
		return nil, err
	}

	b, err := readFile(cogPath)
	if err != nil {
		return nil, err
	}

	gear, err := initGear(b, EnvSubst)
	if err != nil {
		return nil, err
	}

	gear.filePath = cogPath
	gear.outputType = outputType
	gear.recursions = 0
	gear.filter = filter
	cfgMap, err := generate(ctxName, gear)
	if err != nil {
		return nil, err
	}

	return cfgMap, nil

}

func generate(ctxName string, gear Resolver) (CfgMap, error) {
	var err error
	var ctx baseContext

	table, ok := gear.GetTree().Get(ctxName).(*toml.Tree)
	if !ok {
		// TODO  ErrMissingContext = errorW{fmt:"%s: %s context missing from cog file"}
		errMsg := fmt.Sprintf("%q context missing from cog file", ctxName)
		if g, ok := gear.(*Gear); ok {
			errMsg = g.filePath + ": " + errMsg
		}
		return nil, errors.New(errMsg)
	}
	gear.SetName(ctxName)

	var tableMap map[string]interface{}
	if err := table.Unmarshal(&tableMap); err != nil {
		return nil, err
	}

	if err = mapstructure.Decode(tableMap, &ctx); err != nil {
		return nil, fmt.Errorf("generate context: %w", err)
	}
	ctx.Name = ctxName
	genOut, err := gear.ResolveMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ctxName, err)
	}

	return genOut, nil
}

// parseCtx traverses an map interface to populate a gear's configMap
func parseCtx(ctx baseContext) (linkMap map[string]*Link, err error) {
	linkMap = make(map[string]*Link)

	// skip fetching encrypted vars if flag is toggled
	if !NoEnc && ctx.Enc.Vars != nil {
		err = decodeEncVars(linkMap, ctx.Enc)
		if err != nil {
			return nil, errors.Wrap(err, ctx.Name)
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
	Name string

	// config maps
	Vars CfgMap  `mapstructure:",omitempty"`
	Enc  context `mapstructure:",omitempty"`
	// inheritable
	SearchName string      `mapstructure:"name,omitempty"`
	Path       interface{} `mapstructure:",omitempty"`
	ReadType   string      `mapstructure:"type,omitempty"`
	Header     interface{} `mapstructure:",omitempty"`
	Method     string      `mapstructure:",omitempty"`
	Body       string      `mapstructure:",omitempty"`
}

// toContext returns the unencrypted context properties ignoring baseContext.Enc
func (b baseContext) toContext() context {
	return context{
		Name:       b.Name,
		Path:       b.Path,
		ReadType:   b.ReadType,
		SearchName: b.SearchName,
		Vars:       b.Vars,
		Header:     b.Header,
		Method:     b.Method,
		Body:       b.Body,
	}
}

// context is a struct meant to represent both encrypted and plaintext sections of a baseContext
type context struct {
	Name       string
	Path       interface{} `mapstructure:",omitempty"`
	ReadType   string      `mapstructure:"type,omitempty"`
	SearchName string      `mapstructure:"name,omitempty"`
	Vars       CfgMap      `mapstructure:",omitempty"`
	Header     interface{} `mapstructure:",omitempty"`
	Method     string      `mapstructure:",omitempty"`
	Body       string      `mapstructure:",omitempty"`
}

func decodeVars(linkMap map[string]*Link, ctx context) error {
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
	baseLink.SearchName = ctx.SearchName
	// type
	baseLink.readType = ReadType(ctx.ReadType)
	if err := baseLink.readType.Validate(); err != nil {
		return err
	}
	// HTTP header
	if ctx.Header != nil {
		if baseLink.header, err = parseHeader(ctx.Header); err != nil {
			return err
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
		} else if rawLink, ok := v.(map[string]interface{}); ok {
			if linkMap[k], err = parseLink(k, &baseLink, rawLink); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		} else {
			return fmt.Errorf("%s: %T is an unsupported type", k, v)
		}
	}

	// invalid/duplicate alias name should always propagate from the Link.alias field
	// rather thank from a `_, ok := linkMap[k]` check so that the alias index can
	// be provided in the error message
	for _, k := range Keys(linkMap) {
		if err = linkMap[k].validateAliases(linkMap); err != nil {
			return err
		}
	}
	return nil
}

// decodeEncVars is a convenience function for passing ctx.enc variables to decodeEnv
func decodeEncVars(linkMap map[string]*Link, ctx context) error {
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
func parseLink(varName string, baseLink *Link, rawLink map[string]interface{}) (*Link, error) {
	var link Link
	var ok bool
	var err error

	for k, v := range rawLink {
		switch k {
		case "value":
			link.Value = v
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
		case "aliases":
			aliasErr := fmt.Errorf("%s.aliases must be an array of strings", varName)
			slice, ok := v.([]interface{})
			if !ok {
				return nil, aliasErr
			}
			for _, v := range slice {
				str, ok := v.(string)
				if !ok {
					return nil, aliasErr
				}
				link.aliases = append(link.aliases, str)
			}
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

	// if Path is empty string and Value has not been explicitly assigned
	if link.Path == "" && link.Value == nil {
		return nil, fmt.Errorf("%s does not have a value assigned or %s.path defined", varName, varName)
	}

	// if readType was not specified:
	if _, ok := rawLink["type"]; !ok {
		if baseLink != nil {
			link.readType = baseLink.readType
		} else {
			link.readType = deferred
		}
	}

	// if readType is raw and a SubPath exists
	if link.readType == rRaw && link.SubPath != "" {
		return nil, fmt.Errorf("%s subpath must not be defined for an input of raw", varName)
	}

	// if name is not defined: `var = "value"`
	// then set link.Name to the key name, "var" in this case
	link.KeyName = varName
	if _, ok := rawLink["name"]; !ok {
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
		if _, ok := rawLink["header"]; !ok && baseLink.header != nil {
			link.header = baseLink.header
		}
		if _, ok := rawLink["method"]; !ok {
			link.method = baseLink.method
		}
		if _, ok := rawLink["body"]; !ok {
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
