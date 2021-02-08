package cogs

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pelletier/go-toml"
)

var (
	// AllowUnexported whitelists private fields for cmp.Diff comparison
	AllowUnexported cmp.Option = cmp.Exporter(func(reflect.Type) bool { return true })
)

type generateTestOut struct {
	name   string
	env    string
	toml   string
	config CfgMap
	err    error
}

func TestGenerate(t *testing.T) {
	testCases := []generateTestOut{
		{
			name: "BasicConfig",
			env:  "local",
			toml: basicCogToml,
			config: map[string]interface{}{
				"var":       "var_value",
				"other_var": "other_var_value",
			},
			err: nil,
		},
		{
			name: "ConfigWithPath",
			env:  "qa",
			toml: basicCogToml,
			config: map[string]interface{}{
				"enc_var": "|path.enc|./path.enc|subpath|.subpath",
				"var":     "|path|./path|subpath|.subpath",
			},
			err: nil,
		},
		{
			name: "ConfigWithInheritedPath",
			env:  "path_env",
			toml: basicCogToml,
			config: map[string]interface{}{
				"var1":     "|path|./path|subpath|.subpath",
				"var2":     "|path|./path|subpath|.other_subpath",
				"var3":     "|path|./other_path|subpath|.subpath",
				"enc_var1": "|path.enc|./path.enc|subpath|.subpath",
				"enc_var2": "|path.enc|./path.enc|subpath|.other_subpath",
				"enc_var3": "|path.enc|./other_path.enc|subpath|.subpath",
			},
			err: nil,
		},
		{
			name:   "DuplicateKeyInEnc/Error",
			env:    "local",
			toml:   errCogToml,
			config: nil,
			err:    errors.New("local: var: duplicate key present in ctx and ctx.enc"),
		},
		{
			name:   "InvalidPathArray/Error",
			env:    "qa",
			toml:   errCogToml,
			config: nil,
			err:    errors.New("qa: var: var.path: path array must have a length of two, providing path and subpath respectively"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := toml.Load(tc.toml)
			if err != nil {
				t.Errorf("toml.Load: %s", err)
			}
			cogName := tree.Get("name").(string)
			config, err := generate(tc.env, tree, &testGear{Name: tc.env})
			// TODO implement (err cogError) Unwrap() error { return err.err } so that "%w" directive is used
			if diff := cmp.Diff(fmt.Errorf("%s", tc.err), fmt.Errorf("%s", err), AllowUnexported); diff != "" {
				t.Errorf("toml[%s], env[%s]: (-expected err +actual err)\n-%s", cogName, tc.env, diff)
			}
			if diff := cmp.Diff(tc.config, config, AllowUnexported); diff != "" {
				t.Errorf("toml[%s], env[%s]: (-expected config +actual config):\n%s", cogName, tc.env, diff)
			}
		})

	}
}

var (
	basicCogToml = `
name = "basicCogToml"

[local.vars]
var = "var_value"
other_var = "other_var_value"
[qa.vars]
var.path = ["./path", ".subpath"]
[qa.enc.vars]
enc_var.path = ["./path.enc", ".subpath"]
[path_env]
path = ["./path", ".subpath"]
[path_env.vars]
var1.path = []
var2.path = [[], ".other_subpath"]
var3.path = ["./other_path", []]
[path_env.enc]
path = ["./path.enc", ".subpath"]
[path_env.enc.vars]
enc_var1.path = []
enc_var2.path = [[], ".other_subpath"]
enc_var3.path = ["./other_path.enc", []]
`
	errCogToml = `
name = "errCogToml"

[local.vars]
var = "var_value"
[local.enc.vars]
var = "other_var_value"
[qa.vars]
var.path = ["./path", ".subpath", "err_index"]
[qa.enc.vars]
enc_var.path = ["./path.enc", ".subpath"]
`
)

type testGear struct {
	Name    string
	linkMap LinkMap
}

// SetName sets the gear name to the provided string
func (g *testGear) SetName(name string) {
	g.Name = name
}

// ResolveMap is used to satisfy the Generator interface
func (g *testGear) ResolveMap(ctx baseContext) (CfgMap, error) {
	var err error

	g.linkMap, err = parseCtx(ctx)
	if err != nil {
		return nil, err
	}

	// final output
	linkOut := make(map[string]interface{})

	for k, link := range g.linkMap {
		linkOut[k] = ResolveValue(link)
	}
	return linkOut, nil

}

// ResolveValue returns the value corresponding to a Link struct
// if Path resolves to a valid file the file byte value
// is passed to a file reader object, attempting to serialize the contents of
// the file if type is supported
func ResolveValue(c *Link) string {
	// if Path is empty or Value is non empty
	if c.Path == "" {
		if val, ok := c.Value.(string); ok {
			return val
		}
	}

	pathStr := "|path|"

	if c.encrypted {
		// decrypt.File(c.Path, c.SubPath)
		pathStr = "|path.enc|"
	}

	if c.SubPath != "" {
		return pathStr + c.Path + "|subpath|" + c.SubPath
	}

	return "|path|" + c.Path
}
