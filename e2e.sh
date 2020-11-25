#! /usr/bin/env bash

go run ./cmd/cogs gen docker basic.cog.toml
go run ./cmd/cogs gen sops basic.cog.toml
go run ./cmd/cogs gen kustomize basic.cog.toml
go run ./cmd/cogs gen inheritor advanced.cog.toml
go run ./cmd/cogs gen flat_json advanced.cog.toml
go run ./cmd/cogs gen complex_json advanced.cog.toml
go run ./cmd/cogs gen inheritor advanced.cog.toml
go run ./cmd/cogs gen envsubst envsubst.cog.toml -e
