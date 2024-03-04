default: testacc

# Run acceptance tests
.PHONY: testacc lint
testacc: lint
	@[ -n "$(CLIENT_ID)" ] || (echo "CLIENT_ID must be set to run acceptance tests" && exit 1)
	@[ -n "$(CLIENT_SECRET)" ] || (echo "CLIENT_SECRET must be set to run acceptance tests" && exit 1)
	@[ -n "$(SUBDOMAIN)" ] || (echo "SUBDOMAIN must be set to run acceptance tests" && exit 1)
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

lint:
	golangci-lint run
