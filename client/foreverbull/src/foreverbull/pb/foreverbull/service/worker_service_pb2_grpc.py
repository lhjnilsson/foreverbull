# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import warnings

import grpc
from foreverbull.pb.foreverbull.service import (
    worker_service_pb2 as foreverbull_dot_service_dot_worker__service__pb2,
)

GRPC_GENERATED_VERSION = "1.66.1"
GRPC_VERSION = grpc.__version__
_version_not_supported = False

try:
    from grpc._utilities import first_version_is_lower

    _version_not_supported = first_version_is_lower(
        GRPC_VERSION, GRPC_GENERATED_VERSION
    )
except ImportError:
    _version_not_supported = True

if _version_not_supported:
    raise RuntimeError(
        f"The grpc package installed is at version {GRPC_VERSION},"
        + f" but the generated code in foreverbull/service/worker_service_pb2_grpc.py depends on"
        + f" grpcio>={GRPC_GENERATED_VERSION}."
        + f" Please upgrade your grpc module to grpcio>={GRPC_GENERATED_VERSION}"
        + f" or downgrade your generated code using grpcio-tools<={GRPC_VERSION}."
    )


class WorkerStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.GetServiceInfo = channel.unary_unary(
            "/foreverbull.service.Worker/GetServiceInfo",
            request_serializer=foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoRequest.SerializeToString,
            response_deserializer=foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoResponse.FromString,
            _registered_method=True,
        )
        self.ConfigureExecution = channel.unary_unary(
            "/foreverbull.service.Worker/ConfigureExecution",
            request_serializer=foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionResponse.FromString,
            _registered_method=True,
        )
        self.RunExecution = channel.unary_unary(
            "/foreverbull.service.Worker/RunExecution",
            request_serializer=foreverbull_dot_service_dot_worker__service__pb2.RunExecutionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_service_dot_worker__service__pb2.RunExecutionResponse.FromString,
            _registered_method=True,
        )


class WorkerServicer(object):
    """Missing associated documentation comment in .proto file."""

    def GetServiceInfo(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def ConfigureExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def RunExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")


def add_WorkerServicer_to_server(servicer, server):
    rpc_method_handlers = {
        "GetServiceInfo": grpc.unary_unary_rpc_method_handler(
            servicer.GetServiceInfo,
            request_deserializer=foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoRequest.FromString,
            response_serializer=foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoResponse.SerializeToString,
        ),
        "ConfigureExecution": grpc.unary_unary_rpc_method_handler(
            servicer.ConfigureExecution,
            request_deserializer=foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionRequest.FromString,
            response_serializer=foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionResponse.SerializeToString,
        ),
        "RunExecution": grpc.unary_unary_rpc_method_handler(
            servicer.RunExecution,
            request_deserializer=foreverbull_dot_service_dot_worker__service__pb2.RunExecutionRequest.FromString,
            response_serializer=foreverbull_dot_service_dot_worker__service__pb2.RunExecutionResponse.SerializeToString,
        ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
        "foreverbull.service.Worker", rpc_method_handlers
    )
    server.add_generic_rpc_handlers((generic_handler,))
    server.add_registered_method_handlers(
        "foreverbull.service.Worker", rpc_method_handlers
    )


# This class is part of an EXPERIMENTAL API.
class Worker(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def GetServiceInfo(
        request,
        target,
        options=(),
        channel_credentials=None,
        call_credentials=None,
        insecure=False,
        compression=None,
        wait_for_ready=None,
        timeout=None,
        metadata=None,
    ):
        return grpc.experimental.unary_unary(
            request,
            target,
            "/foreverbull.service.Worker/GetServiceInfo",
            foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoRequest.SerializeToString,
            foreverbull_dot_service_dot_worker__service__pb2.GetServiceInfoResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True,
        )

    @staticmethod
    def ConfigureExecution(
        request,
        target,
        options=(),
        channel_credentials=None,
        call_credentials=None,
        insecure=False,
        compression=None,
        wait_for_ready=None,
        timeout=None,
        metadata=None,
    ):
        return grpc.experimental.unary_unary(
            request,
            target,
            "/foreverbull.service.Worker/ConfigureExecution",
            foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionRequest.SerializeToString,
            foreverbull_dot_service_dot_worker__service__pb2.ConfigureExecutionResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True,
        )

    @staticmethod
    def RunExecution(
        request,
        target,
        options=(),
        channel_credentials=None,
        call_credentials=None,
        insecure=False,
        compression=None,
        wait_for_ready=None,
        timeout=None,
        metadata=None,
    ):
        return grpc.experimental.unary_unary(
            request,
            target,
            "/foreverbull.service.Worker/RunExecution",
            foreverbull_dot_service_dot_worker__service__pb2.RunExecutionRequest.SerializeToString,
            foreverbull_dot_service_dot_worker__service__pb2.RunExecutionResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True,
        )
