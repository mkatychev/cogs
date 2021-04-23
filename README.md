COGS: COnfiguration manaGement S
---
`cogs` is a cli tool that allows generation of configuration files through different references sources.

Sources of reference can include:

* local files
* remote files (through HTTP GET requests)
* [SOPS encrypted files][sops] (can also be remote)

`cogs` allows one to deduplicate sources of truth by maintaining a **source of reference** (the cog file) that points to the location of values (such as port numbers and password strings).

## installation:

### With `go`:

Clone this repo and `cd` into it.

```sh
go build -o $GOPATH/bin/ ./cmd/cogs
```

### Without `go`

`PL`atform can be Linux/Windows/Darwin:

```sh
PL="Darwin" VR="0.8.0" \
  curl -SLk \
  "github.com/Bestowinc/cogs/releases/download/v${VR}/cogs_${VR}_${PL}_x86_64.tar.gz" | \
  tar xvz -C /usr/local/bin cogs
```

## help string:

```
COGS COnfiguration manaGement S

Usage:
  cogs gen <ctx> <cog-file> [options]

Options:
  -h --help        Show this screen.
  --version        Show version.
  --no-enc, -n     Skips fetching encrypted vars.
  --no-decrypt	   Skipts decrypting encrypted vars.
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
name = "basic example" # every cog manifest should have a name key that corresponds to a string

# key value pairs for a context/ctx are defined under <ctx>.vars
# try running `cogs gen basic basic.cog.toml` to see what output cogs generates
[basic.vars]
var = "var_value"
other_var = "other_var_value"

# if <var>.path is given a string value,
# cogs will look for the key name of <var> in the file that that corresponds to the <var>.path key,
# returning the corresponding value
manifest_var.path = "./test_files/manifest.yaml"
# try removing manifest_var from "./test_files/manifest.yaml" and see what happens

# some variables can set an explicit key name to look for instead of defaulting to look for
# the key name "<var>":
# if <var>.name is defined then cogs will look for a key name that matches <var>.name
look_for_manifest_var.path = "./test_files/manifest.yaml"
look_for_manifest_var.name = "manifest_var"

# dangling variable names should return an error
# try uncommenting the line below and run `cogs gen basic basic.cog.toml`:
# empty_var.name = "some_name"
```

## example data:

The example data (in `./examples`) are ordered by increasing complexity and should be used as a tutorial. Run `cogs gen` on the files in the order below,
then read the file to see how the underlying logic is used.

1. basic example:
   * `cogs gen basic 1.basic.cog.toml`
1. HTTP example:
   * `cogs gen http 1.http.cog.toml`
1. secret values and paths example:
   * `gpg --import ./test_files/sops_functional_tests_key.asc` should be run to import the test private key used for encrypted dummy data
   * `cogs gen sops 2.secrets.cog.toml`
1. read types example:
   * `cogs gen kustomize 3.read_types.cog.toml`
1. advanced patterns example:
   * `cogs gen complex_json 4.advanced.cog.toml`
1. envsubst patterns example:
   * `NVIM=nvim cogs gen envsubst 5.envsubst.cog.toml --envsubst`

## `envsubst` cheatsheet:


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


## Notes and references:

`envsubst` warning: make sure that any environmental substition declarations allow a file to be parsed as TOML without the usage of the `--envsubst` flag:
```toml
# valid envsubst definitions can be placed anywhere string values are valid
["${ENV}".vars]
thing = "${THING_VAR}"
# the `${ENV}` below creates a TOML read error
[env.vars]${ENV}
thing = "${THING_VAR}"
```

### Further references
* [TOML spec](https://toml.io/en/v1.0.0-rc.3#keyvalue-pair)
* [`yq` expressions](https://mikefarah.gitbook.io/yq/)
* [envsubst](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html)

[sops]: https://github.com/mozilla/sops
