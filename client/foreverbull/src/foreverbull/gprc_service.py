from concurrent import futures

import grpc
from foreverbull.algorithm import WorkerPool
from foreverbull.models import Algorithm
from foreverbull.pb.service import service_pb2, service_pb2_grpc


class WorkerService(service_pb2_grpc.WorkerServicer):
    def __init__(self, worker_pool: WorkerPool, algorithm: Algorithm):
        self._worker_pool = worker_pool
        self._algorithm = algorithm

    def GetServiceInfo(self, request, context):
        entity = self._algorithm.get_entity()
        return service_pb2.GetServiceInfoResponse(
            serviceType="worker",
            version="0.0.0",
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

    def ConfigureExecution(self, request, context):
        return self._worker_pool.configure_execution(request)

    def RunExecution(self, request, context):
        return self._worker_pool.run_execution(request, None)

    def Stop(self, request, context):
        return service_pb2.StopResponse()


def serre(worker_pool: WorkerPool) -> grpc.Server:
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))
    service = WorkerService(worker_pool)
    service_pb2_grpc.add_WorkerServicer_to_server(service, server)
    server.add_insecure_port("[::]:50055")
    return server


def main():
    foreverbull = Foreverbull(file_path=sys.argv[1])
    with foreverbull as fb:
        broker.service.update_instance(socket.gethostname(), True)
        signal.signal(signal.SIGINT, lambda x, y: fb._stop_event.set())
        signal.signal(signal.SIGTERM, lambda x, y: fb._stop_event.set())
        fb.join()
        broker.service.update_instance(socket.gethostname(), False)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 foreverbull/_run_instance.py <file_path>")
        exit(1)
    set_start_method("spawn")
    main()
