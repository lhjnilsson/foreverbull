.PHONY: proto-gen mock-gen py-dist zipline-image foreverbull-image grafana-image docker-images env

proto-gen:
	# Python
	find client/foreverbull/src/foreverbull/pb/ -type f -name "*_pb2*" -delete
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/foreverbull/finance/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/foreverbull/backtest/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/foreverbull/service/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/foreverbull/strategy/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/foreverbull/common.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src/foreverbull/pb --pyi_out=client/foreverbull/src/foreverbull/pb --grpc_python_out=client/foreverbull/src/foreverbull/pb proto/buf/validate/validate.proto
	# Update imports, could maybe be solved by organizing the proto files in a better way
	find client/foreverbull/src/foreverbull/pb -name "*_pb2*" -exec sed -i '' 's/from foreverbull\./from foreverbull\.pb\.foreverbull./g' {} \;
	find client/foreverbull/src/foreverbull/pb -name "*_pb2*" -exec sed -i '' 's/from foreverbull import common_pb2/from foreverbull\.pb\.foreverbull import common_pb2/g' {} \;
	find client/foreverbull/src/foreverbull/pb -name "*_pb2*" -exec sed -i '' 's/from buf\.validate import/from foreverbull\.pb\.buf\.validate import/g' {} \;

	# Go
	find pkg/ -type f -name "*.pb.go" -delete
	find internal/ -type f -name "*.pb.go" -delete
	protoc -Iproto --go_out=pkg/pb/finance --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/finance --go-grpc_out=pkg/pb/finance --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/finance proto/foreverbull/finance/*.proto
	protoc -Iproto --go_out=pkg/pb/backtest --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/backtest --go-grpc_out=pkg/pb/backtest --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/backtest proto/foreverbull/backtest/*.proto
	protoc -Iproto --go_out=pkg/pb/service --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/service --go-grpc_out=pkg/pb/service --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/service proto/foreverbull/service/*.proto
	protoc -Iproto --go_out=pkg/pb/strategy --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/strategy --go-grpc_out=pkg/pb/strategy --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb/strategy proto/foreverbull/strategy/*.proto
	protoc -Iproto --go_out=pkg/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/pb proto/foreverbull/common.proto
	@echo "Generated protobuf files"

mock-gen:
	find pkg/ -type f -name "mock_*.go" -delete
	mockery --all --inpackage

TAG=local
ZIPLINE_NAME=lhjnilsson/zipline:$(TAG)
FOREVERBULL_NAME=lhjnilsson/foreverbull:$(TAG)
GRAFANA_NAME=lhjnilsson/fb-grafana:$(TAG)

py-dist:
	@if [ ! -f "client/dist/foreverbull-0.0.1-py3-none-any.whl" ] || [ -n "$$(git status --porcelain client/foreverbull)" ]; then \
	    echo "Building Python distribution..."; \
	    cd client/foreverbull && rye build && cd ../..; \
	else \
	    echo "No changes detected. Skipping Python distribution build."; \
	fi

define build_docker_image
    @if [ -z "$$(docker images -q $(1) 2> /dev/null)" ]; then \
        echo "Docker image $(1) does not exist. Building..."; \
        docker build -t $(1) -f $(2) $(3); \
    elif [ -n "$$(git status --porcelain $(3))" ]; then \
        echo "Unstaged changes detected in $(3). Rebuilding Docker container."; \
        docker build -t $(1) -f $(2) $(3); \
    else \
        echo "No unstaged changes in $(3). Using cached Docker image."; \
    fi
endef

zipline-image:
	$(call build_docker_image,$(ZIPLINE_NAME),client/foreverbull_zipline/Dockerfile,client/foreverbull_zipline)

foreverbull-image:
	$(call build_docker_image,$(FOREVERBULL_NAME),docker/Dockerfile,.)

grafana-image:
	$(call build_docker_image,$(GRAFANA_NAME),grafana/Dockerfile,grafana/)

docker-images: foreverbull-image grafana-image

env: py-dist zipline-image foreverbull-image grafana-image
	(cd client && rye sync)
	(cd client && rye run fbull env stop)
	(cd client && rye run fbull env start --version $(TAG))
