# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/service/worker.proto
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
    'foreverbull/service/worker.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n foreverbull/service/worker.proto\x12\x13\x66oreverbull.service\"\x8c\x03\n\tAlgorithm\x12\x11\n\tfile_path\x18\x01 \x01(\t\x12:\n\tfunctions\x18\x02 \x03(\x0b\x32\'.foreverbull.service.Algorithm.Function\x12\x12\n\nnamespaces\x18\x03 \x03(\t\x1a}\n\x11\x46unctionParameter\x12\x0b\n\x03key\x18\x01 \x01(\t\x12\x19\n\x0c\x64\x65\x66\x61ultValue\x18\x02 \x01(\tH\x00\x88\x01\x01\x12\x12\n\x05value\x18\x03 \x01(\tH\x01\x88\x01\x01\x12\x11\n\tvalueType\x18\x04 \x01(\tB\x0f\n\r_defaultValueB\x08\n\x06_value\x1a\x9c\x01\n\x08\x46unction\x12\x0c\n\x04name\x18\x01 \x01(\t\x12\x44\n\nparameters\x18\x02 \x03(\x0b\x32\x30.foreverbull.service.Algorithm.FunctionParameter\x12\x19\n\x11parallelExecution\x18\x03 \x01(\x08\x12\x10\n\x08runFirst\x18\x04 \x01(\x08\x12\x0f\n\x07runLast\x18\x05 \x01(\x08\"\xbf\x02\n\x16\x45xecutionConfiguration\x12\x12\n\nbrokerPort\x18\x01 \x01(\x05\x12\x15\n\rnamespacePort\x18\x02 \x01(\x05\x12\x13\n\x0b\x64\x61tabaseURL\x18\x03 \x01(\t\x12G\n\tfunctions\x18\x04 \x03(\x0b\x32\x34.foreverbull.service.ExecutionConfiguration.Function\x1a/\n\x11\x46unctionParameter\x12\x0b\n\x03key\x18\x01 \x01(\t\x12\r\n\x05value\x18\x02 \x01(\t\x1ak\n\x08\x46unction\x12\x0c\n\x04name\x18\x01 \x01(\t\x12Q\n\nparameters\x18\x02 \x03(\x0b\x32=.foreverbull.service.ExecutionConfiguration.FunctionParameterB2Z0github.com/lhjnilsson/foreverbull/pkg/pb/serviceb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.service.worker_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z0github.com/lhjnilsson/foreverbull/pkg/pb/service'
  _globals['_ALGORITHM']._serialized_start=58
  _globals['_ALGORITHM']._serialized_end=454
  _globals['_ALGORITHM_FUNCTIONPARAMETER']._serialized_start=170
  _globals['_ALGORITHM_FUNCTIONPARAMETER']._serialized_end=295
  _globals['_ALGORITHM_FUNCTION']._serialized_start=298
  _globals['_ALGORITHM_FUNCTION']._serialized_end=454
  _globals['_EXECUTIONCONFIGURATION']._serialized_start=457
  _globals['_EXECUTIONCONFIGURATION']._serialized_end=776
  _globals['_EXECUTIONCONFIGURATION_FUNCTIONPARAMETER']._serialized_start=620
  _globals['_EXECUTIONCONFIGURATION_FUNCTIONPARAMETER']._serialized_end=667
  _globals['_EXECUTIONCONFIGURATION_FUNCTION']._serialized_start=669
  _globals['_EXECUTIONCONFIGURATION_FUNCTION']._serialized_end=776
# @@protoc_insertion_point(module_scope)
