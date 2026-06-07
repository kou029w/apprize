.PHONY: help
help:
	@cat README.md

.PHONY: build
build:
	go build

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	$(RM) apprize

testdata/swagger.yaml:
	curl -sSf -o $@ https://raw.githubusercontent.com/caronc/apprise-api/refs/heads/master/swagger.yaml
