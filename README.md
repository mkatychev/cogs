## installation: 
* clone this repo, `cd` into it
* `go build -o $GOPATH/bin ./cmd/cogs`

```
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [--out=<type>] [--keys=<key,>] [-n] [-e]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Return specific keys from cog manifest.
  --out=<type>     Configuration output type [default: json].
                   Valid types: json, toml, yaml, dotenv, raw.
```

## annotated spec:

```toml
name = "basic_service"

# key value pairs for a context are defined under <ctx>.vars
[docker.vars]
var = "var_value"
other_var = "other_var_value"

[sops]
# a default path to be inherited can be defined under <ctx>.path
path = ["./test_files/manifest.yaml", "subpath"]
[sops.vars]
# a <var>.path key can map to four valid types:
# 1. path value is "string_value" - indicating a single file to look through
# 2. path value is [] - thus <ctx>.path will be inherited
# 3. path value is a ["two_index, "array"] - either index being [] or "string_value":
# -  [[], "subpath"] - path will be inherited from <ctx>.path if present
# -  ["path", []] - subpath will be inherited from <ctx>.path if present
# -  ["path", "subpath"] - nothing will be inherited
var1.path = ["./test_files/manifest.yaml", "subpath"]
var2.path = []
var3.path = [[], "other_subpath"]
# dangling variable should return 'some_var = ""' since only name override was defined
some_var.name = "some_name" 
# key value pairs for an encrypted context are defined under <ctx>.enc.vars
[sops.enc.vars]
enc_var.path = "./test_files/test.enc.yaml"

[kustomize]
path = ["./test_files/kustomization.yaml", "configMapGenerator.[0].literals"]
# a default deserialization path to be inherited can be defined under <ctx>.path
# once <var>.path has been traversed, attempt to deserialize the returned object
# as if it was in dotenv format
type = "dotenv"
[kustomize.vars]
# var1.name = "VAR_1" means that the key alias "VAR_1" will
# be searched for to retrieve the var1 value
var1 = {path = [], name = "VAR_1"}
var2 = {path = [], name = "VAR_2"}
```

## goals:

1. Allow a flat style of managing configurations across disparate contexts and different formats (plaintext vs. encrypted)
    * aggregates plaintext config values and SOPS secrets in one manifest
        - ex: local development vs. docker vs. production environments

1. Introduce an automated and cohesive way to change configurations wholesale
    * allow a gradual introduction of new variable names by automating:
        - introduction of new name for same value (`DB_SECRETS -> DATABASE_SECRETS`)
        - and deprecation of old name (managing deletion of old `DB_SECRETS` references)

## scope of support:

- microservice configuration
- parse YAML manifests
- valid [viper package](https://github.com/spf13/viper) input (so able to output JSON, YAML, and TOML)
- [SOPS secret management](https://github.com/mozilla/sops)
- [docker-compose](https://github.com/docker/compose) YAML env config scheme

## subcommands

* `cogs gen`
  - outputs a flat and serialized K:V array

* `cogs migrate` TODO
  - `cogs migrate <OLD_KEY_NAME> <NEW_KEY_NAME> [<envs>...]`
  - `cogs migrate --commit <OLD_KEY_NAME> <NEW_KEY_NAME> (<envs>...)`

Aims to allow a gradual and automated migration of key names without risking sensitive environments:

```yaml
# config.yaml pre migration
DB_SECRETS: "secret_pw"
```

Should happen in two main steps: 
1. `cogs migrate DB_SECRETS DATABASE_SECRETS`
- should default to creating the new key name in all environments
- creates new variable in remote file or cog manifest

```yaml
# config.yaml during migration
DB_SECRETS: "secret_pw"
DATABASE_SECRETS: "secret_pw"
```

2. `cogs migrate --commit DB_SECRETS DATABASE_SECRETS <env>...`
- removes old key name  for all `<envs>` specified

```yaml
# config.yaml post migration
DATABASE_SECRETS: "secret_pw"
```

* should apply to plaintext K/Vs and SOPS encrypted values

## running example data locally:
* `gpg --import ./test_files/sops_functional_tests_key.asc` should be run to import the test private key used for encrypted dummy data
* Building binary locally : `go build -o $GOPATH/bin ./cmd/cogs`
* Kustomize style var retrieval: `cogs gen  kustomize ./basic.cog.toml`
* Encrypted var retrieval: `cogs gen sops ./basic.cog.toml`
* `some-service.cog.toml` shows how a toml definition correlates to the JSON counterpart


## further references

[TOML spec](https://toml.io/en/v1.0.0-rc.3#keyvalue-pair)

[envsubst](https://www.gnu.org/software/gettext/manual/html_node/envsubst-Invocation.html)
