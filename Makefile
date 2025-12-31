.PHONY: jet-generate jet-clean help

# Default connection string - can be overridden with POSTGRES_CONNECTION_STRING env var
# Format: host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable
POSTGRES_CONNECTION_STRING ?= host=localhost port=5432 user=postgres password= dbname=screpdb sslmode=disable

# Helper function to extract value from connection string
get-param = $(shell echo "$(POSTGRES_CONNECTION_STRING)" | grep -oE '$(1)=[^ ]+' | cut -d= -f2 || echo "$(2)")

# Generate jet models from database schema
jet-generate:
	@echo "Generating jet models..."
	@echo "Connection string: $(POSTGRES_CONNECTION_STRING)"
	@jet -source=postgres \
		-host=$(call get-param,host,localhost) \
		-port=$(call get-param,port,5432) \
		-user=$(call get-param,user,postgres) \
		-password=$(call get-param,password,) \
		-dbname=$(call get-param,dbname,screpdb) \
		-sslmode=$(call get-param,sslmode,disable) \
		-schema=public \
		-path=internal/jet
	@echo "Jet models generated successfully in internal/jet/"

# Clean generated jet files
jet-clean:
	@echo "Cleaning jet generated files..."
	@rm -rf internal/jet
	@echo "Jet files cleaned"

# Help target
help:
	@echo "Available targets:"
	@echo "  jet-generate  - Generate jet models from database schema"
	@echo "  jet-clean     - Remove generated jet files"
	@echo ""
	@echo "Usage:"
	@echo "  make jet-generate POSTGRES_CONNECTION_STRING='host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable'"

