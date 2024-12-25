.py-protoc-gen:
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

.go-protoc-gen:
	#find pkg/ -type f -name "pb.go" -delete
	find pkg/ -type f -name "*.pb.go" -delete
	protoc -Iproto --go_out=pkg/finance/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/finance/pb --go-grpc_out=pkg/finance/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/finance/pb proto/foreverbull/finance/*.proto
	protoc -Iproto --go_out=pkg/backtest/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/backtest/pb --go-grpc_out=pkg/backtest/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/backtest/pb proto/foreverbull/backtest/*.proto
	protoc -Iproto --go_out=pkg/service/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/service/pb --go-grpc_out=pkg/service/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/service/pb proto/foreverbull/service/*.proto
	protoc -Iproto --go_out=pkg/strategy/pb --go_opt=module=github.com/lhjnilsson/foreverbull/pkg/strategy/pb --go-grpc_out=pkg/strategy/pb --go-grpc_opt=module=github.com/lhjnilsson/foreverbull/pkg/strategy/pb proto/foreverbull/strategy/*.proto
	protoc -Iproto --go_out=internal/pb --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb  proto/foreverbull/common.proto


proto-gen: .go-protoc-gen .py-protoc-gen
	@echo "Generated protobuf files"

mock-gen:
	find pkg/ -type f -name "mock_*.go" -delete
	mockery --all --inpackage
