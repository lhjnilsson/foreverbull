import grpc
import pytest
from foreverbull.gprc_service import serre
from foreverbull.pb.service import service_pb2, service_pb2_grpc
from foreverbull.worker import WorkerPool


@pytest.fixture(scope="function")
def servicer(parallel_algo_file):
    file_name, _, _ = parallel_algo_file
    with WorkerPool(file_name) as pool:
        server = serre(pool)
        server.start()
        yield serre
        server.stop(None)


@pytest.fixture
def stub():
    return service_pb2_grpc.WorkerStub(grpc.insecure_channel("localhost:50055"))


def test_get_service_info(servicer, stub, parallel_algo_file):
    stub.GetServiceInfo(service_pb2.GetServiceInfoRequest())


def test_configure_execution(servicer, stub, namespace_server, parallel_algo_file):
    file_name, request, _ = parallel_algo_file
    stub.ConfigureExecution(request)


def test_configure_and_run_execution(servicer, stub, namespace_server, parallel_algo_file):
    file_name, request, process = parallel_algo_file

    stub.ConfigureExecution(request)
    stub.RunExecution(service_pb2.RunExecutionRequest())
    orders = process()
    assert orders


def test_stop_execution(servicer, stub, parallel_algo_file):
    stub.Stop(service_pb2.StopRequest())
