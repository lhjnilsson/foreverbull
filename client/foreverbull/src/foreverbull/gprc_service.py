from concurrent import futures

import grpc
from foreverbull.algorithm import WorkerPool
from foreverbull.models import Algorithm
from foreverbull.pb.service import service_pb2, service_pb2_grpc, worker_pb2, worker_pb2_grpc


class WorkerService(worker_pb2_grpc.WorkerServicer):
    def __init__(self, worker_pool: WorkerPool, algorithm: Algorithm):
        self._worker_pool = worker_pool
        self._algorithm = algorithm

    def GetServiceInfo(self, request, context):
        entity = self._algorithm.get_entity()
        return worker_pb2.GetServiceInfoResponse(
            algorithm=service_pb2.Algorithm(
                file_path=self._worker_pool._file_path,
                functions=[
                    service_pb2.Algorithm.Function(
                        name=function.name,
                        parameters=[service_pb2.Algorithm.FunctionParameter() for param in function.parameters],
                        parallelExecution=function.parallel_execution,
                        runFirst=function.run_first,
                        runLast=function.run_last,
                    )
                    for function in entity.functions
                ],
                namespaces=entity.namespaces,
            ),
        )

    def ConfigureExecution(self, request: worker_pb2.ConfigureExecutionRequest, context):
        self._worker_pool.configure_execution(request.configuration)
        return worker_pb2.ConfigureExecutionResponse()

    def RunExecution(self, request, context):
        self._worker_pool.run_execution(None)
        return worker_pb2.RunExecutionResponse()


def new_grpc_server(worker_pool: WorkerPool, algorithm: Algorithm, port=50055) -> grpc.Server:
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))
    service = WorkerService(worker_pool, algorithm)
    worker_pb2_grpc.add_WorkerServicer_to_server(service, server)
    server.add_insecure_port(f"[::]:{port}")
    return server


if __name__ == "__main__":
    foreverbull = Foreverbull(file_path=sys.argv[1])
    with foreverbull as fb:
        broker.service.update_instance(socket.gethostname(), True)
        signal.signal(signal.SIGINT, lambda x, y: fb._stop_event.set())
        signal.signal(signal.SIGTERM, lambda x, y: fb._stop_event.set())
        fb.join()
        broker.service.update_instance(socket.gethostname(), False)
