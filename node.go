package cogs

import (
	"strings"

	"gopkg.in/yaml.v3"
)

const GoTemplateDelimL = "gt{{"
const GoTemplateDelimR = "}}gt"

var GoTemplateDelimPresent = false

func StripGoTemplateDelim(str string) string {
	str = strings.ReplaceAll(str, GoTemplateDelimL, "{{ ")
	str = strings.ReplaceAll(str, GoTemplateDelimR, " }}")
	return str
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

type Node struct {
	inner *yaml.Node
}

func NewGoTemplateToStr(node *yaml.Node) {
	templateNode := &Node{inner: node}
	templateNode.GoTemplateToStr()
	*node = *templateNode.inner
}

func (n *Node) Keys() []*yaml.Node {
	var keys []*yaml.Node
	getKeys(n.inner, &keys)
	return keys
}

func getKeys(node *yaml.Node, keys *[]*yaml.Node) {
	switch node.Kind {
	case yaml.DocumentNode:
		getKeys(node.Content[0], keys)
	case yaml.SequenceNode:
		sequenceKeys(node, keys)
	case yaml.MappingNode:
		mappingKeys(node, keys)
	default:
		return
	}
}

func mappingKeys(node *yaml.Node, keys *[]*yaml.Node) {
	for i, n := range node.Content {
		if i%2 == 0 {
			n.Alias = node
			*keys = append(*keys, n)
		} else {
			getKeys(n, keys)
		}
	}
	return
}

func sequenceKeys(node *yaml.Node, keys *[]*yaml.Node) {
	for _, n := range node.Content {
		getKeys(n, keys)
	}
	return
}

func (n *Node) GoTemplateToStr() {
	keys := n.Keys()
	for i, k := range keys {
		if k.Kind == yaml.SequenceNode || k.Kind == yaml.MappingNode {

			content := joinContent(k)
			*keys[i].Alias = yaml.Node{
				Kind:    yaml.ScalarNode,
				Tag:     "!!str",
				Value:   GoTemplateDelimL + content + GoTemplateDelimR,
				Line:    k.Line,
				Column:  k.Column,
				Content: []*yaml.Node{},
			}
			GoTemplateDelimPresent = true
		}
	}

}

func joinContent(node *yaml.Node) string {
	str := ""
	for _, c := range node.Content {
		str += c.Value
	}
	return str
}
