import grpc
import pytest

from foreverbull.gprc_service import new_grpc_server
from foreverbull.pb.foreverbull.service import worker_service_pb2
from foreverbull.pb.foreverbull.service import worker_service_pb2_grpc
from foreverbull.worker import WorkerPool


@pytest.fixture(scope="function")
def grpc_service(parallel_algo_file):
    algorithm, _, _ = parallel_algo_file
    with WorkerPool(algorithm._file_path) as pool:
        server = new_grpc_server(pool, algorithm)
        port = server.add_insecure_port("[::]:0")
        server.start()
        yield port
        server.stop(None)


@pytest.fixture
def stub(grpc_service):
    return worker_service_pb2_grpc.WorkerStub(grpc.insecure_channel(f"localhost:{grpc_service}"))


def test_configure_execution(stub, namespace_server, parallel_algo_file):
    file_name, configuration, _ = parallel_algo_file
    request = worker_service_pb2.ConfigureExecutionRequest(configuration=configuration)
    stub.ConfigureExecution(request)


def test_configure_and_run_execution(stub, namespace_server, parallel_algo_file):
    _, configuration, process = parallel_algo_file
    request = worker_service_pb2.ConfigureExecutionRequest(configuration=configuration)
    stub.ConfigureExecution(request)
    stub.RunExecution(worker_service_pb2.RunExecutionRequest())
    orders = process()
    assert orders
