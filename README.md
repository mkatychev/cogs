# COGS: COnfiguration manaGement S.

goals:

1. Allow a flat style of managing configurations across disparate environments and different formats (plaintext vs. encrypted)
    * aggregates plaintext config values and SOPS secrets in one manifest
        - ex: local development vs. docker vs. production environments

1. Introduce an automanted and cohesive way to change configurations wholesale
    * allow a gradual introduction of new variable names by automating:
        - introduction of new name for same value (`DB_SECRETS -> DATABASE_SECRETS`)
        - and deprecation of old name (managing deletion of old `DB_SECRETS` references)
        - ex draft command: `cogs migrate --commit DB_SECRETS DATABASE_SECRETS`

aims to support:

- microservice configuration
- viper package
- sops secrets
- docker env configs
