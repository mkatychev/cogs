#! /usr/bin/env bash

go build -o ./tmp_cogs ./cmd/cogs
./tmp_cogs gen basic              ./examples/1.basic.cog.toml
./tmp_cogs gen basic              ./examples/1.basic.cog.toml --keys=var --out=toml
./tmp_cogs gen get                ./examples/2.http.cog.toml
./tmp_cogs gen post               ./examples/2.http.cog.toml
./tmp_cogs gen post_multiple      ./examples/2.http.cog.toml
./tmp_cogs gen sops               ./examples/3.secrets.cog.toml
./tmp_cogs gen kustomize          ./examples/4.read_types.cog.toml
./tmp_cogs gen inheritor          ./examples/5.advanced.cog.toml
./tmp_cogs gen flat_json          ./examples/5.advanced.cog.toml
./tmp_cogs gen complex_json       ./examples/5.advanced.cog.toml
./tmp_cogs gen inheritor          ./examples/5.advanced.cog.toml
./tmp_cogs gen external_inheritor ./examples/5.advanced.cog.toml
NEWLINE_VAR="
This Var is on More than one line
" NVIM=nvim ./tmp_cogs gen envsubst ./examples/6.envsubst.cog.toml -e
rm ./tmp_cogs
