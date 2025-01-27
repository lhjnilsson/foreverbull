# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/finance/trading_service.proto
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
    'foreverbull/finance/trading_service.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from foreverbull.pb.foreverbull.finance import finance_pb2 as foreverbull_dot_finance_dot_finance__pb2
from foreverbull.pb.buf.validate import validate_pb2 as buf_dot_validate_dot_validate__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n)foreverbull/finance/trading_service.proto\x12\x13\x66oreverbull.finance\x1a!foreverbull/finance/finance.proto\x1a\x1b\x62uf/validate/validate.proto\"\x15\n\x13GetPortfolioRequest\"I\n\x14GetPortfolioResponse\x12\x31\n\tportfolio\x18\x01 \x01(\x0b\x32\x1e.foreverbull.finance.Portfolio\"\x12\n\x10GetOrdersRequest\"?\n\x11GetOrdersResponse\x12*\n\x06orders\x18\x01 \x03(\x0b\x32\x1a.foreverbull.finance.Order\"g\n\x11PlaceOrderRequest\x12R\n\x05order\x18\x01 \x01(\x0b\x32\x1a.foreverbull.finance.OrderB\'\xbaH$\xba\x01\x1e\n\x0eorder_required\x1a\x0cthis != null\xc8\x01\x01\"\x14\n\x12PlaceOrderResponse2\xaf\x02\n\x07Trading\x12\x65\n\x0cGetPortfolio\x12(.foreverbull.finance.GetPortfolioRequest\x1a).foreverbull.finance.GetPortfolioResponse\"\x00\x12\\\n\tGetOrders\x12%.foreverbull.finance.GetOrdersRequest\x1a&.foreverbull.finance.GetOrdersResponse\"\x00\x12_\n\nPlaceOrder\x12&.foreverbull.finance.PlaceOrderRequest\x1a\'.foreverbull.finance.PlaceOrderResponse\"\x00\x42\x32Z0github.com/lhjnilsson/foreverbull/pkg/pb/financeb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.finance.trading_service_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z0github.com/lhjnilsson/foreverbull/pkg/pb/finance'
  _globals['_PLACEORDERREQUEST'].fields_by_name['order']._loaded_options = None
  _globals['_PLACEORDERREQUEST'].fields_by_name['order']._serialized_options = b'\272H$\272\001\036\n\016order_required\032\014this != null\310\001\001'
  _globals['_GETPORTFOLIOREQUEST']._serialized_start=130
  _globals['_GETPORTFOLIOREQUEST']._serialized_end=151
  _globals['_GETPORTFOLIORESPONSE']._serialized_start=153
  _globals['_GETPORTFOLIORESPONSE']._serialized_end=226
  _globals['_GETORDERSREQUEST']._serialized_start=228
  _globals['_GETORDERSREQUEST']._serialized_end=246
  _globals['_GETORDERSRESPONSE']._serialized_start=248
  _globals['_GETORDERSRESPONSE']._serialized_end=311
  _globals['_PLACEORDERREQUEST']._serialized_start=313
  _globals['_PLACEORDERREQUEST']._serialized_end=416
  _globals['_PLACEORDERRESPONSE']._serialized_start=418
  _globals['_PLACEORDERRESPONSE']._serialized_end=438
  _globals['_TRADING']._serialized_start=441
  _globals['_TRADING']._serialized_end=744
# @@protoc_insertion_point(module_scope)
