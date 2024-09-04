.py-protoc-gen:
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src --pyi_out=client/foreverbull/src --grpc_python_out=client/foreverbull/src proto/foreverbull/pb/finance/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src --pyi_out=client/foreverbull/src --grpc_python_out=client/foreverbull/src proto/foreverbull/pb/backtest/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src --pyi_out=client/foreverbull/src --grpc_python_out=client/foreverbull/src proto/foreverbull/pb/service/*.proto
	python -m grpc_tools.protoc -Iproto --python_out=client/foreverbull/src --pyi_out=client/foreverbull/src --grpc_python_out=client/foreverbull/src proto/foreverbull/pb/common.proto


.go-protoc-gen:
	protoc -Iproto --go_out=internal/pb/ --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb proto/foreverbull/pb/finance/*.proto
	protoc -Iproto --go_out=internal/pb/ --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb proto/foreverbull/pb/backtest/*.proto
	protoc -Iproto --go_out=internal/pb/ --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb proto/foreverbull/pb/service/*.proto
	protoc -Iproto --go_out=internal/pb/ --go_opt=module=github.com/lhjnilsson/foreverbull/internal/pb proto/foreverbull/pb/common.proto


proto-gen: .go-protoc-gen .py-protoc-gen
	@echo "Generated protobuf files"
