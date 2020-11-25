## installation: 

With `go`:
* clone this repo, `cd` into it
* `go build -o $GOPATH/bin ./cmd/cogs`

Without `go`, `PL`atform can be Linux/Windows/Darwin:
```sh
PL="Darwin" VR="0.5.0" \
  curl -SLk \ 
  "github.com/Bestowinc/cogs/releases/download/v${VR}/cogs_${VR}_${PL}_x86_64.tar.gz" | \
  tar xvz -C /usr/local/bin cogs
```


```
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [options]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --envsubst, -e   Perform environmental substitution on the given cog file.
  --keys=<key,>    Include specific keys, comma separated.
  --not=<key,>     Exclude specific keys, comma separated.
  --out=<type>     Configuration output type [default: json].
                   <type>: json, toml, yaml, dotenv, raw.
  
  --export, -x     If --out=dotenv: Prepends "export " to each line.
  --preserve, -p   If --out=dotenv: Preserves variable casing.
  --sep=<sep>      If --out=raw:    Delimits values with a <sep>arator.
```

## annotated spec:

```toml
name = "basic_example"

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
# 4. ["path", "subpath"] - nothing will be inherited
var1.path = ["./test_files/manifest.yaml", "subpath"]
var2.path = []
var3.path = [[], "other_subpath"]
# dangling variable should return {"empty_var": ""} since only name override was defined
empty_var.name = "some_name"
# key value pairs for an encrypted context are defined under <ctx>.enc.vars
[sops.enc.vars]
yaml_enc.path = "./test_files/test.enc.yaml"
dotenv_enc = {path = "./test_files/test.enc.env", name = "DOTENV_ENC"}
json_enc.path = "./test_files/test.enc.json"

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
var3 = {path = [[], "jsonMap"], type = "json"}
```

## goals:

1. Allow a flat style of managing configurations across disparate contexts and different formats (plaintext vs. encrypted)
    * aggregates plaintext config values and SOPS secrets in one manifest
        - ex: local development vs. docker vs. production environments

1. Introduce an automated and cohesive way to validate and correlate configurations
    * `TODO`: allow a gradual introduction of new variable names by automating:
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

[envsubst](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) cheatsheet:


| __Expression__                  | __Meaning__    |
| -----------------               | -------------- |
| `${var}`                        | Value of var (same as `$var`)
| `${var-`${DEFAULT}`}`           | If var not set, evaluate expression as `${DEFAULT}`
| `${var:-`${DEFAULT}`}`          | If var not set or is empty, evaluate expression as `${DEFAULT}`
| `${var=`${DEFAULT}`}`           | If var not set, evaluate expression as `${DEFAULT}`
| `${var:=`${DEFAULT}`}`          | If var not set or is empty, evaluate expression as `${DEFAULT}`
| `$$var`                         | Escape expressions. Result will be `$var`.
| `${var^^}`                      | Uppercase value of `$var`
| `${var,,}`                      | Lowercase value of `$var`
| `${#var}`                       | Value of `$var` string length
| `${var^}`                       |
| `${var,}`                       |
| `${var:position}`               |
| `${var:position:length}`        |
| `${var#substring}`              |
| `${var##substring}`             |
| `${var%substring}`              |
| `${var%%substring}`             |
| `${var/substring/replacement}`  |
| `${var//substring/replacement}` |
| `${var/#substring/replacement}` |
| `${var/%substring/replacement}` |


Notes:
* `envsubst` warning, make sure that any `--envsubst` tags retain a file's membership as valid TOML:
```toml
# yes
[env.vars]
thing = "${THING_VAR}"

# NO
[sloppy.vars]${NO}
thing = "${THING_VAR}"
```
