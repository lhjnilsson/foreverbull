from foreverbull.models import Algorithm
from foreverbull.pb.service import service_pb2_grpc


class Foreverbull(service_pb2_grpc.ServiceServicer):
    def __init__(self, file_path: str | None = None, executors=2):
        self.file_path = file_path
        self.executors = executors

        self._session = None
        if self.file_path:
            Algorithm.from_file_path(self.file_path)

        self._worker_surveyor_address = "ipc:///tmp/worker_pool.ipc"
        self._worker_surveyor_socket: pynng.Surveyor0 | None = None
        self._worker_states_address = "ipc:///tmp/worker_states.ipc"
        self._worker_states_socket: pynng.Sub0 | None = None
        self._stop_event: synchronize.Event | None = None
        self._workers = []
        self.logger = logging.getLogger(__name__)

    def GetServiceInfo(self, request, context):
        pass

    def ConfigureExecution(self, request, context):
        pass

    def Execute(self, request, context):
        pass

    def Stop(self, request, context):
        pass
