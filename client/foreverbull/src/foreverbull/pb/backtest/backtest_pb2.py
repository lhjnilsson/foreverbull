# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/pb/backtest/backtest.proto
# Protobuf Python Version: 5.27.2
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder

_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC, 5, 27, 2, "", "foreverbull/pb/backtest/backtest.proto"
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from foreverbull.pb.finance import finance_pb2 as foreverbull_dot_pb_dot_finance_dot_finance__pb2
from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2

DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(
    b'\n&foreverbull/pb/backtest/backtest.proto\x12\x17\x66oreverbull.pb.backtest\x1a\x1fgoogle/protobuf/timestamp.proto\x1a$foreverbull/pb/finance/finance.proto"z\n\tIngestion\x12.\n\nstart_date\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12,\n\x08\x65nd_date\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0f\n\x07symbols\x18\x03 \x03(\t"\x9f\x01\n\x08\x42\x61\x63ktest\x12.\n\nstart_date\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12,\n\x08\x65nd_date\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0f\n\x07symbols\x18\x03 \x03(\t\x12\x16\n\tbenchmark\x18\x04 \x01(\tH\x00\x88\x01\x01\x42\x0c\n\n_benchmark"\x8b\x01\n\x08Position\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06\x61mount\x18\x02 \x01(\x05\x12\x12\n\ncost_basis\x18\x03 \x01(\x01\x12\x17\n\x0flast_sale_price\x18\x04 \x01(\x01\x12\x32\n\x0elast_sale_date\x18\x05 \x01(\x0b\x32\x1a.google.protobuf.Timestamp"\x94\x02\n\tPortfolio\x12-\n\ttimestamp\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x11\n\tcash_flow\x18\x02 \x01(\x01\x12\x15\n\rstarting_cash\x18\x03 \x01(\x01\x12\x17\n\x0fportfolio_value\x18\x04 \x01(\x01\x12\x0b\n\x03pnl\x18\x05 \x01(\x01\x12\x0f\n\x07returns\x18\x06 \x01(\x01\x12\x0c\n\x04\x63\x61sh\x18\x07 \x01(\x01\x12\x17\n\x0fpositions_value\x18\x08 \x01(\x01\x12\x1a\n\x12positions_exposure\x18\t \x01(\x01\x12\x34\n\tpositions\x18\n \x03(\x0b\x32!.foreverbull.pb.backtest.Position"\'\n\x05Order\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06\x61mount\x18\x02 \x01(\x05"\xa8\x07\n\x06Period\x12-\n\ttimestamp\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0b\n\x03PNL\x18\x02 \x01(\x01\x12\x0f\n\x07returns\x18\x03 \x01(\x01\x12\x17\n\x0fportfolio_value\x18\x04 \x01(\x01\x12\x13\n\x0blongs_count\x18\x05 \x01(\x05\x12\x14\n\x0cshorts_count\x18\x06 \x01(\x05\x12\x12\n\nlong_value\x18\x07 \x01(\x01\x12\x13\n\x0bshort_value\x18\x08 \x01(\x01\x12\x19\n\x11starting_exposure\x18\t \x01(\x01\x12\x17\n\x0f\x65nding_exposure\x18\n \x01(\x01\x12\x15\n\rlong_exposure\x18\x0b \x01(\x01\x12\x16\n\x0eshort_exposure\x18\x0c \x01(\x01\x12\x14\n\x0c\x63\x61pital_used\x18\r \x01(\x01\x12\x16\n\x0egross_leverage\x18\x0e \x01(\x01\x12\x14\n\x0cnet_leverage\x18\x0f \x01(\x01\x12\x16\n\x0estarting_value\x18\x10 \x01(\x01\x12\x14\n\x0c\x65nding_value\x18\x11 \x01(\x01\x12\x15\n\rstarting_cash\x18\x12 \x01(\x01\x12\x13\n\x0b\x65nding_cash\x18\x13 \x01(\x01\x12\x14\n\x0cmax_drawdown\x18\x14 \x01(\x01\x12\x14\n\x0cmax_leverage\x18\x15 \x01(\x01\x12\x15\n\rexcess_return\x18\x16 \x01(\x01\x12\x1e\n\x16treasury_period_return\x18\x17 \x01(\x01\x12\x1f\n\x17\x61lgorithm_period_return\x18\x18 \x01(\x01\x12\x1c\n\x0f\x61lgo_volatility\x18\x19 \x01(\x01H\x00\x88\x01\x01\x12\x13\n\x06sharpe\x18\x1a \x01(\x01H\x01\x88\x01\x01\x12\x14\n\x07sortino\x18\x1b \x01(\x01H\x02\x88\x01\x01\x12$\n\x17\x62\x65nchmark_period_return\x18\x1c \x01(\x01H\x03\x88\x01\x01\x12!\n\x14\x62\x65nchmark_volatility\x18\x1d \x01(\x01H\x04\x88\x01\x01\x12\x12\n\x05\x61lpha\x18\x1e \x01(\x01H\x05\x88\x01\x01\x12\x11\n\x04\x62\x65ta\x18\x1f \x01(\x01H\x06\x88\x01\x01\x12\x33\n\tpositions\x18  \x03(\x0b\x32 .foreverbull.pb.finance.PositionB\x12\n\x10_algo_volatilityB\t\n\x07_sharpeB\n\n\x08_sortinoB\x1a\n\x18_benchmark_period_returnB\x17\n\x15_benchmark_volatilityB\x08\n\x06_alphaB\x07\n\x05_beta"V\n\rIngestRequest\x12\x35\n\tingestion\x18\x01 \x01(\x0b\x32".foreverbull.pb.backtest.Ingestion\x12\x0e\n\x06upload\x18\x02 \x01(\x08"G\n\x0eIngestResponse\x12\x35\n\tingestion\x18\x01 \x01(\x0b\x32".foreverbull.pb.backtest.Ingestion"A\n\nRunRequest\x12\x33\n\x08\x62\x61\x63ktest\x18\x01 \x01(\x0b\x32!.foreverbull.pb.backtest.Backtest"B\n\x0bRunResponse\x12\x33\n\x08\x62\x61\x63ktest\x18\x01 \x01(\x0b\x32!.foreverbull.pb.backtest.Backtest"D\n\x12PlaceOrdersRequest\x12.\n\x06orders\x18\x01 \x03(\x0b\x32\x1e.foreverbull.pb.backtest.Order"\x15\n\x13PlaceOrdersResponse"\x16\n\x14GetNextPeriodRequest"b\n\x15GetNextPeriodResponse\x12\x12\n\nis_running\x18\x01 \x01(\x08\x12\x35\n\tportfolio\x18\x02 \x01(\x0b\x32".foreverbull.pb.backtest.Portfolio"5\n\x10GetResultRequest\x12\x11\n\texecution\x18\x01 \x01(\t\x12\x0e\n\x06upload\x18\x02 \x01(\x08"E\n\x11GetResultResponse\x12\x30\n\x07periods\x18\x01 \x03(\x0b\x32\x1f.foreverbull.pb.backtest.Period"\r\n\x0bStopRequest"\x0e\n\x0cStopResponseB8Z6github.com/lhjnilsson/foreverbull/internal/pb/backtestb\x06proto3'
)

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, "foreverbull.pb.backtest.backtest_pb2", _globals)
if not _descriptor._USE_C_DESCRIPTORS:
    _globals["DESCRIPTOR"]._loaded_options = None
    _globals["DESCRIPTOR"]._serialized_options = b"Z6github.com/lhjnilsson/foreverbull/internal/pb/backtest"
    _globals["_INGESTION"]._serialized_start = 138
    _globals["_INGESTION"]._serialized_end = 260
    _globals["_BACKTEST"]._serialized_start = 263
    _globals["_BACKTEST"]._serialized_end = 422
    _globals["_POSITION"]._serialized_start = 425
    _globals["_POSITION"]._serialized_end = 564
    _globals["_PORTFOLIO"]._serialized_start = 567
    _globals["_PORTFOLIO"]._serialized_end = 843
    _globals["_ORDER"]._serialized_start = 845
    _globals["_ORDER"]._serialized_end = 884
    _globals["_PERIOD"]._serialized_start = 887
    _globals["_PERIOD"]._serialized_end = 1823
    _globals["_INGESTREQUEST"]._serialized_start = 1825
    _globals["_INGESTREQUEST"]._serialized_end = 1911
    _globals["_INGESTRESPONSE"]._serialized_start = 1913
    _globals["_INGESTRESPONSE"]._serialized_end = 1984
    _globals["_RUNREQUEST"]._serialized_start = 1986
    _globals["_RUNREQUEST"]._serialized_end = 2051
    _globals["_RUNRESPONSE"]._serialized_start = 2053
    _globals["_RUNRESPONSE"]._serialized_end = 2119
    _globals["_PLACEORDERSREQUEST"]._serialized_start = 2121
    _globals["_PLACEORDERSREQUEST"]._serialized_end = 2189
    _globals["_PLACEORDERSRESPONSE"]._serialized_start = 2191
    _globals["_PLACEORDERSRESPONSE"]._serialized_end = 2212
    _globals["_GETNEXTPERIODREQUEST"]._serialized_start = 2214
    _globals["_GETNEXTPERIODREQUEST"]._serialized_end = 2236
    _globals["_GETNEXTPERIODRESPONSE"]._serialized_start = 2238
    _globals["_GETNEXTPERIODRESPONSE"]._serialized_end = 2336
    _globals["_GETRESULTREQUEST"]._serialized_start = 2338
    _globals["_GETRESULTREQUEST"]._serialized_end = 2391
    _globals["_GETRESULTRESPONSE"]._serialized_start = 2393
    _globals["_GETRESULTRESPONSE"]._serialized_end = 2462
    _globals["_STOPREQUEST"]._serialized_start = 2464
    _globals["_STOPREQUEST"]._serialized_end = 2477
    _globals["_STOPRESPONSE"]._serialized_start = 2479
    _globals["_STOPRESPONSE"]._serialized_end = 2493
# @@protoc_insertion_point(module_scope)
