# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/pb/backtest/backtest.proto
# Protobuf Python Version: 5.27.1
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder

_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC, 5, 27, 1, "", "foreverbull/pb/backtest/backtest.proto"
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2

from foreverbull.pb.service import service_pb2 as foreverbull_dot_pb_dot_service_dot_service__pb2

DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(
    b'\n&foreverbull/pb/backtest/backtest.proto\x12\x17\x66oreverbull.pb.backtest\x1a\x1fgoogle/protobuf/timestamp.proto\x1a$foreverbull/pb/service/service.proto"K\n\x13NewExecutionRequest\x12\x34\n\talgorithm\x18\x01 \x01(\x0b\x32!.foreverbull.pb.service.Algorithm"\xaf\x01\n\x14NewExecutionResponse\x12\n\n\x02id\x18\x01 \x01(\t\x12.\n\nstart_date\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12,\n\x08\x65nd_date\x18\x03 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0f\n\x07symbols\x18\x04 \x03(\t\x12\x12\n\x05\x65rror\x18\x05 \x01(\tH\x00\x88\x01\x01\x42\x08\n\x06_error"\xc3\x01\n\x19\x43onfigureExecutionRequest\x12\x11\n\texecution\x18\x01 \x01(\t\x12.\n\nstart_date\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12,\n\x08\x65nd_date\x18\x03 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0f\n\x07symbols\x18\x04 \x03(\t\x12\x16\n\tbenchmark\x18\x05 \x01(\tH\x00\x88\x01\x01\x42\x0c\n\n_benchmark"x\n\x1a\x43onfigureExecutionResponse\x12\x11\n\texecution\x18\x01 \x01(\t\x12\x12\n\nbrokerPort\x18\x02 \x01(\x05\x12\x15\n\rnamespacePort\x18\x03 \x01(\x05\x12\x12\n\x05\x65rror\x18\x04 \x01(\tH\x00\x88\x01\x01\x42\x08\n\x06_error"(\n\x13RunExecutionRequest\x12\x11\n\texecution\x18\x01 \x01(\t"G\n\x14RunExecutionResponse\x12\x11\n\texecution\x18\x01 \x01(\t\x12\x12\n\x05\x65rror\x18\x02 \x01(\tH\x00\x88\x01\x01\x42\x08\n\x06_errorB8Z6github.com/lhjnilsson/foreverbull/internal/pb/backtestb\x06proto3'
)

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, "foreverbull.pb.backtest.backtest_pb2", _globals)
if not _descriptor._USE_C_DESCRIPTORS:
    _globals["DESCRIPTOR"]._loaded_options = None
    _globals["DESCRIPTOR"]._serialized_options = b"Z6github.com/lhjnilsson/foreverbull/internal/pb/backtest"
    _globals["_NEWEXECUTIONREQUEST"]._serialized_start = 138
    _globals["_NEWEXECUTIONREQUEST"]._serialized_end = 213
    _globals["_NEWEXECUTIONRESPONSE"]._serialized_start = 216
    _globals["_NEWEXECUTIONRESPONSE"]._serialized_end = 391
    _globals["_CONFIGUREEXECUTIONREQUEST"]._serialized_start = 394
    _globals["_CONFIGUREEXECUTIONREQUEST"]._serialized_end = 589
    _globals["_CONFIGUREEXECUTIONRESPONSE"]._serialized_start = 591
    _globals["_CONFIGUREEXECUTIONRESPONSE"]._serialized_end = 711
    _globals["_RUNEXECUTIONREQUEST"]._serialized_start = 713
    _globals["_RUNEXECUTIONREQUEST"]._serialized_end = 753
    _globals["_RUNEXECUTIONRESPONSE"]._serialized_start = 755
    _globals["_RUNEXECUTIONRESPONSE"]._serialized_end = 826
# @@protoc_insertion_point(module_scope)
