package cogs

import (
	"errors"
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
	config map[string]string
	err    error
}

func TestGenerate(t *testing.T) {
	testCases := []generateTestOut{
		{
			name: "BasicConfig",
			env:  "local",
			toml: basicCogToml,
			config: map[string]string{
				"var":       "var_value",
				"other_var": "other_var_value",
			},
			err: nil,
		},
		{
			name: "ConfigWithPath",
			env:  "qa",
			toml: basicCogToml,
			config: map[string]string{
				"enc_var": "|enc|./path.enc",
				"var":     "|path|./path|subpath|.subpath",
			},
			err: nil,
		},
		{
			name:   "DuplicateKeyInEnc/Error",
			env:    "local",
			toml:   errCogToml,
			config: nil,
			err:    errors.New("local: var: duplicate key present in env and env.enc"),
		},
		{
			name:   "InvalidPathArray/Error",
			env:    "qa",
			toml:   errCogToml,
			config: nil,
			err:    errors.New("qa: var: path array must only contain two values mapping to path and subpath respectively"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := toml.Load(tc.toml)
			if err != nil {
				t.Fatalf("toml.Load: %s", err)
			}
			cogName := tree.Get("name").(string)
			config, err := generate(tc.env, tree, &testGear{})

			if diff := cmp.Diff(tc.err, err, AllowUnexported); diff != "" {
				t.Fatalf("toml[%s], env[%s]: (-expected error +actual error)\n-\t%s\n+\t%s", cogName, tc.env, tc.err, err)
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

[local]
var = "var_value"
other_var = "other_var_value"
[qa]
var.path = ["./path", ".subpath"]
[qa.enc]
enc_var.path = ["./path.enc", ".subpath"]
`
	errCogToml = `
name = "errCogToml"

[local]
var = "var_value"
[local.enc]
var = "other_var_value"
[qa]
var.path = ["./path", ".subpath", "err_index"]
[qa.enc]
enc_var.path = ["./path.enc", ".subpath"]
`
)

type testGear struct {
	Name   string
	cfgMap configMap
}

// SetName sets the gear name to the provided string
func (g *testGear) SetName(name string) {
	g.Name = name
}

// ResolveMap is used to satisfy the Generator interface
func (g *testGear) ResolveMap(env RawEnv) (map[string]string, error) {
	var err error

	g.cfgMap, err = parseEnv(env)
	if err != nil {
		return nil, err
	}

	// final output
	cfgOut := make(map[string]string)

	for k, cfg := range g.cfgMap {
		cfgOut[k] = g.ResolveValue(cfg)
	}
	return cfgOut, nil

}

// ResolveValue returns the value corresponding to a Cfg struct
// if Path resolves to a valid file the file byte value
// is passed to a file reader object, attempting to serialize the contents of
// the file if type is supported
func (g *testGear) ResolveValue(c *Cfg) string {
	// if Path is empty or Value is non empty
	if c.Path == "" || c.Value != "" {
		return c.Value
	}

	if c.encrypted {
		// decrypt.File(c.Path, c.SubPath)
		return "|enc|" + c.Path
	}

	if c.SubPath != "" {
		return "|path|" + c.Path + "|subpath|" + c.SubPath
	}

	return "|path|" + c.Path
}
