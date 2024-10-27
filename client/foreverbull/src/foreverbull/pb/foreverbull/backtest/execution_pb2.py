# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/backtest/execution.proto
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
    'foreverbull/backtest/execution.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as foreverbull_dot_finance_dot_finance__pb2
from foreverbull.pb.foreverbull import common_pb2 as foreverbull_dot_common__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n$foreverbull/backtest/execution.proto\x12\x14\x66oreverbull.backtest\x1a\x1fgoogle/protobuf/timestamp.proto\x1a!foreverbull/finance/finance.proto\x1a\x18\x66oreverbull/common.proto\"\x89\x04\n\tExecution\x12\n\n\x02id\x18\x01 \x01(\t\x12\x0f\n\x07session\x18\x02 \x01(\t\x12,\n\nstart_date\x18\x03 \x01(\x0b\x32\x18.foreverbull.common.Date\x12*\n\x08\x65nd_date\x18\x04 \x01(\x0b\x32\x18.foreverbull.common.Date\x12\x16\n\tbenchmark\x18\x05 \x01(\tH\x00\x88\x01\x01\x12\x0f\n\x07symbols\x18\x06 \x03(\t\x12\x38\n\x08statuses\x18\x07 \x03(\x0b\x32&.foreverbull.backtest.Execution.Status\x12\x31\n\x06result\x18\x08 \x01(\x0b\x32\x1c.foreverbull.backtest.PeriodH\x01\x88\x01\x01\x1a\xd5\x01\n\x06Status\x12=\n\x06status\x18\x01 \x01(\x0e\x32-.foreverbull.backtest.Execution.Status.Status\x12\x12\n\x05\x65rror\x18\x02 \x01(\tH\x00\x88\x01\x01\x12/\n\x0boccurred_at\x18\x03 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\"=\n\x06Status\x12\x0b\n\x07\x43REATED\x10\x00\x12\x0b\n\x07RUNNING\x10\x01\x12\r\n\tCOMPLETED\x10\x02\x12\n\n\x06\x46\x41ILED\x10\x03\x42\x08\n\x06_errorB\x0c\n\n_benchmarkB\t\n\x07_result\"\x9e\x07\n\x06Period\x12&\n\x04\x64\x61te\x18\x01 \x01(\x0b\x32\x18.foreverbull.common.Date\x12\x0b\n\x03PNL\x18\x02 \x01(\x01\x12\x0f\n\x07returns\x18\x03 \x01(\x01\x12\x17\n\x0fportfolio_value\x18\x04 \x01(\x01\x12\x13\n\x0blongs_count\x18\x05 \x01(\x05\x12\x14\n\x0cshorts_count\x18\x06 \x01(\x05\x12\x12\n\nlong_value\x18\x07 \x01(\x01\x12\x13\n\x0bshort_value\x18\x08 \x01(\x01\x12\x19\n\x11starting_exposure\x18\t \x01(\x01\x12\x17\n\x0f\x65nding_exposure\x18\n \x01(\x01\x12\x15\n\rlong_exposure\x18\x0b \x01(\x01\x12\x16\n\x0eshort_exposure\x18\x0c \x01(\x01\x12\x14\n\x0c\x63\x61pital_used\x18\r \x01(\x01\x12\x16\n\x0egross_leverage\x18\x0e \x01(\x01\x12\x14\n\x0cnet_leverage\x18\x0f \x01(\x01\x12\x16\n\x0estarting_value\x18\x10 \x01(\x01\x12\x14\n\x0c\x65nding_value\x18\x11 \x01(\x01\x12\x15\n\rstarting_cash\x18\x12 \x01(\x01\x12\x13\n\x0b\x65nding_cash\x18\x13 \x01(\x01\x12\x14\n\x0cmax_drawdown\x18\x14 \x01(\x01\x12\x14\n\x0cmax_leverage\x18\x15 \x01(\x01\x12\x15\n\rexcess_return\x18\x16 \x01(\x01\x12\x1e\n\x16treasury_period_return\x18\x17 \x01(\x01\x12\x1f\n\x17\x61lgorithm_period_return\x18\x18 \x01(\x01\x12\x1c\n\x0f\x61lgo_volatility\x18\x19 \x01(\x01H\x00\x88\x01\x01\x12\x13\n\x06sharpe\x18\x1a \x01(\x01H\x01\x88\x01\x01\x12\x14\n\x07sortino\x18\x1b \x01(\x01H\x02\x88\x01\x01\x12$\n\x17\x62\x65nchmark_period_return\x18\x1c \x01(\x01H\x03\x88\x01\x01\x12!\n\x14\x62\x65nchmark_volatility\x18\x1d \x01(\x01H\x04\x88\x01\x01\x12\x12\n\x05\x61lpha\x18\x1e \x01(\x01H\x05\x88\x01\x01\x12\x11\n\x04\x62\x65ta\x18\x1f \x01(\x01H\x06\x88\x01\x01\x12\x30\n\tpositions\x18  \x03(\x0b\x32\x1d.foreverbull.finance.PositionB\x12\n\x10_algo_volatilityB\t\n\x07_sharpeB\n\n\x08_sortinoB\x1a\n\x18_benchmark_period_returnB\x17\n\x15_benchmark_volatilityB\x08\n\x06_alphaB\x07\n\x05_betaB3Z1github.com/lhjnilsson/foreverbull/pkg/backtest/pbb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.backtest.execution_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z1github.com/lhjnilsson/foreverbull/pkg/backtest/pb'
  _globals['_EXECUTION']._serialized_start=157
  _globals['_EXECUTION']._serialized_end=678
  _globals['_EXECUTION_STATUS']._serialized_start=440
  _globals['_EXECUTION_STATUS']._serialized_end=653
  _globals['_EXECUTION_STATUS_STATUS']._serialized_start=582
  _globals['_EXECUTION_STATUS_STATUS']._serialized_end=643
  _globals['_PERIOD']._serialized_start=681
  _globals['_PERIOD']._serialized_end=1607
# @@protoc_insertion_point(module_scope)
