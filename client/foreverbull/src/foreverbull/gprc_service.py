from concurrent import futures
from typing import Generator

import grpc
from foreverbull.algorithm import WorkerPool
from foreverbull.models import Algorithm
from foreverbull.pb.service import service_pb2, service_pb2_grpc


class WorkerService(service_pb2_grpc.WorkerServicer):
    def __init__(self, worker_pool: WorkerPool):
        self._worker_pool = worker_pool

    def GetServiceInfo(self, request, context):
        return service_pb2.GetServiceInfoResponse(
            serviceType="worker",
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
