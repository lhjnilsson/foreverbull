# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""


import grpc
from foreverbull.pb.backtest import broker_pb2 as foreverbull_dot_pb_dot_backtest_dot_broker__pb2

GRPC_GENERATED_VERSION = "1.66.1"
GRPC_VERSION = grpc.__version__
_version_not_supported = False

try:
    from grpc._utilities import first_version_is_lower

    _version_not_supported = first_version_is_lower(GRPC_VERSION, GRPC_GENERATED_VERSION)
except ImportError:
    _version_not_supported = True

if _version_not_supported:
    raise RuntimeError(
        f"The grpc package installed is at version {GRPC_VERSION},"
        + " but the generated code in foreverbull/pb/backtest/broker_pb2_grpc.py depends on"
        + f" grpcio>={GRPC_GENERATED_VERSION}."
        + f" Please upgrade your grpc module to grpcio>={GRPC_GENERATED_VERSION}"
        + f" or downgrade your generated code using grpcio-tools<={GRPC_VERSION}."
    )


class BrokerStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.GetBacktest = channel.unary_unary(
            "/foreverbull.pb.backtest.Broker/GetBacktest",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestResponse.FromString,
            _registered_method=True,
        )
        self.CreateSession = channel.unary_unary(
            "/foreverbull.pb.backtest.Broker/CreateSession",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionResponse.FromString,
            _registered_method=True,
        )
        self.GetSession = channel.unary_unary(
            "/foreverbull.pb.backtest.Broker/GetSession",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionResponse.FromString,
            _registered_method=True,
        )
        self.CreateExecution = channel.unary_unary(
            "/foreverbull.pb.backtest.Broker/CreateExecution",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionResponse.FromString,
            _registered_method=True,
        )
        self.RunExecution = channel.unary_stream(
            "/foreverbull.pb.backtest.Broker/RunExecution",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionResponse.FromString,
            _registered_method=True,
        )
        self.GetExecution = channel.unary_unary(
            "/foreverbull.pb.backtest.Broker/GetExecution",
            request_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionRequest.SerializeToString,
            response_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionResponse.FromString,
            _registered_method=True,
        )


class BrokerServicer(object):
    """Missing associated documentation comment in .proto file."""

    def GetBacktest(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def CreateSession(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def GetSession(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def CreateExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def RunExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def GetExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")


def add_BrokerServicer_to_server(servicer, server):
    rpc_method_handlers = {
        "GetBacktest": grpc.unary_unary_rpc_method_handler(
            servicer.GetBacktest,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestResponse.SerializeToString,
        ),
        "CreateSession": grpc.unary_unary_rpc_method_handler(
            servicer.CreateSession,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionResponse.SerializeToString,
        ),
        "GetSession": grpc.unary_unary_rpc_method_handler(
            servicer.GetSession,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionResponse.SerializeToString,
        ),
        "CreateExecution": grpc.unary_unary_rpc_method_handler(
            servicer.CreateExecution,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionResponse.SerializeToString,
        ),
        "RunExecution": grpc.unary_stream_rpc_method_handler(
            servicer.RunExecution,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionResponse.SerializeToString,
        ),
        "GetExecution": grpc.unary_unary_rpc_method_handler(
            servicer.GetExecution,
            request_deserializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionRequest.FromString,
            response_serializer=foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionResponse.SerializeToString,
        ),
    }
    generic_handler = grpc.method_handlers_generic_handler("foreverbull.pb.backtest.Broker", rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))
    server.add_registered_method_handlers("foreverbull.pb.backtest.Broker", rpc_method_handlers)


# This class is part of an EXPERIMENTAL API.
class Broker(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def GetBacktest(
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
            "/foreverbull.pb.backtest.Broker/GetBacktest",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetBacktestResponse.FromString,
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
    def CreateSession(
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
            "/foreverbull.pb.backtest.Broker/CreateSession",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateSessionResponse.FromString,
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
    def GetSession(
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
            "/foreverbull.pb.backtest.Broker/GetSession",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetSessionResponse.FromString,
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
    def CreateExecution(
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
            "/foreverbull.pb.backtest.Broker/CreateExecution",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.CreateExecutionResponse.FromString,
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
        return grpc.experimental.unary_stream(
            request,
            target,
            "/foreverbull.pb.backtest.Broker/RunExecution",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.RunExecutionResponse.FromString,
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
    def GetExecution(
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
            "/foreverbull.pb.backtest.Broker/GetExecution",
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionRequest.SerializeToString,
            foreverbull_dot_pb_dot_backtest_dot_broker__pb2.GetExecutionResponse.FromString,
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
