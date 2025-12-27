# LLM Guidelines for screpdb

## Generated Files - DO NOT MODIFY DIRECTLY

This codebase uses code generation tools. **Never modify generated files directly**. Instead, modify the source files and regenerate.

### SQLC Generated Files

The `internal/dashboard/dashdb/` package contains **generated code** from SQLC. These files are automatically generated from SQL queries.

**Source files (modify these):**
- `internal/dashboard/sqlc/queries.sql` - SQL queries with sqlc annotations
- `internal/dashboard/migrations/` - Database migration files

**Generated files (DO NOT modify):**
- `internal/dashboard/dashdb/models.go` - Generated Go models
- `internal/dashboard/dashdb/queries.sql.go` - Generated query functions

### How to Make Changes to Database Queries

1. **Modify the source SQL file**: Edit `internal/dashboard/sqlc/queries.sql`
2. **Regenerate the code**: Run `sqlc generate` from the `internal/dashboard/` directory:
   ```bash
   cd internal/dashboard
   sqlc generate
   ```
3. **Verify**: Check that the generated files in `dashdb/` have been updated

### Important Notes

- Always modify the `.sql` source files, never the `.go` generated files
- After modifying `queries.sql`, you MUST run `sqlc generate` before the changes take effect
- The generated files will be overwritten on the next generation, so any manual edits will be lost
- If you need to add custom logic, add it in separate non-generated files (e.g., `dashboard.go`)

### Other Generated Files

If this codebase uses other code generators (e.g., `go generate`, protobuf, etc.), follow the same principle:
- Find the source files
- Modify the source files
- Run the appropriate generation command
- Never modify generated files directly

