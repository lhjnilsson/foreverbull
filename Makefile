MODULES ?= . ./tests /external /docs

COVER_MODULES ?= $(shell go list ./... | grep -v /tests/ | grep -v /external/ | grep -v /docs/)

unittests:
	@echo "Running unit tests..."
	go test $(shell go list ./... | grep -v /tests/) -v -race -failfast -p 1 -coverprofile=coverage.out -covermode=atomic

cover:
	go tool cover -html=coverage.out -o coverage.html

update_modules:
	@echo "Updating modules..."
	@go get -u -v ./... 
	@go mod tidy
