.PHONY: lint

lint:
	@for module in . examples/*; do \
		(cd "$$module" && golangci-lint run ./...) || exit; \
	done
