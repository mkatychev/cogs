## installation: 

With `go`:
* clone this repo, `cd` into it
* `go build -o $GOPATH/bin ./cmd/cogs`

Without `go`, `PL`atform can be Linux/Windows/Darwin:
```sh
PL="Darwin" VR="0.7.1" \
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


`cogs gen` - outputs a flat and serialized K:V array

## annotated spec:

```toml
name = "basic_example"

# key value pairs for a context are defined under <ctx>.vars
[docker.vars]
var = "var_value"
other_var = "other_var_value"

[sops]
# a default path to be inherited can be defined under <ctx>.path
path = ["./test_files/manifest.yaml", ".subpath"]
[sops.vars]
# a <var>.path key can map to four valid types:
# 1. path value is "string_value" - indicating a single file to look through
# 2. path value is [] - thus <ctx>.path will be inherited
# 3. path value is a ["two_index, "array"] - either index being [] or "string_value":
# -  [[], "subpath"] - path will be inherited from <ctx>.path if present
# -  ["path", []] - subpath will be inherited from <ctx>.path if present
# 4. ["path", "subpath"] - nothing will be inherited
var1.path = ["./test_files/manifest.yaml", ".subpath"]
var2.path = []
var3.path = [[], ".other_subpath"]
# dangling variable should return {"empty_var": ""} since only name override was defined
empty_var.name = "some_name"
# key value pairs for an encrypted context are defined under <ctx>.enc.vars
[sops.enc.vars]
yaml_enc.path = "./test_files/test.enc.yaml"
dotenv_enc = {path = "./test_files/test.enc.env", name = "DOTENV_ENC"}
json_enc.path = "./test_files/test.enc.json"

[kustomize]
path = ["./test_files/kustomization.yaml", ".configMapGenerator.[0].literals"]
# a default deserialization path to be inherited can be defined under <ctx>.path
# once <var>.path has been traversed, attempt to deserialize the returned object
# as if it was in dotenv format
type = "dotenv"
[kustomize.vars]
# var1.name = "VAR_1" means that the key alias "VAR_1" will
# be searched for to retrieve the var1 value
var1 = {path = [], name = "VAR_1"}
var2 = {path = [], name = "VAR_2"}
var3 = {path = [[], ".jsonMap"], type = "json"}
```

## running example data locally:
* `gpg --import ./test_files/sops_functional_tests_key.asc` should be run to import the test private key used for encrypted dummy data
* Building binary locally : `go build -o $GOPATH/bin ./cmd/cogs`
* Kustomize style var retrieval: `cogs gen  kustomize ./basic.cog.toml`
* Encrypted var retrieval: `cogs gen sops ./basic.cog.toml`
* `some-service.cog.toml` shows how a toml definition correlates to the JSON counterpart


## further references

[TOML spec](https://toml.io/en/v1.0.0-rc.3#keyvalue-pair)
[envsubst](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) cheatsheet:
[yq expressions](https://mikefarah.gitbook.io/yq/)


| __Expression__                | __Meaning__                                                     |
| -----------------             | --------------                                                  |
| `${var}`                      | Value of `$var`
| `${var-${DEFAULT}}`           | If `$var` is not set, evaluate expression as `${DEFAULT}`
| `${var:-${DEFAULT}}`          | If `$var` is not set or is empty, evaluate expression as `${DEFAULT}`
| `${var=${DEFAULT}}`           | If `$var` is not set, evaluate expression as `${DEFAULT}`
| `${var:=${DEFAULT}}`          | If `$var` is not set or is empty, evaluate expression as `${DEFAULT}`
| `$$var`                       | Escape expressions. Result will be the string `$var`
| `${var^}`                     | Uppercase first character of `$var`
| `${var^^}`                    | Uppercase all characters in `$var`
| `${var,}`                     | Lowercase first character of `$var`
| `${var,,}`                    | Lowercase all characters in `$var`
| `${#var}`                     | String length of `$var`
| `${var:n}`                    | Offset `$var` `n` characters from start
| `${var: -n}`                  | Offset `$var` `n` characters from end
| `${var:n:len}`                | Offset `$var` `n` characters with max length of `len`
| `${var#pattern}`              | Strip shortest `pattern` match from start
| `${var##pattern}`             | Strip longest `pattern` match from start
| `${var%pattern}`              | Strip shortest `pattern` match from end
| `${var%%pattern}`             | Strip longest `pattern` match from end
| `${var/pattern/replacement}`  | Replace as few `pattern` matches as possible with `replacement`
| `${var//pattern/replacement}` | Replace as many `pattern` matches as possible with `replacement`
| `${var/#pattern/replacement}` | Replace `pattern` match with `replacement` from `$var` start
| `${var/%pattern/replacement}` | Replace `pattern` match with `replacement` from `$var` end


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
