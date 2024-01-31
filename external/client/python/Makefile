PLATFORM ?= linux/amd64
TAG ?= latest

zipline:
	docker build --platform $(PLATFORM) -t quay.io/foreverbull/zipline:$(TAG) -f zipline.Dockerfile .

client:
	docker build --platform $(PLATFORM) -t quay.io/foreverbull/client:$(TAG) .

client_examples:
	docker build --no-cache --platform $(PLATFORM) -t quay.io/foreverbull/client-monkey-example:$(TAG) --no-cache examples/monkey

publish:
	docker push	quay.io/foreverbull/zipline:$(TAG)
	docker push	quay.io/foreverbull/client:$(TAG)
	docker push	quay.io/foreverbull/client-monkey-example:$(TAG)
	docker push	quay.io/foreverbull/client-ema-example:$(TAG)

all: zipline client examples
