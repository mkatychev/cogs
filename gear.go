package cogs

import (
	"fmt"
	"os"
	"path"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// Gear represents one of the contexts in a cog manifest.
// The term "gear" is used to refer to the operating state of a machine (similar
// to how a microservice can operate locally or in a remote environment)
// rather than a gear object. The term "switching gears" is an apt representation
// of how one Cog manifest file can have many contexts/environments
type Gear struct {
	Name       string
	linkMap    map[string]*Link // unresolved links: map["var_name"]*Link{Value: nil}
	filePath   string           // filepath of file.cog.toml
	fileBuf    []byte           // byte representation of TOML file
	tree       *toml.Tree       // TOML object tree
	outputType Format           // desired output type of the marshalled Gear
	recursions uint             // the amount of recursions for the current Gear
	filter     LinkFilter
}

func initGear(b []byte, envSubst bool) (*Gear, error) {
	var tree *toml.Tree
	var err error
	gear := &Gear{}

	if tree, err = toml.LoadBytes(b); err != nil {
		return nil, err
	}

	_, ok := tree.Get("name").(string)
	if !ok {
		return nil, fmt.Errorf("manifest.name string value must be present as a non-empty string")
	}

	var varMap map[string]string
	if tree.Has("env") {
		errMsg := "manifest.env value must be present as a map[string]string"
		values := tree.Get("env").(*toml.Tree).ToMap()
		if !ok {
			return nil, fmt.Errorf(errMsg)
		}
		envMap := make(map[string]string)
		for k, v := range values {
			strV, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf(errMsg)
			}
			envMap[k] = strV

		}
		varMap = envMap

	}
	if envSubst || varMap != nil {
		if b, err = envSub(b, envSubst, varMap); err != nil {
			return nil, err
		}
		if tree, err = toml.LoadBytes(b); err != nil {
			return nil, err
		}
	}

	gear.fileBuf = b
	gear.tree = tree

	return gear, nil

}

// SetName sets the gear name to the provided string
func (g *Gear) SetName(name string) {
	if g.Name != "" {
		g.Name = g.Name + "." + name
		return
	}
	g.Name = name
}

// ResolveMap outputs the flat associative string, resolving potential filepath pointers
// held by Link objects by calling the .SetValue() method
func (g *Gear) ResolveMap(ctx baseContext) (CfgMap, error) {
	var err error

	if g.linkMap, err = parseCtx(ctx); err != nil {
		return nil, err
	}
	if g.filter != nil {
		if g.linkMap, err = g.filter(g.linkMap); err != nil {
			return nil, err
		}
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
						return requestHTTPFile(path, header, method, body)
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
		var gearVar *Gear
		// 2. for each distinct Path: generate a Reader object
		linkFilePath := g.getLinkFilePath(p.path)
		// if link.Path references the cog file, return the already read (and envsubst applied) value
		if p.path == selfPath {
			fileBuf = g.fileBuf
		} else if fileBuf, err = pGroup.loadFile(linkFilePath); err != nil {
			if os.IsNotExist(err) {
				errs = multierr.Append(errs, err)
				continue
			}
			return nil, err
		}

		newVisitorFn := NewYAMLVisitor
		var visitor Visitor
		// 3. create visitor to handle SubPath strings
		// all read files should resolve to a yaml.Node, this includes JSON, TOML, and dotenv
		switch FormatForPath(linkFilePath) {
		case JSON:
			newVisitorFn = NewJSONVisitor
		case YAML:
			newVisitorFn = NewYAMLVisitor
		case TOML:
			newVisitorFn = NewTOMLVisitor
		case Dotenv:
			newVisitorFn = NewDotenvVisitor
		}

		// 4. traverse every Path and possible SubPath retrieving the Link.Values associated with it
		for _, link := range pGroup.links {
			switch link.readType {
			case rRaw: // no visitor is needed for a raw input
				link.Value = string(fileBuf)
			case rGear:
				if g.recursions > uint(RecursionLimit) {
					return nil, errors.New("recursion limit reached")
				}
				// assume that if path is selfPath then environmental substitution has
				// already been applied
				if gearVar == nil {
					envSubst := EnvSubst && p.path != selfPath
					gearVar, err = initGear(fileBuf, envSubst)
					if err != nil {
						return nil, errors.Wrap(err, link.KeyName)
					}
					gearVar.outputType = g.outputType
					gearVar.filePath = g.getLinkFilePath(link.Path)
					gearVar.recursions = g.recursions + 1
					gearVar.recursions = g.recursions + 1
				}
				// always reapply filter since gear read types can specify separate
				// key names
				gearVar.filter = link.GearFilter
				gearVar.Name = link.KeyName
				// begin recursion
				cfgMap, err := generate(link.SubPath, gearVar)
				if err != nil {
					return nil, errors.Wrap(err, link.KeyName)
				}
				link.Value = cfgMap[link.SearchName]
			default:
				if visitor == nil {
					visitor, err = newVisitorFn(fileBuf)
					if err != nil {
						return nil, err
					}
				}
				if err := visitor.SetValue(link); err != nil {
					return nil, errors.Wrap(err, link.KeyName)
				}
			}

		}

		// 5. add missing links to errs
		if visitor != nil {
			if visitorErrs := visitor.Errors(); visitorErrs != nil {
				errs = multierr.Append(errs, multierr.Combine(visitorErrs...))
			}
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
	dir := path.Dir(g.filePath)
	return path.Join(dir, linkPath)
}

// GetTree returns the toml.Tree private property
func (g *Gear) GetTree() *toml.Tree {
	return g.tree
}
