
.PHONY: packr
packr:
	@packr -i ./store

.PHONY: build
build: packr
	@go build main.go
