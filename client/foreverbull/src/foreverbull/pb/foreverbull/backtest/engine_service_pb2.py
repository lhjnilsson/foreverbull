# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: foreverbull/backtest/engine_service.proto
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
    'foreverbull/backtest/engine_service.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from foreverbull.pb.foreverbull.backtest import backtest_pb2 as foreverbull_dot_backtest_dot_backtest__pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as foreverbull_dot_finance_dot_finance__pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2 as foreverbull_dot_backtest_dot_execution__pb2
from foreverbull.pb.foreverbull.backtest import ingestion_pb2 as foreverbull_dot_backtest_dot_ingestion__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n)foreverbull/backtest/engine_service.proto\x12\x14\x66oreverbull.backtest\x1a#foreverbull/backtest/backtest.proto\x1a!foreverbull/finance/finance.proto\x1a$foreverbull/backtest/execution.proto\x1a$foreverbull/backtest/ingestion.proto\"\x15\n\x13GetIngestionRequest\"J\n\x14GetIngestionResponse\x12\x32\n\tingestion\x18\x01 \x01(\x0b\x32\x1f.foreverbull.backtest.Ingestion\":\n\x18\x44ownloadIngestionRequest\x12\x0e\n\x06\x62ucket\x18\x01 \x01(\t\x12\x0e\n\x06object\x18\x02 \x01(\t\"O\n\x19\x44ownloadIngestionResponse\x12\x32\n\tingestion\x18\x01 \x01(\x0b\x32\x1f.foreverbull.backtest.Ingestion\"\x83\x01\n\rIngestRequest\x12\x32\n\tingestion\x18\x01 \x01(\x0b\x32\x1f.foreverbull.backtest.Ingestion\x12\x13\n\x06\x62ucket\x18\x02 \x01(\tH\x00\x88\x01\x01\x12\x13\n\x06object\x18\x03 \x01(\tH\x01\x88\x01\x01\x42\t\n\x07_bucketB\t\n\x07_object\"\x10\n\x0eIngestResponse\"\x1f\n\x11NewSessionRequest\x12\n\n\x02id\x18\x01 \x01(\t\"\"\n\x12NewSessionResponse\x12\x0c\n\x04port\x18\x01 \x01(\x03\"F\n\x12RunBacktestRequest\x12\x30\n\x08\x62\x61\x63ktest\x18\x01 \x01(\x0b\x32\x1e.foreverbull.backtest.Backtest\"G\n\x13RunBacktestResponse\x12\x30\n\x08\x62\x61\x63ktest\x18\x01 \x01(\x0b\x32\x1e.foreverbull.backtest.Backtest\"\x19\n\x17GetCurrentPeriodRequest\"t\n\x18GetCurrentPeriodResponse\x12\x12\n\nis_running\x18\x01 \x01(\x08\x12\x36\n\tportfolio\x18\x02 \x01(\x0b\x32\x1e.foreverbull.finance.PortfolioH\x00\x88\x01\x01\x42\x0c\n\n_portfolio\"K\n\x1dPlaceOrdersAndContinueRequest\x12*\n\x06orders\x18\x01 \x03(\x0b\x32\x1a.foreverbull.finance.Order\" \n\x1ePlaceOrdersAndContinueResponse\"5\n\x10GetResultRequest\x12\x11\n\texecution\x18\x01 \x01(\t\x12\x0e\n\x06upload\x18\x02 \x01(\x08\"B\n\x11GetResultResponse\x12-\n\x07periods\x18\x01 \x03(\x0b\x32\x1c.foreverbull.backtest.Period2\xa3\x03\n\x06\x45ngine\x12g\n\x0cGetIngestion\x12).foreverbull.backtest.GetIngestionRequest\x1a*.foreverbull.backtest.GetIngestionResponse\"\x00\x12v\n\x11\x44ownloadIngestion\x12..foreverbull.backtest.DownloadIngestionRequest\x1a/.foreverbull.backtest.DownloadIngestionResponse\"\x00\x12U\n\x06Ingest\x12#.foreverbull.backtest.IngestRequest\x1a$.foreverbull.backtest.IngestResponse\"\x00\x12\x61\n\nNewSession\x12\'.foreverbull.backtest.NewSessionRequest\x1a(.foreverbull.backtest.NewSessionResponse\"\x00\x32\xd2\x03\n\rEngineSession\x12\x64\n\x0bRunBacktest\x12(.foreverbull.backtest.RunBacktestRequest\x1a).foreverbull.backtest.RunBacktestResponse\"\x00\x12s\n\x10GetCurrentPeriod\x12-.foreverbull.backtest.GetCurrentPeriodRequest\x1a..foreverbull.backtest.GetCurrentPeriodResponse\"\x00\x12\x85\x01\n\x16PlaceOrdersAndContinue\x12\x33.foreverbull.backtest.PlaceOrdersAndContinueRequest\x1a\x34.foreverbull.backtest.PlaceOrdersAndContinueResponse\"\x00\x12^\n\tGetResult\x12&.foreverbull.backtest.GetResultRequest\x1a\'.foreverbull.backtest.GetResultResponse\"\x00\x42\x33Z1github.com/lhjnilsson/foreverbull/pkg/pb/backtestb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'foreverbull.backtest.engine_service_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z1github.com/lhjnilsson/foreverbull/pkg/pb/backtest'
  _globals['_GETINGESTIONREQUEST']._serialized_start=215
  _globals['_GETINGESTIONREQUEST']._serialized_end=236
  _globals['_GETINGESTIONRESPONSE']._serialized_start=238
  _globals['_GETINGESTIONRESPONSE']._serialized_end=312
  _globals['_DOWNLOADINGESTIONREQUEST']._serialized_start=314
  _globals['_DOWNLOADINGESTIONREQUEST']._serialized_end=372
  _globals['_DOWNLOADINGESTIONRESPONSE']._serialized_start=374
  _globals['_DOWNLOADINGESTIONRESPONSE']._serialized_end=453
  _globals['_INGESTREQUEST']._serialized_start=456
  _globals['_INGESTREQUEST']._serialized_end=587
  _globals['_INGESTRESPONSE']._serialized_start=589
  _globals['_INGESTRESPONSE']._serialized_end=605
  _globals['_NEWSESSIONREQUEST']._serialized_start=607
  _globals['_NEWSESSIONREQUEST']._serialized_end=638
  _globals['_NEWSESSIONRESPONSE']._serialized_start=640
  _globals['_NEWSESSIONRESPONSE']._serialized_end=674
  _globals['_RUNBACKTESTREQUEST']._serialized_start=676
  _globals['_RUNBACKTESTREQUEST']._serialized_end=746
  _globals['_RUNBACKTESTRESPONSE']._serialized_start=748
  _globals['_RUNBACKTESTRESPONSE']._serialized_end=819
  _globals['_GETCURRENTPERIODREQUEST']._serialized_start=821
  _globals['_GETCURRENTPERIODREQUEST']._serialized_end=846
  _globals['_GETCURRENTPERIODRESPONSE']._serialized_start=848
  _globals['_GETCURRENTPERIODRESPONSE']._serialized_end=964
  _globals['_PLACEORDERSANDCONTINUEREQUEST']._serialized_start=966
  _globals['_PLACEORDERSANDCONTINUEREQUEST']._serialized_end=1041
  _globals['_PLACEORDERSANDCONTINUERESPONSE']._serialized_start=1043
  _globals['_PLACEORDERSANDCONTINUERESPONSE']._serialized_end=1075
  _globals['_GETRESULTREQUEST']._serialized_start=1077
  _globals['_GETRESULTREQUEST']._serialized_end=1130
  _globals['_GETRESULTRESPONSE']._serialized_start=1132
  _globals['_GETRESULTRESPONSE']._serialized_end=1198
  _globals['_ENGINE']._serialized_start=1201
  _globals['_ENGINE']._serialized_end=1620
  _globals['_ENGINESESSION']._serialized_start=1623
  _globals['_ENGINESESSION']._serialized_end=2089
# @@protoc_insertion_point(module_scope)
