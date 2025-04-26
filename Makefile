TEST_FLAGS := -v ./...
LINT_FLAGS := ./...

.PHONY: test
test:
	go test $(TEST_FLAGS)

.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, installing..."; \
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run $(LINT_FLAGS)
