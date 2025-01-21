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

	# GO
	find pkg/ -type f -name "*.pb.go" -delete
	find internal/ -type f -name "*.pb.go" -delete
	protoc -Iproto --go_out=pkg/finance/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/finance/pb --go-grpc_out=pkg/finance/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/finance/pb proto/foreverbull/finance/*.proto
	protoc -Iproto --go_out=pkg/backtest/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/backtest/pb --go-grpc_out=pkg/backtest/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/backtest/pb proto/foreverbull/backtest/*.proto
	protoc -Iproto --go_out=pkg/service/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/service/pb --go-grpc_out=pkg/service/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/service/pb proto/foreverbull/service/*.proto
	protoc -Iproto --go_out=pkg/strategy/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/strategy/pb --go-grpc_out=pkg/strategy/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/strategy/pb proto/foreverbull/strategy/*.proto
	protoc -Iproto --go_out=internal/pb --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb  proto/foreverbull/common.proto
	@echo "Generated protobuf files"

mock-gen:
	find pkg/ -type f -name "mock_*.go" -delete
	mockery --all --inpackage

py-dist:
	cd client/foreverbull && rye build && cd ../..

TAG=latest
ZIPLINE_NAME=lhjnilsson/zipline:$(TAG)
FOREVERBULL_NAME=lhjnilsson/foreverbull:$(TAG)
GRAFANA_NAME=lhjnilsson/fb-grafana:$(TAG)

docker-images: py-dist
	docker build -t $(ZIPLINE_NAME) --build-arg FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl -f client/foreverbull_zipline/Dockerfile client/
	docker build -t $(FOREVERBULL_NAME) -f docker/Dockerfile .
	docker build -t $(GRAFANA_NAME) -f grafana/Dockerfile grafana/

env: docker-images
	(cd client && rye sync)
	(cd client && rye run fbull env stop)
	(cd client && rye run fbull env start --version $(TAG))

.PHONY: proto-gen, mock-gen
