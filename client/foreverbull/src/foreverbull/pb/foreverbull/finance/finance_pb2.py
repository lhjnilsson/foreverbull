# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/finance/finance.proto
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
    'foreverbull/finance/finance.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n!foreverbull/finance/finance.proto\x12\x13\x66oreverbull.finance\x1a\x1fgoogle/protobuf/timestamp.proto\"%\n\x05\x41sset\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0c\n\x04name\x18\x02 \x01(\t\"\x8d\x01\n\x04OHLC\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12-\n\ttimestamp\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0c\n\x04open\x18\x03 \x01(\x01\x12\x0c\n\x04high\x18\x04 \x01(\x01\x12\x0b\n\x03low\x18\x05 \x01(\x01\x12\r\n\x05\x63lose\x18\x06 \x01(\x01\x12\x0e\n\x06volume\x18\x07 \x01(\x05\"\x8b\x01\n\x08Position\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06\x61mount\x18\x02 \x01(\x05\x12\x12\n\ncost_basis\x18\x03 \x01(\x01\x12\x17\n\x0flast_sale_price\x18\x04 \x01(\x01\x12\x32\n\x0elast_sale_date\x18\x05 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\"\x90\x02\n\tPortfolio\x12-\n\ttimestamp\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x11\n\tcash_flow\x18\x02 \x01(\x01\x12\x15\n\rstarting_cash\x18\x03 \x01(\x01\x12\x17\n\x0fportfolio_value\x18\x04 \x01(\x01\x12\x0b\n\x03pnl\x18\x05 \x01(\x01\x12\x0f\n\x07returns\x18\x06 \x01(\x01\x12\x0c\n\x04\x63\x61sh\x18\x07 \x01(\x01\x12\x17\n\x0fpositions_value\x18\x08 \x01(\x01\x12\x1a\n\x12positions_exposure\x18\t \x01(\x01\x12\x30\n\tpositions\x18\n \x03(\x0b\x32\x1d.foreverbull.finance.Position\"\'\n\x05Order\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06\x61mount\x18\x02 \x01(\x05\x42\x32Z0github.com/lhjnilsson/foreverbull/pkg/finance/pbb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.finance.finance_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z0github.com/lhjnilsson/foreverbull/pkg/finance/pb'
  _globals['_ASSET']._serialized_start=91
  _globals['_ASSET']._serialized_end=128
  _globals['_OHLC']._serialized_start=131
  _globals['_OHLC']._serialized_end=272
  _globals['_POSITION']._serialized_start=275
  _globals['_POSITION']._serialized_end=414
  _globals['_PORTFOLIO']._serialized_start=417
  _globals['_PORTFOLIO']._serialized_end=689
  _globals['_ORDER']._serialized_start=691
  _globals['_ORDER']._serialized_end=730
# @@protoc_insertion_point(module_scope)
