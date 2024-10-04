TAG=latest
PARALLEL_NAME=lhjnilsson/client-parallel:$(TAG)
NON_PARALLEL_NAME=lhjnilsson/client-non-parallel:$(TAG)
NAMESPACE_NAME=lhjnilsson/client-namespace:$(TAG)
ZIPLINE_NAME=lhjnilsson/zipline:$(TAG)

.build-py-foreverbull:
	cd foreverbull && rye build && cd ..

.build-docker-parallel: .build-py-foreverbull
	docker build -t $(PARALLEL_NAME) --build-arg ALGO_FILE=example_algorithms/src/example_algorithms/parallel.py --build-arg FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl .

.build-docker-non-parallel: .build-py-foreverbull
	docker build -t $(NON_PARALLEL_NAME) --build-arg ALGO_FILE=example_algorithms/src/example_algorithms/non_parallel.py --build-arg FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl .

.build-docker-namespace: .build-py-foreverbull
	docker build -t $(NAMESPACE_NAME) --build-arg ALGO_FILE=example_algorithms/src/example_algorithms/multistep_with_namespace.py --build-arg FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl .

.build-docker-clients: .build-docker-parallel .build-docker-non-parallel .build-docker-namespace

.build-docker-zipline: .build-py-foreverbull
	docker build -t $(ZIPLINE_NAME) --build-arg FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl -f foreverbull_zipline/Dockerfile .

all: .build-docker-clients .build-docker-zipline
	@echo "export IMAGES=$(PARALLEL_NAME),$(NON_PARALLEL_NAME),$(NAMESPACE_NAME)"
	@echo "export BACKTEST_IMAGE=$(ZIPLINE_NAME)"
