# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/backtest/backtest.proto
# Protobuf Python Version: 5.27.2
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder

_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC, 5, 27, 2, "", "foreverbull/backtest/backtest.proto"
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2

DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(
    b'\n#foreverbull/backtest/backtest.proto\x12\x14\x66oreverbull.backtest\x1a\x1fgoogle/protobuf/timestamp.proto"\xab\x03\n\x08\x42\x61\x63ktest\x12\x0c\n\x04name\x18\x01 \x01(\t\x12.\n\nstart_date\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12,\n\x08\x65nd_date\x18\x03 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0f\n\x07symbols\x18\x04 \x03(\t\x12\x16\n\tbenchmark\x18\x05 \x01(\tH\x00\x88\x01\x01\x12\x37\n\x08statuses\x18\x06 \x03(\x0b\x32%.foreverbull.backtest.Backtest.Status\x1a\xc2\x01\n\x06Status\x12<\n\x06status\x18\x01 \x01(\x0e\x32,.foreverbull.backtest.Backtest.Status.Status\x12\x12\n\x05\x65rror\x18\x02 \x01(\tH\x00\x88\x01\x01\x12/\n\x0boccurred_at\x18\x03 \x01(\x0b\x32\x1a.google.protobuf.Timestamp"+\n\x06Status\x12\x0b\n\x07\x43REATED\x10\x00\x12\t\n\x05READY\x10\x01\x12\t\n\x05\x45RROR\x10\x02\x42\x08\n\x06_errorB\x0c\n\n_benchmarkB3Z1github.com/lhjnilsson/foreverbull/pkg/backtest/pbb\x06proto3'
)

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(
    DESCRIPTOR, "foreverbull.backtest.backtest_pb2", _globals
)
if not _descriptor._USE_C_DESCRIPTORS:
    _globals["DESCRIPTOR"]._loaded_options = None
    _globals["DESCRIPTOR"]._serialized_options = (
        b"Z1github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
    )
    _globals["_BACKTEST"]._serialized_start = 95
    _globals["_BACKTEST"]._serialized_end = 522
    _globals["_BACKTEST_STATUS"]._serialized_start = 314
    _globals["_BACKTEST_STATUS"]._serialized_end = 508
    _globals["_BACKTEST_STATUS_STATUS"]._serialized_start = 455
    _globals["_BACKTEST_STATUS_STATUS"]._serialized_end = 498
# @@protoc_insertion_point(module_scope)