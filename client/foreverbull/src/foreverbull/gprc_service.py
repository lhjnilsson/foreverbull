from concurrent import futures

import grpc

from foreverbull.algorithm import WorkerPool
from foreverbull.models import Algorithm
from foreverbull.pb.foreverbull.service import worker_service_pb2
from foreverbull.pb.foreverbull.service import worker_service_pb2_grpc


class WorkerService(worker_service_pb2_grpc.WorkerServicer):
    def __init__(self, worker_pool: WorkerPool, algorithm: Algorithm):
        self._worker_pool = worker_pool
        self._algorithm = algorithm

    def ConfigureExecution(self, request: worker_service_pb2.ConfigureExecutionRequest, context):
        self._worker_pool.configure_execution(request.configuration)
        return worker_service_pb2.ConfigureExecutionResponse()

    def RunExecution(self, request, context):
        self._worker_pool.run_execution(None)  # type: ignore
        return worker_service_pb2.RunExecutionResponse()


def new_grpc_server(worker_pool: WorkerPool, algorithm: Algorithm) -> grpc.Server:
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))
    service = WorkerService(worker_pool, algorithm)
    worker_service_pb2_grpc.add_WorkerServicer_to_server(service, server)
    return server


if __name__ == "__main__":
    """
    foreverbull = Foreverbull(file_path=sys.argv[1])
    with foreverbull as fb:
        broker.service.update_instance(socket.gethostname(), True)
        signal.signal(signal.SIGINT, lambda x, y: fb._stop_event.set())
        signal.signal(signal.SIGTERM, lambda x, y: fb._stop_event.set())
        fb.join()
        broker.service.update_instance(socket.gethostname(), False)
    """
