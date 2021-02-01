package cogs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/drone/envsubst"
	"github.com/joho/godotenv"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// "." - a single period is a reserved filepath string
// it is used to self-reference the cog file
// this helps avoid breaking generation when the cog file is moved or renamed
const selfPath string = "."

type readType string

const (
	// read format overrides
	rDotenv      readType = "dotenv"
	rJSON        readType = "json"
	rJSONComplex readType = "json{}" // complex json key value pair: {"k":{"v1":[],"v2":[]}}

	// read format derived from filepath suffix
	rWhole   readType = "whole" // indicates to associate the entirety of a file to the given key name
	deferred readType = ""      // defer file config type to filename suffix
	rGear    readType = "gear"  // treat TOML table as a nested gear object
)

// Validate ensures that a string is a valid readType enum
func (t readType) Validate() error {
	switch t {
	case rDotenv, rJSON, rJSONComplex, rWhole:
		return nil
	default: // deferred readType should not be validated
		return fmt.Errorf("%s is an invalid linkType", string(t))
	}
}

func (t readType) String() string {
	switch t {
	case rDotenv:
		return string(rDotenv)
	case rJSON:
		return "flat json"
	case rJSONComplex:
		return "complex json"
	case rWhole:
		return "whole file"
	case rGear:
		return "gear object"
	case deferred:
		return "deferred"
	default:
		return "unknown"
	}
}

// readFile takes a filepath and returns the byte value of the data within
func readFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats, statsErr := file.Stat()
	if statsErr != nil {
		return nil, statsErr
	}

	var size int64 = stats.Size()
	bytes := make([]byte, size)

	if _, err = file.Read(bytes); err != nil {
		return nil, err
	}

	return bytes, nil

}

// envSubBytes returns a TOML string with environmental substitution applied, call tldr for more:
// $ tldr envsubst
func envSubBytes(bytes []byte) ([]byte, error) {
	// ------------------------------------------------------------------------
	// strip comments so we dont do comment substitution by tokenizing the file
	// and reemiting the file as bytes  ¯\_(ツ)_/¯
	tree, err := toml.LoadBytes(bytes)
	if err != nil {
		return nil, err
	}
	noCommentTree, err := toml.Marshal(tree)
	if err != nil {
		return nil, err
	}
	// ------------------------------------------------------------------------

	substEnv, err := envsubst.EvalEnv(string(noCommentTree))
	if err != nil {
		return nil, err
	}
	return []byte(substEnv), nil
}

// kindStr maps the yaml node types to strings for error messaging
var kindStr = map[yaml.Kind]string{
	0:                 "None",
	yaml.DocumentNode: "DocumentNode",
	yaml.SequenceNode: "SequenceNode",
	yaml.MappingNode:  "MappingNode",
	yaml.ScalarNode:   "ScalarNode",
	yaml.AliasNode:    "AliasNode",
}

// Visitor allows a query path to return the underlying value for a given visitor
type Visitor interface {
	SetValue(*Link) error
}

// NewJSONVisitor returns a visitor object that satisfies the Visitor interface
// attempting to turn a supposed JSON byte slice into a *yaml.Node object
func NewJSONVisitor(buf []byte) (Visitor, error) {
	var i interface{}
	if err := json.Unmarshal(buf, &i); err != nil {
		return nil, err
	}
	// deserialize to yaml.Node
	rootNode := &yaml.Node{}
	if err := rootNode.Encode(i); err != nil {
		return nil, errors.Wrap(err, "NewJSONVisitor")
	}
	return newVisitor(rootNode), nil
}

// NewYAMLVisitor returns a visitor object that satisfies the Visitor interface
func NewYAMLVisitor(buf []byte) (Visitor, error) {
	// deserialize to yaml.Node
	rootNode := &yaml.Node{}
	if err := yaml.Unmarshal(buf, rootNode); err != nil {
		return nil, errors.Wrap(err, "NewYAMLVisitor")
	}
	return newVisitor(rootNode), nil
}

// NewTOMLVisitor returns a visitor object that satisfies the Visitor interface
// attempting to turn a supposed TOML byte slice into a *yaml.Node object
func NewTOMLVisitor(buf []byte) (Visitor, error) {
	var i interface{}
	if err := toml.Unmarshal(buf, &i); err != nil {
		return nil, errors.Wrap(err, "NewTOMLVisitor")
	}
	// deserialize to yaml.Node
	rootNode := &yaml.Node{}
	if err := rootNode.Encode(i); err != nil {
		return nil, err
	}
	return newVisitor(rootNode), nil
}

// NewDotenvVisitor returns a visitor object that satisfies the Visitor interface
// attempting to turn a supposed dotenv byte slice into a *yaml.Node object
func NewDotenvVisitor(buf []byte) (Visitor, error) {
	tempMap, err := godotenv.Unmarshal(string(buf))
	if err != nil {
		return nil, err
	}
	// deserialize to yaml.Node
	rootNode := &yaml.Node{}
	if err := rootNode.Encode(tempMap); err != nil {
		return nil, err
	}
	return newVisitor(rootNode), nil
}

func newVisitor(node *yaml.Node) Visitor {
	return &visitor{
		rootNode:       node,
		visited:        make(map[string]map[string]string),
		visitedComplex: make(map[string]interface{}),
		evaluator:      yqlib.NewAllAtOnceEvaluator(),
	}
}

