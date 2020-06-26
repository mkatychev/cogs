# COGS: COnfiguration manaGement S

goals:

1. Allow a flat style of managing configurations across disparate environments and different formats (plaintext vs. encrypted)
    * aggregates plaintext config values and SOPS secrets in one manifest
        - ex: local development vs. docker vs. production environments

1. Introduce an automanted and cohesive way to change configurations wholesale
    * allow a gradual introduction of new variable names by automating:
        - introduction of new name for same value (`DB_SECRETS -> DATABASE_SECRETS`)
        - and deprecation of old name (managing deletion of old `DB_SECRETS` references)

scope of support:

- microservice configuration
- parse YAML manifests
- valid [viper package](https://github.com/spf13/viper) input (so able to output TOML & JSON)
- [SOPS secret management](https://github.com/mozilla/sops)
- [docker-compose](https://github.com/docker/compose) YAML env config scheme

## subcommands

### `cogs generate`
* `cogs generate <env_name> ./service-name.cog.yaml [--out=(json|toml)]`
- outputs a flat JSON/TOML K:V array

### `cogs migrate`
* `cogs migrate <OLD_KEY_NAME> <NEW_KEY_NAME> [<envs>...]`
* `cogs migrate --commit <OLD_KEY_NAME> <NEW_KEY_NAME> (<envs>...)`

Aims to allow a gradual and automated migration of key names without risking sensitive environments

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
