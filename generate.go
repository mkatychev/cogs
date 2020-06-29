package cogs

import (
	"fmt"
	"github.com/BurntSushi/toml"
)

type CogManifest struct {
	Name string
	Envs map[string]Env
}

type Env struct {
	Name string
	Cfgs []Cfg
}

type Cfg struct {
	name    string
	path    string
	subPath string
	value   string
}

func Generate(env, cogFile string) error {
	var manifest CogManifest
	if _, err := toml.DecodeFile(cogFile, &manifest); err != nil {
		return err
	}
	fmt.Println(manifest)
	return nil
}