type visitor struct {
	rootNode       *yaml.Node
	visited        map[string]map[string]string
	visitedComplex map[string]interface{}
	evaluator      yqlib.Evaluator
}

// SetValue assigns the Value for a given Link using the existing Link.Path and Link.SubPath
func (vi *visitor) SetValue(link *Link) (err error) {
	switch link.readType {
	case rWhole, rJSONComplex:
		return vi.visitComplex(link)
	case rGear:
		panic("rGear unsupported at this time")
	}

	// 1. check if link.SubPath value has been used in a previous SetValue call
	if flatMap, ok := vi.visited[link.SubPath]; ok {
		if link.Value, ok = flatMap[link.Name]; !ok {
			return fmt.Errorf("unable to find %s", link.Name)
		}
		return nil
	}

	// 2. grab the yaml node corresponding to the subpath
	node, err := vi.get(link.SubPath)
	if err != nil {
		return err
	}

	supportedKind := func() bool {
		for _, kind := range []yaml.Kind{yaml.MappingNode, yaml.ScalarNode, yaml.SequenceNode, yaml.DocumentNode} {
			if node.Kind == kind {
				return true
			}
		}
		return false
	}()

	if !supportedKind {
		return fmt.Errorf("%s: NodeKind/readType unsupported: %s/%s",
			link.Name, kindStr[node.Kind], link.readType)
	}

	cachedMap := make(map[string]string)

	// 3. traverse node based on read type
	switch link.readType {
	case rDotenv:
		err = visitDotenv(&cachedMap, node)
	case rJSON:
		err = visitJSON(cachedMap, node)
	case deferred:
		err = node.Decode(cachedMap)
	default:
		err = fmt.Errorf("unsupported readType: %s", link.readType)
	}
	if err != nil {
		return errors.Wrap(err, "SetValue")
	}

	// 4. add value to cache
	vi.visited[link.SubPath] = cachedMap

	// 5. recurse to access cache
	return vi.SetValue(link)

}

// visitComplex handles the rWhole and rJSONComplex read types
func (vi *visitor) visitComplex(link *Link) (err error) {
	// 1. check if link.SubPath and readType has been used before
	if v, ok := vi.visitedComplex[link.SubPath]; ok {
		if link.readType == rWhole {
			link.ComplexValue = v

			return nil
		}
		complexMap, ok := v.(map[string]interface{})
		if !ok {
			return fmt.Errorf("path does not resolve to a map: %T", v)
		}
		if link.ComplexValue, ok = complexMap[link.Name]; !ok {
			return fmt.Errorf("unable to find %s", link.Name)
		}
		return nil
	}
	// 2. grab the yaml node corresponding to the subpath
	node, err := vi.get(link.SubPath)
	if err != nil {
		return err
	}
	// 3. traverse node based on read type
	var i interface{}
	switch link.readType {
	case rWhole:
		err = node.Decode(&i)
	case rJSONComplex:
		i = make(map[string]interface{})
		err = visitJSONComplex(i.(map[string]interface{}), node)
	case rGear:
		panic("rGear unsupported at this time")
	default:
		err = fmt.Errorf("unsupported readType: %s", link.readType)
	}
	if err != nil {
		return errors.Wrap(err, "visitComplex")
	}
	// 4. add value to cache
	vi.visitedComplex[link.SubPath] = i
	// 5. recurse to access cache
	return vi.visitComplex(link)
}

func (vi *visitor) get(subPath string) (*yaml.Node, error) {
	list, err := vi.evaluator.EvaluateNodes(subPath, vi.rootNode)
	if err != nil {
		return nil, err
	}
	nodes := []*yqlib.CandidateNode{}
	for el := list.Front(); el != nil; el = el.Next() {
		n := el.Value.(*yqlib.CandidateNode)
		nodes = append(nodes, n)
	}
	// should only match a single node
	if len(nodes) != 1 {
		return nil, fmt.Errorf("returned non singular result for path '%s'", subPath)
	}
	return nodes[0].Node, nil
}

func visitDotenv(cache *map[string]string, node *yaml.Node) (err error) {
	var strEnv string

	if err = node.Decode(&strEnv); err != nil {
		var sliceEnv []string
		if err := node.Decode(&sliceEnv); err != nil {
			return fmt.Errorf("Unable to decode node kind: %s to dotenv format", kindStr[node.Kind])
		}
		strEnv = strings.Join(sliceEnv, "\n")
	}
	*cache, err = godotenv.Unmarshal(strEnv)
	return err
}

func visitJSON(cache map[string]string, node *yaml.Node) error {
	if err := node.Decode(&cache); err == nil {
		return nil
	}

	var strEnv string

	if err := node.Decode(&strEnv); err != nil {
		var sliceEnv []string
		if err := node.Decode(&sliceEnv); err != nil {
			return fmt.Errorf("Unable to decode node kind: %s to flat JSON format", kindStr[node.Kind])
		}
		strEnv = strings.Join(sliceEnv, "\n")
	}
	return json.Unmarshal([]byte(strEnv), &cache)
}

func visitJSONComplex(cache map[string]interface{}, node *yaml.Node) error {
	if err := node.Decode(&cache); err == nil {
		return nil
	}

	var strEnv string
	if err := node.Decode(&strEnv); err != nil {
		return fmt.Errorf("Unable to decode node kind: %s to complex JSON format: %w", kindStr[node.Kind], err)
	}
	return json.Unmarshal([]byte(strEnv), &cache)
}
