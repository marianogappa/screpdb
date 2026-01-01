# Jet Models

This directory contains auto-generated models and table definitions from the database schema using [go-jet/jet](https://github.com/go-jet/jet).

## Generating Models

To generate the jet models, run:

```bash
make jet-generate POSTGRES_CONNECTION_STRING='host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable'
```

Or use the default connection string:

```bash
make jet-generate
```

The models will be generated in this directory (`internal/jet/`).

## Cleaning Generated Files

To remove all generated jet files:

```bash
make jet-clean
```

## Usage

The generated models are used in the postgres storage layer for type-safe database operations. The models are automatically generated from the database schema, so when you add or remove columns, you just need to:

1. Update your migration files
2. Run the migrations
3. Regenerate the jet models with `make jet-generate`

The code will automatically use the updated models.

