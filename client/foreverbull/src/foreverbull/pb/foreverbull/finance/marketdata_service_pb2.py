# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/finance/marketdata_service.proto
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
    'foreverbull/finance/marketdata_service.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from foreverbull.pb.foreverbull import common_pb2 as foreverbull_dot_common__pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as foreverbull_dot_finance_dot_finance__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n,foreverbull/finance/marketdata_service.proto\x12\x13\x66oreverbull.finance\x1a\x18\x66oreverbull/common.proto\x1a!foreverbull/finance/finance.proto\"!\n\x0fGetAssetRequest\x12\x0e\n\x06symbol\x18\x01 \x01(\t\"=\n\x10GetAssetResponse\x12)\n\x05\x61sset\x18\x01 \x01(\x0b\x32\x1a.foreverbull.finance.Asset\"!\n\x0fGetIndexRequest\x12\x0e\n\x06symbol\x18\x01 \x01(\t\">\n\x10GetIndexResponse\x12*\n\x06\x61ssets\x18\x01 \x03(\x0b\x32\x1a.foreverbull.finance.Asset\"\x8a\x01\n\x1d\x44ownloadHistoricalDataRequest\x12\x0f\n\x07symbols\x18\x01 \x03(\t\x12,\n\nstart_date\x18\x02 \x01(\x0b\x32\x18.foreverbull.common.Date\x12*\n\x08\x65nd_date\x18\x03 \x01(\x0b\x32\x18.foreverbull.common.Date\" \n\x1e\x44ownloadHistoricalDataResponse2\xc8\x02\n\nMarketdata\x12Y\n\x08GetAsset\x12$.foreverbull.finance.GetAssetRequest\x1a%.foreverbull.finance.GetAssetResponse\"\x00\x12Y\n\x08GetIndex\x12$.foreverbull.finance.GetIndexRequest\x1a%.foreverbull.finance.GetIndexResponse\"\x00\x12\x83\x01\n\x16\x44ownloadHistoricalData\x12\x32.foreverbull.finance.DownloadHistoricalDataRequest\x1a\x33.foreverbull.finance.DownloadHistoricalDataResponse\"\x00\x42\x32Z0github.com/lhjnilsson/foreverbull/pkg/finance/pbb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.finance.marketdata_service_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z0github.com/lhjnilsson/foreverbull/pkg/finance/pb'
  _globals['_GETASSETREQUEST']._serialized_start=130
  _globals['_GETASSETREQUEST']._serialized_end=163
  _globals['_GETASSETRESPONSE']._serialized_start=165
  _globals['_GETASSETRESPONSE']._serialized_end=226
  _globals['_GETINDEXREQUEST']._serialized_start=228
  _globals['_GETINDEXREQUEST']._serialized_end=261
  _globals['_GETINDEXRESPONSE']._serialized_start=263
  _globals['_GETINDEXRESPONSE']._serialized_end=325
  _globals['_DOWNLOADHISTORICALDATAREQUEST']._serialized_start=328
  _globals['_DOWNLOADHISTORICALDATAREQUEST']._serialized_end=466
  _globals['_DOWNLOADHISTORICALDATARESPONSE']._serialized_start=468
  _globals['_DOWNLOADHISTORICALDATARESPONSE']._serialized_end=500
  _globals['_MARKETDATA']._serialized_start=503
  _globals['_MARKETDATA']._serialized_end=831
# @@protoc_insertion_point(module_scope)
