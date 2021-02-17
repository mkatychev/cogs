#! /usr/bin/env bash

go build -o ./tmp_cogs ./cmd/cogs
./tmp_cogs gen basic basic.cog.toml
./tmp_cogs gen basic basic.cog.toml --keys=var --out=toml
./tmp_cogs gen sops secrets.cog.toml
./tmp_cogs gen kustomize read_types.cog.toml
./tmp_cogs gen inheritor advanced.cog.toml
./tmp_cogs gen flat_json advanced.cog.toml
./tmp_cogs gen complex_json advanced.cog.toml
./tmp_cogs gen inheritor advanced.cog.toml
./tmp_cogs gen external_inheritor advanced.cog.toml
NVIM=nvim ./tmp_cogs gen envsubst envsubst.cog.toml -e
rm ./tmp_cogs
