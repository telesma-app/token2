.PHONY: lint

lint:
	@for module in . examples/*; do \
		(cd "$$module" && GOWORK=off golangci-lint run ./...) || exit; \
	done
