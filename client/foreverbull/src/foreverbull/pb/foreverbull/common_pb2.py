# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/common.proto
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
    'foreverbull/common.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x18\x66oreverbull/common.proto\x12\x12\x66oreverbull.common\"3\n\x07Request\x12\x0c\n\x04task\x18\x01 \x01(\t\x12\x11\n\x04\x64\x61ta\x18\x02 \x01(\x0cH\x00\x88\x01\x01\x42\x07\n\x05_data\"R\n\x08Response\x12\x0c\n\x04task\x18\x01 \x01(\t\x12\x11\n\x04\x64\x61ta\x18\x02 \x01(\x0cH\x00\x88\x01\x01\x12\x12\n\x05\x65rror\x18\x03 \x01(\tH\x01\x88\x01\x01\x42\x07\n\x05_dataB\x08\n\x06_error\"0\n\x04\x44\x61te\x12\x0c\n\x04year\x18\x01 \x01(\x05\x12\r\n\x05month\x18\x02 \x01(\x05\x12\x0b\n\x03\x64\x61y\x18\x03 \x01(\x05\x42*Z(github.com/lhjnilsson/foreverbull/pkg/pbb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.common_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z(github.com/lhjnilsson/foreverbull/pkg/pb'
  _globals['_REQUEST']._serialized_start=48
  _globals['_REQUEST']._serialized_end=99
  _globals['_RESPONSE']._serialized_start=101
  _globals['_RESPONSE']._serialized_end=183
  _globals['_DATE']._serialized_start=185
  _globals['_DATE']._serialized_end=233
# @@protoc_insertion_point(module_scope)
