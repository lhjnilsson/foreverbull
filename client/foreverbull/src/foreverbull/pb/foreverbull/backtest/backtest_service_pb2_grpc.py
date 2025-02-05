# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import grpc
import warnings

from foreverbull.pb.foreverbull.backtest import backtest_service_pb2 as foreverbull_dot_backtest_dot_backtest__service__pb2

GRPC_GENERATED_VERSION = '1.66.1'
GRPC_VERSION = grpc.__version__
_version_not_supported = False

try:
    from grpc._utilities import first_version_is_lower
    _version_not_supported = first_version_is_lower(GRPC_VERSION, GRPC_GENERATED_VERSION)
except ImportError:
    _version_not_supported = True

if _version_not_supported:
    raise RuntimeError(
        f'The grpc package installed is at version {GRPC_VERSION},'
        + f' but the generated code in foreverbull/backtest/backtest_service_pb2_grpc.py depends on'
        + f' grpcio>={GRPC_GENERATED_VERSION}.'
        + f' Please upgrade your grpc module to grpcio>={GRPC_GENERATED_VERSION}'
        + f' or downgrade your generated code using grpcio-tools<={GRPC_VERSION}.'
    )


class BacktestServicerStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.ListBacktests = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/ListBacktests',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsResponse.FromString,
                _registered_method=True)
        self.CreateBacktest = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/CreateBacktest',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestResponse.FromString,
                _registered_method=True)
        self.GetBacktest = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/GetBacktest',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestResponse.FromString,
                _registered_method=True)
        self.CreateSession = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/CreateSession',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionResponse.FromString,
                _registered_method=True)
        self.GetSession = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/GetSession',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionResponse.FromString,
                _registered_method=True)
        self.ListExecutions = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/ListExecutions',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsResponse.FromString,
                _registered_method=True)
        self.GetExecution = channel.unary_unary(
                '/foreverbull.backtest.BacktestServicer/GetExecution',
                request_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionRequest.SerializeToString,
                response_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionResponse.FromString,
                _registered_method=True)


class BacktestServicerServicer(object):
    """Missing associated documentation comment in .proto file."""

    def ListBacktests(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def CreateBacktest(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetBacktest(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def CreateSession(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetSession(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def ListExecutions(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetExecution(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')


def add_BacktestServicerServicer_to_server(servicer, server):
    rpc_method_handlers = {
            'ListBacktests': grpc.unary_unary_rpc_method_handler(
                    servicer.ListBacktests,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsResponse.SerializeToString,
            ),
            'CreateBacktest': grpc.unary_unary_rpc_method_handler(
                    servicer.CreateBacktest,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestResponse.SerializeToString,
            ),
            'GetBacktest': grpc.unary_unary_rpc_method_handler(
                    servicer.GetBacktest,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestResponse.SerializeToString,
            ),
            'CreateSession': grpc.unary_unary_rpc_method_handler(
                    servicer.CreateSession,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionResponse.SerializeToString,
            ),
            'GetSession': grpc.unary_unary_rpc_method_handler(
                    servicer.GetSession,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionResponse.SerializeToString,
            ),
            'ListExecutions': grpc.unary_unary_rpc_method_handler(
                    servicer.ListExecutions,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsResponse.SerializeToString,
            ),
            'GetExecution': grpc.unary_unary_rpc_method_handler(
                    servicer.GetExecution,
                    request_deserializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionRequest.FromString,
                    response_serializer=foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionResponse.SerializeToString,
            ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
            'foreverbull.backtest.BacktestServicer', rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))
    server.add_registered_method_handlers('foreverbull.backtest.BacktestServicer', rpc_method_handlers)


 # This class is part of an EXPERIMENTAL API.
class BacktestServicer(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def ListBacktests(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/ListBacktests',
            foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.ListBacktestsResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def CreateBacktest(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/CreateBacktest',
            foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.CreateBacktestResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def GetBacktest(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/GetBacktest',
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetBacktestResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def CreateSession(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/CreateSession',
            foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.CreateSessionResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def GetSession(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/GetSession',
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetSessionResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def ListExecutions(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/ListExecutions',
            foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.ListExecutionsResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def GetExecution(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/foreverbull.backtest.BacktestServicer/GetExecution',
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionRequest.SerializeToString,
            foreverbull_dot_backtest_dot_backtest__service__pb2.GetExecutionResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)
