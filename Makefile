export GO_VERSION=$(go version | awk '{print $3;}')

.PHONY: go-test
go-test:
	@go test -race -v -cover ./...

# Release
.PHONY: release
release:
	@goreleaser --rm-dist

# Create dist only locally
.PHONY: release-check
release-check:
	@goreleaser --snapshot --skip-publish --rm-dist

# Run without without publishing
.PHONY: release-dry-run
release-dry-run:
	@goreleaser release --skip-publish --rm-dist
