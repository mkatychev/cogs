#! /usr/bin/env bash

go build -o ./tmp_cogs ./cmd/cogs
./tmp_cogs gen ./examples/1.basic.cog.toml basic
./tmp_cogs gen ./examples/1.basic.cog.toml basic  --keys=var --out=toml
./tmp_cogs gen ./examples/2.http.cog.toml get
./tmp_cogs gen ./examples/2.http.cog.toml post
./tmp_cogs gen ./examples/2.http.cog.toml post_multiple
./tmp_cogs gen ./examples/3.secrets.cog.toml sops
./tmp_cogs gen ./examples/4.read_types.cog.toml kustomize
./tmp_cogs gen ./examples/5.advanced.cog.toml inheritor
./tmp_cogs gen ./examples/5.advanced.cog.toml flat_json
./tmp_cogs gen ./examples/5.advanced.cog.toml complex_json
./tmp_cogs gen ./examples/5.advanced.cog.toml inheritor
./tmp_cogs gen ./examples/5.advanced.cog.toml external_inheritor
NEWLINE_VAR="
This Var is on More than one line
" NVIM=nvim ./tmp_cogs gen ./examples/6.envsubst.cog.toml envsubst -e
rm ./tmp_cogs
