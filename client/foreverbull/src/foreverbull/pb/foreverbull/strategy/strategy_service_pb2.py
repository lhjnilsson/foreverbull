# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/strategy/strategy_service.proto
# Protobuf Python Version: 5.27.2
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC,
    5,
    27,
    2,
    '',
    'foreverbull/strategy/strategy_service.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from foreverbull.pb.foreverbull import common_pb2 as foreverbull_dot_common__pb2
from foreverbull.pb.foreverbull.service import worker_pb2 as foreverbull_dot_service_dot_worker__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n+foreverbull/strategy/strategy_service.proto\x12\x14\x66oreverbull.strategy\x1a\x18\x66oreverbull/common.proto\x1a foreverbull/service/worker.proto\"\x86\x01\n\x12RunStrategyRequest\x12\x0f\n\x07symbols\x18\x01 \x03(\t\x12,\n\nstart_date\x18\x02 \x01(\x0b\x32\x18.foreverbull.common.Date\x12\x31\n\talgorithm\x18\x03 \x01(\x0b\x32\x1e.foreverbull.service.Algorithm\"\xd7\x02\n\x13RunStrategyResponse\x12@\n\x06status\x18\x01 \x01(\x0b\x32\x30.foreverbull.strategy.RunStrategyResponse.Status\x12\x42\n\rconfiguration\x18\x02 \x01(\x0b\x32+.foreverbull.service.ExecutionConfiguration\x1a\xb9\x01\n\x06Status\x12G\n\x06status\x18\x01 \x01(\x0e\x32\x37.foreverbull.strategy.RunStrategyResponse.Status.Status\x12\x12\n\x05\x65rror\x18\x02 \x01(\tH\x00\x88\x01\x01\"H\n\x06Status\x12\x0b\n\x07\x43REATED\x10\x00\x12\t\n\x05READY\x10\x01\x12\x0b\n\x07RUNNING\x10\x02\x12\r\n\tCOMPLETED\x10\x03\x12\n\n\x06\x46\x41ILED\x10\x04\x42\x08\n\x06_error2x\n\x10StrategyServicer\x12\x64\n\x0bRunStrategy\x12(.foreverbull.strategy.RunStrategyRequest\x1a).foreverbull.strategy.RunStrategyResponse0\x01\x42\x33Z1github.com/lhjnilsson/foreverbull/pkg/strategy/pbb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.strategy.strategy_service_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z1github.com/lhjnilsson/foreverbull/pkg/strategy/pb'
  _globals['_RUNSTRATEGYREQUEST']._serialized_start=130
  _globals['_RUNSTRATEGYREQUEST']._serialized_end=264
  _globals['_RUNSTRATEGYRESPONSE']._serialized_start=267
  _globals['_RUNSTRATEGYRESPONSE']._serialized_end=610
  _globals['_RUNSTRATEGYRESPONSE_STATUS']._serialized_start=425
  _globals['_RUNSTRATEGYRESPONSE_STATUS']._serialized_end=610
  _globals['_RUNSTRATEGYRESPONSE_STATUS_STATUS']._serialized_start=528
  _globals['_RUNSTRATEGYRESPONSE_STATUS_STATUS']._serialized_end=600
  _globals['_STRATEGYSERVICER']._serialized_start=612
  _globals['_STRATEGYSERVICER']._serialized_end=732
# @@protoc_insertion_point(module_scope)
