package cogs

import (
	"fmt"
	"os"

	"github.com/mikefarah/yq/v3/pkg/yqlib"
	"gopkg.in/yaml.v3"
)

type readType string

const (
	dotenv   readType = "dotenv"
	deferred readType = "" // defer file config type to filename suffix
)

// Validate ensures that a string is a valid readType enum
func (t readType) Validate() error {
	switch t {
	case dotenv:
		return nil
	default:
		return fmt.Errorf("%s is an invalid cfgType", t)
	}
}

type Queryable interface {
	Get(queryPath string) (string, error)
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

	_, err = file.Read(bytes)

	return bytes, nil

}

// NewYamlVisitor returns a visitor object that satisfies the Queryable interface
func NewYamlVisitor(buf []byte) (*yamlVisitor, error) {
	visitor := &yamlVisitor{
		rootNode:    &yaml.Node{},
		cachedNodes: make(map[string]map[string]string),
		parser:      yqlib.NewYqLib(),
	}

	// deserialize to yaml.Node
	if err := yaml.Unmarshal(buf, visitor.rootNode); err != nil {
		return nil, err
	}

	return visitor, nil
}

type cachedNode struct {
	readType readType
}
type yamlVisitor struct {
	rootNode    *yaml.Node
	cachedNodes map[string]map[string]string
	parser      yqlib.YqLib
}

func (n *yamlVisitor) Get(cfg *Cfg) (err error) {
	var ok bool

	if cfg.SubPath == "" {
		node, err := n.get(cfg.Name)
		if err != nil {
			return err
		}
		err = node.Decode(&cfg.Value)
		if err != nil {
			return err
		}

		return nil
	}

	if valMap, ok := n.cachedNodes[cfg.SubPath]; ok {
		cfg.Value, ok = valMap[cfg.Name]
		if !ok {
			return fmt.Errorf("unable to find %s", cfg)
		}
		return nil
	}

	node, err := n.get(cfg.SubPath)
	if err != nil {
		return err
	}

	// nodes with readType of deferred should be a string to string k/v pair
	if node.Kind != yaml.MappingNode || cfg.readType != deferred {
		return fmt.Errorf("Node kind unsupported at this time: %s", kindStr[node.Kind])
	}

	// for now only support string maps
	// TODO handle dotenv readType - P0PS-755
	cachedMap := make(map[string]string)
	err = node.Decode(&cachedMap)
	if err != nil {
		return err
	}

	cfg.Value, ok = cachedMap[cfg.Name]
	if !ok {
		return fmt.Errorf("unable to find %s", cfg)
	}

	// cache the valid node before returning the desired value
	n.cachedNodes[cfg.SubPath] = cachedMap

	return nil

}

func (n *yamlVisitor) get(subPath string) (*yaml.Node, error) {
	nodeCtx, err := n.parser.Get(n.rootNode, subPath)
	if err != nil {
		return nil, err
	}
	// should only match a single node
	if len(nodeCtx) != 1 {
		return nil, fmt.Errorf("returned non signular result for path '%s'", subPath)
	}
	return nodeCtx[0].Node, nil
}

var kindStr = map[yaml.Kind]string{
	0:                 "None",
	yaml.DocumentNode: "DocumentNode",
	yaml.SequenceNode: "SequenceNode",
	yaml.MappingNode:  "MappingNode",
	yaml.ScalarNode:   "ScalarNode",
	yaml.AliasNode:    "AliasNode",
}
