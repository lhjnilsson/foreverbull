// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/backtest/execution.proto

package pb

import (
	pb1 "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Execution_Status_Status int32

const (
	Execution_Status_CREATED   Execution_Status_Status = 0
	Execution_Status_RUNNING   Execution_Status_Status = 1
	Execution_Status_COMPLETED Execution_Status_Status = 2
	Execution_Status_FAILED    Execution_Status_Status = 3
)

// Enum value maps for Execution_Status_Status.
var (
	Execution_Status_Status_name = map[int32]string{
		0: "CREATED",
		1: "RUNNING",
		2: "COMPLETED",
		3: "FAILED",
	}
	Execution_Status_Status_value = map[string]int32{
		"CREATED":   0,
		"RUNNING":   1,
		"COMPLETED": 2,
		"FAILED":    3,
	}
)

func (x Execution_Status_Status) Enum() *Execution_Status_Status {
	p := new(Execution_Status_Status)
	*p = x
	return p
}

func (x Execution_Status_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Execution_Status_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_foreverbull_backtest_execution_proto_enumTypes[0].Descriptor()
}

func (Execution_Status_Status) Type() protoreflect.EnumType {
	return &file_foreverbull_backtest_execution_proto_enumTypes[0]
}

func (x Execution_Status_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Execution_Status_Status.Descriptor instead.
func (Execution_Status_Status) EnumDescriptor() ([]byte, []int) {
	return file_foreverbull_backtest_execution_proto_rawDescGZIP(), []int{0, 0, 0}
}

type Execution struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id        string              `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Backtest  string              `protobuf:"bytes,2,opt,name=backtest,proto3" json:"backtest,omitempty"`
	Session   string              `protobuf:"bytes,3,opt,name=session,proto3" json:"session,omitempty"`
	StartDate *pb.Date            `protobuf:"bytes,4,opt,name=start_date,json=startDate,proto3" json:"start_date,omitempty"`
	EndDate   *pb.Date            `protobuf:"bytes,5,opt,name=end_date,json=endDate,proto3" json:"end_date,omitempty"`
	Benchmark *string             `protobuf:"bytes,6,opt,name=benchmark,proto3,oneof" json:"benchmark,omitempty"`
	Symbols   []string            `protobuf:"bytes,7,rep,name=symbols,proto3" json:"symbols,omitempty"`
	Statuses  []*Execution_Status `protobuf:"bytes,8,rep,name=statuses,proto3" json:"statuses,omitempty"`
	Result    *Period             `protobuf:"bytes,9,opt,name=result,proto3,oneof" json:"result,omitempty"`
}

func (x *Execution) Reset() {
	*x = Execution{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_execution_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Execution) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Execution) ProtoMessage() {}

func (x *Execution) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_execution_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Execution.ProtoReflect.Descriptor instead.
func (*Execution) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_execution_proto_rawDescGZIP(), []int{0}
}

func (x *Execution) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Execution) GetBacktest() string {
	if x != nil {
		return x.Backtest
	}
	return ""
}

func (x *Execution) GetSession() string {
	if x != nil {
		return x.Session
	}
	return ""
}

func (x *Execution) GetStartDate() *pb.Date {
	if x != nil {
		return x.StartDate
	}
	return nil
}

func (x *Execution) GetEndDate() *pb.Date {
	if x != nil {
		return x.EndDate
	}
	return nil
}

func (x *Execution) GetBenchmark() string {
	if x != nil && x.Benchmark != nil {
		return *x.Benchmark
	}
	return ""
}

func (x *Execution) GetSymbols() []string {
	if x != nil {
		return x.Symbols
	}
	return nil
}

func (x *Execution) GetStatuses() []*Execution_Status {
	if x != nil {
		return x.Statuses
	}
	return nil
}

func (x *Execution) GetResult() *Period {
	if x != nil {
		return x.Result
	}
	return nil
}

type Period struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Date                  *pb.Date        `protobuf:"bytes,1,opt,name=date,proto3" json:"date,omitempty"`
	PNL                   float64         `protobuf:"fixed64,2,opt,name=PNL,proto3" json:"PNL,omitempty"`
	Returns               float64         `protobuf:"fixed64,3,opt,name=returns,proto3" json:"returns,omitempty"`
	PortfolioValue        float64         `protobuf:"fixed64,4,opt,name=portfolio_value,json=portfolioValue,proto3" json:"portfolio_value,omitempty"`
	LongsCount            int32           `protobuf:"varint,5,opt,name=longs_count,json=longsCount,proto3" json:"longs_count,omitempty"`
	ShortsCount           int32           `protobuf:"varint,6,opt,name=shorts_count,json=shortsCount,proto3" json:"shorts_count,omitempty"`
	LongValue             float64         `protobuf:"fixed64,7,opt,name=long_value,json=longValue,proto3" json:"long_value,omitempty"`
	ShortValue            float64         `protobuf:"fixed64,8,opt,name=short_value,json=shortValue,proto3" json:"short_value,omitempty"`
	StartingExposure      float64         `protobuf:"fixed64,9,opt,name=starting_exposure,json=startingExposure,proto3" json:"starting_exposure,omitempty"`
	EndingExposure        float64         `protobuf:"fixed64,10,opt,name=ending_exposure,json=endingExposure,proto3" json:"ending_exposure,omitempty"`
	LongExposure          float64         `protobuf:"fixed64,11,opt,name=long_exposure,json=longExposure,proto3" json:"long_exposure,omitempty"`
	ShortExposure         float64         `protobuf:"fixed64,12,opt,name=short_exposure,json=shortExposure,proto3" json:"short_exposure,omitempty"`
	CapitalUsed           float64         `protobuf:"fixed64,13,opt,name=capital_used,json=capitalUsed,proto3" json:"capital_used,omitempty"`
	GrossLeverage         float64         `protobuf:"fixed64,14,opt,name=gross_leverage,json=grossLeverage,proto3" json:"gross_leverage,omitempty"`
	NetLeverage           float64         `protobuf:"fixed64,15,opt,name=net_leverage,json=netLeverage,proto3" json:"net_leverage,omitempty"`
	StartingValue         float64         `protobuf:"fixed64,16,opt,name=starting_value,json=startingValue,proto3" json:"starting_value,omitempty"`
	EndingValue           float64         `protobuf:"fixed64,17,opt,name=ending_value,json=endingValue,proto3" json:"ending_value,omitempty"`
	StartingCash          float64         `protobuf:"fixed64,18,opt,name=starting_cash,json=startingCash,proto3" json:"starting_cash,omitempty"`
	EndingCash            float64         `protobuf:"fixed64,19,opt,name=ending_cash,json=endingCash,proto3" json:"ending_cash,omitempty"`
	MaxDrawdown           float64         `protobuf:"fixed64,20,opt,name=max_drawdown,json=maxDrawdown,proto3" json:"max_drawdown,omitempty"`
	MaxLeverage           float64         `protobuf:"fixed64,21,opt,name=max_leverage,json=maxLeverage,proto3" json:"max_leverage,omitempty"`
	ExcessReturn          float64         `protobuf:"fixed64,22,opt,name=excess_return,json=excessReturn,proto3" json:"excess_return,omitempty"`
	TreasuryPeriodReturn  float64         `protobuf:"fixed64,23,opt,name=treasury_period_return,json=treasuryPeriodReturn,proto3" json:"treasury_period_return,omitempty"`
	AlgorithmPeriodReturn float64         `protobuf:"fixed64,24,opt,name=algorithm_period_return,json=algorithmPeriodReturn,proto3" json:"algorithm_period_return,omitempty"`
	AlgoVolatility        *float64        `protobuf:"fixed64,25,opt,name=algo_volatility,json=algoVolatility,proto3,oneof" json:"algo_volatility,omitempty"`
	Sharpe                *float64        `protobuf:"fixed64,26,opt,name=sharpe,proto3,oneof" json:"sharpe,omitempty"`
	Sortino               *float64        `protobuf:"fixed64,27,opt,name=sortino,proto3,oneof" json:"sortino,omitempty"`
	BenchmarkPeriodReturn *float64        `protobuf:"fixed64,28,opt,name=benchmark_period_return,json=benchmarkPeriodReturn,proto3,oneof" json:"benchmark_period_return,omitempty"`
	BenchmarkVolatility   *float64        `protobuf:"fixed64,29,opt,name=benchmark_volatility,json=benchmarkVolatility,proto3,oneof" json:"benchmark_volatility,omitempty"`
	Alpha                 *float64        `protobuf:"fixed64,30,opt,name=alpha,proto3,oneof" json:"alpha,omitempty"`
	Beta                  *float64        `protobuf:"fixed64,31,opt,name=beta,proto3,oneof" json:"beta,omitempty"`
	Positions             []*pb1.Position `protobuf:"bytes,32,rep,name=positions,proto3" json:"positions,omitempty"`
}

func (x *Period) Reset() {
	*x = Period{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_execution_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Period) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Period) ProtoMessage() {}

func (x *Period) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_execution_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Period.ProtoReflect.Descriptor instead.
func (*Period) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_execution_proto_rawDescGZIP(), []int{1}
}

func (x *Period) GetDate() *pb.Date {
	if x != nil {
		return x.Date
	}
	return nil
}

func (x *Period) GetPNL() float64 {
	if x != nil {
		return x.PNL
	}
	return 0
}

func (x *Period) GetReturns() float64 {
	if x != nil {
		return x.Returns
	}
	return 0
}

func (x *Period) GetPortfolioValue() float64 {
	if x != nil {
		return x.PortfolioValue
	}
	return 0
}

func (x *Period) GetLongsCount() int32 {
	if x != nil {
		return x.LongsCount
	}
	return 0
}

func (x *Period) GetShortsCount() int32 {
	if x != nil {
		return x.ShortsCount
	}
	return 0
}

func (x *Period) GetLongValue() float64 {
	if x != nil {
		return x.LongValue
	}
	return 0
}

func (x *Period) GetShortValue() float64 {
	if x != nil {
		return x.ShortValue
	}
	return 0
}

func (x *Period) GetStartingExposure() float64 {
	if x != nil {
		return x.StartingExposure
	}
	return 0
}

func (x *Period) GetEndingExposure() float64 {
	if x != nil {
		return x.EndingExposure
	}
	return 0
}

func (x *Period) GetLongExposure() float64 {
	if x != nil {
		return x.LongExposure
	}
	return 0
}

func (x *Period) GetShortExposure() float64 {
	if x != nil {
		return x.ShortExposure
	}
	return 0
}

func (x *Period) GetCapitalUsed() float64 {
	if x != nil {
		return x.CapitalUsed
	}
	return 0
}

func (x *Period) GetGrossLeverage() float64 {
	if x != nil {
		return x.GrossLeverage
	}
	return 0
}

func (x *Period) GetNetLeverage() float64 {
	if x != nil {
		return x.NetLeverage
	}
	return 0
}

func (x *Period) GetStartingValue() float64 {
	if x != nil {
		return x.StartingValue
	}
	return 0
}

func (x *Period) GetEndingValue() float64 {
	if x != nil {
		return x.EndingValue
	}
	return 0
}

func (x *Period) GetStartingCash() float64 {
	if x != nil {
		return x.StartingCash
	}
	return 0
}

func (x *Period) GetEndingCash() float64 {
	if x != nil {
		return x.EndingCash
	}
	return 0
}

func (x *Period) GetMaxDrawdown() float64 {
	if x != nil {
		return x.MaxDrawdown
	}
	return 0
}

func (x *Period) GetMaxLeverage() float64 {
	if x != nil {
		return x.MaxLeverage
	}
	return 0
}

func (x *Period) GetExcessReturn() float64 {
	if x != nil {
		return x.ExcessReturn
	}
	return 0
}

func (x *Period) GetTreasuryPeriodReturn() float64 {
	if x != nil {
		return x.TreasuryPeriodReturn
	}
	return 0
}

func (x *Period) GetAlgorithmPeriodReturn() float64 {
	if x != nil {
		return x.AlgorithmPeriodReturn
	}
	return 0
}

func (x *Period) GetAlgoVolatility() float64 {
	if x != nil && x.AlgoVolatility != nil {
		return *x.AlgoVolatility
	}
	return 0
}

func (x *Period) GetSharpe() float64 {
	if x != nil && x.Sharpe != nil {
		return *x.Sharpe
	}
	return 0
}

func (x *Period) GetSortino() float64 {
	if x != nil && x.Sortino != nil {
		return *x.Sortino
	}
	return 0
}

func (x *Period) GetBenchmarkPeriodReturn() float64 {
	if x != nil && x.BenchmarkPeriodReturn != nil {
		return *x.BenchmarkPeriodReturn
	}
	return 0
}

func (x *Period) GetBenchmarkVolatility() float64 {
	if x != nil && x.BenchmarkVolatility != nil {
		return *x.BenchmarkVolatility
	}
	return 0
}

func (x *Period) GetAlpha() float64 {
	if x != nil && x.Alpha != nil {
		return *x.Alpha
	}
	return 0
}

func (x *Period) GetBeta() float64 {
	if x != nil && x.Beta != nil {
		return *x.Beta
	}
	return 0
}

func (x *Period) GetPositions() []*pb1.Position {
	if x != nil {
		return x.Positions
	}
	return nil
}

type Execution_Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status     Execution_Status_Status `protobuf:"varint,1,opt,name=status,proto3,enum=foreverbull.backtest.Execution_Status_Status" json:"status,omitempty"`
	Error      *string                 `protobuf:"bytes,2,opt,name=error,proto3,oneof" json:"error,omitempty"`
	OccurredAt *timestamppb.Timestamp  `protobuf:"bytes,3,opt,name=occurred_at,json=occurredAt,proto3" json:"occurred_at,omitempty"`
}

func (x *Execution_Status) Reset() {
	*x = Execution_Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_execution_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Execution_Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Execution_Status) ProtoMessage() {}

func (x *Execution_Status) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_execution_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Execution_Status.ProtoReflect.Descriptor instead.
func (*Execution_Status) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_execution_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Execution_Status) GetStatus() Execution_Status_Status {
	if x != nil {
		return x.Status
	}
	return Execution_Status_CREATED
}

func (x *Execution_Status) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

func (x *Execution_Status) GetOccurredAt() *timestamppb.Timestamp {
	if x != nil {
		return x.OccurredAt
	}
	return nil
}

var File_foreverbull_backtest_execution_proto protoreflect.FileDescriptor

var file_foreverbull_backtest_execution_proto_rawDesc = []byte{
	0x0a, 0x24, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x62, 0x61,
	0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62,
	0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e,
	0x63, 0x65, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x18, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x63, 0x6f,
	0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x87, 0x05, 0x0a, 0x09, 0x45,
	0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x62, 0x61, 0x63, 0x6b,
	0x74, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x62, 0x61, 0x63, 0x6b,
	0x74, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x37,
	0x0a, 0x0a, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x18, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c,
	0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x44, 0x61, 0x74, 0x65, 0x52, 0x09, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x44, 0x61, 0x74, 0x65, 0x12, 0x33, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x5f, 0x64,
	0x61, 0x74, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x66, 0x6f, 0x72, 0x65,
	0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x44,
	0x61, 0x74, 0x65, 0x52, 0x07, 0x65, 0x6e, 0x64, 0x44, 0x61, 0x74, 0x65, 0x12, 0x21, 0x0a, 0x09,
	0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x00, 0x52, 0x09, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x88, 0x01, 0x01, 0x12,
	0x18, 0x0a, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x73, 0x12, 0x42, 0x0a, 0x08, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x65, 0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x26, 0x2e, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x52, 0x08, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x65, 0x73, 0x12, 0x39, 0x0a,
	0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e,
	0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b,
	0x74, 0x65, 0x73, 0x74, 0x2e, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x48, 0x01, 0x52, 0x06, 0x72,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x88, 0x01, 0x01, 0x1a, 0xf0, 0x01, 0x0a, 0x06, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x12, 0x45, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x2d, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c,
	0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x45, 0x78, 0x65, 0x63, 0x75,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x19, 0x0a, 0x05, 0x65, 0x72,
	0x72, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x05, 0x65, 0x72, 0x72,
	0x6f, 0x72, 0x88, 0x01, 0x01, 0x12, 0x3b, 0x0a, 0x0b, 0x6f, 0x63, 0x63, 0x75, 0x72, 0x72, 0x65,
	0x64, 0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0a, 0x6f, 0x63, 0x63, 0x75, 0x72, 0x72, 0x65, 0x64,
	0x41, 0x74, 0x22, 0x3d, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x0b, 0x0a, 0x07,
	0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x52, 0x55, 0x4e,
	0x4e, 0x49, 0x4e, 0x47, 0x10, 0x01, 0x12, 0x0d, 0x0a, 0x09, 0x43, 0x4f, 0x4d, 0x50, 0x4c, 0x45,
	0x54, 0x45, 0x44, 0x10, 0x02, 0x12, 0x0a, 0x0a, 0x06, 0x46, 0x41, 0x49, 0x4c, 0x45, 0x44, 0x10,
	0x03, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x42, 0x0c, 0x0a, 0x0a, 0x5f,
	0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x72, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x22, 0xca, 0x0a, 0x0a, 0x06, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x12,
	0x2c, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e,
	0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x63, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x2e, 0x44, 0x61, 0x74, 0x65, 0x52, 0x04, 0x64, 0x61, 0x74, 0x65, 0x12, 0x10, 0x0a,
	0x03, 0x50, 0x4e, 0x4c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x01, 0x52, 0x03, 0x50, 0x4e, 0x4c, 0x12,
	0x18, 0x0a, 0x07, 0x72, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x07, 0x72, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x73, 0x12, 0x27, 0x0a, 0x0f, 0x70, 0x6f, 0x72,
	0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x0e, 0x70, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x6c, 0x6f, 0x6e, 0x67, 0x73, 0x5f, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x6c, 0x6f, 0x6e, 0x67, 0x73, 0x43, 0x6f,
	0x75, 0x6e, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x68, 0x6f, 0x72, 0x74, 0x73, 0x5f, 0x63, 0x6f,
	0x75, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0b, 0x73, 0x68, 0x6f, 0x72, 0x74,
	0x73, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x6c, 0x6f, 0x6e, 0x67, 0x5f, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x01, 0x52, 0x09, 0x6c, 0x6f, 0x6e, 0x67,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x73, 0x68, 0x6f, 0x72, 0x74, 0x5f, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0a, 0x73, 0x68, 0x6f, 0x72,
	0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x2b, 0x0a, 0x11, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69,
	0x6e, 0x67, 0x5f, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28,
	0x01, 0x52, 0x10, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x45, 0x78, 0x70, 0x6f, 0x73,
	0x75, 0x72, 0x65, 0x12, 0x27, 0x0a, 0x0f, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x65, 0x78,
	0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0e, 0x65, 0x6e,
	0x64, 0x69, 0x6e, 0x67, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x12, 0x23, 0x0a, 0x0d,
	0x6c, 0x6f, 0x6e, 0x67, 0x5f, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x18, 0x0b, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x0c, 0x6c, 0x6f, 0x6e, 0x67, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72,
	0x65, 0x12, 0x25, 0x0a, 0x0e, 0x73, 0x68, 0x6f, 0x72, 0x74, 0x5f, 0x65, 0x78, 0x70, 0x6f, 0x73,
	0x75, 0x72, 0x65, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0d, 0x73, 0x68, 0x6f, 0x72, 0x74,
	0x45, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x61, 0x70, 0x69,
	0x74, 0x61, 0x6c, 0x5f, 0x75, 0x73, 0x65, 0x64, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b,
	0x63, 0x61, 0x70, 0x69, 0x74, 0x61, 0x6c, 0x55, 0x73, 0x65, 0x64, 0x12, 0x25, 0x0a, 0x0e, 0x67,
	0x72, 0x6f, 0x73, 0x73, 0x5f, 0x6c, 0x65, 0x76, 0x65, 0x72, 0x61, 0x67, 0x65, 0x18, 0x0e, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x0d, 0x67, 0x72, 0x6f, 0x73, 0x73, 0x4c, 0x65, 0x76, 0x65, 0x72, 0x61,
	0x67, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x6e, 0x65, 0x74, 0x5f, 0x6c, 0x65, 0x76, 0x65, 0x72, 0x61,
	0x67, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b, 0x6e, 0x65, 0x74, 0x4c, 0x65, 0x76,
	0x65, 0x72, 0x61, 0x67, 0x65, 0x12, 0x25, 0x0a, 0x0e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e,
	0x67, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x10, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0d, 0x73,
	0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c,
	0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x11, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x0b, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12,
	0x23, 0x0a, 0x0d, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x63, 0x61, 0x73, 0x68,
	0x18, 0x12, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0c, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67,
	0x43, 0x61, 0x73, 0x68, 0x12, 0x1f, 0x0a, 0x0b, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x63,
	0x61, 0x73, 0x68, 0x18, 0x13, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0a, 0x65, 0x6e, 0x64, 0x69, 0x6e,
	0x67, 0x43, 0x61, 0x73, 0x68, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x61, 0x78, 0x5f, 0x64, 0x72, 0x61,
	0x77, 0x64, 0x6f, 0x77, 0x6e, 0x18, 0x14, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b, 0x6d, 0x61, 0x78,
	0x44, 0x72, 0x61, 0x77, 0x64, 0x6f, 0x77, 0x6e, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x61, 0x78, 0x5f,
	0x6c, 0x65, 0x76, 0x65, 0x72, 0x61, 0x67, 0x65, 0x18, 0x15, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b,
	0x6d, 0x61, 0x78, 0x4c, 0x65, 0x76, 0x65, 0x72, 0x61, 0x67, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x65,
	0x78, 0x63, 0x65, 0x73, 0x73, 0x5f, 0x72, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x18, 0x16, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x0c, 0x65, 0x78, 0x63, 0x65, 0x73, 0x73, 0x52, 0x65, 0x74, 0x75, 0x72, 0x6e,
	0x12, 0x34, 0x0a, 0x16, 0x74, 0x72, 0x65, 0x61, 0x73, 0x75, 0x72, 0x79, 0x5f, 0x70, 0x65, 0x72,
	0x69, 0x6f, 0x64, 0x5f, 0x72, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x18, 0x17, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x14, 0x74, 0x72, 0x65, 0x61, 0x73, 0x75, 0x72, 0x79, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64,
	0x52, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x12, 0x36, 0x0a, 0x17, 0x61, 0x6c, 0x67, 0x6f, 0x72, 0x69,
	0x74, 0x68, 0x6d, 0x5f, 0x70, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x5f, 0x72, 0x65, 0x74, 0x75, 0x72,
	0x6e, 0x18, 0x18, 0x20, 0x01, 0x28, 0x01, 0x52, 0x15, 0x61, 0x6c, 0x67, 0x6f, 0x72, 0x69, 0x74,
	0x68, 0x6d, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x52, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x12, 0x2c,
	0x0a, 0x0f, 0x61, 0x6c, 0x67, 0x6f, 0x5f, 0x76, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69, 0x74,
	0x79, 0x18, 0x19, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x0e, 0x61, 0x6c, 0x67, 0x6f, 0x56,
	0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x88, 0x01, 0x01, 0x12, 0x1b, 0x0a, 0x06,
	0x73, 0x68, 0x61, 0x72, 0x70, 0x65, 0x18, 0x1a, 0x20, 0x01, 0x28, 0x01, 0x48, 0x01, 0x52, 0x06,
	0x73, 0x68, 0x61, 0x72, 0x70, 0x65, 0x88, 0x01, 0x01, 0x12, 0x1d, 0x0a, 0x07, 0x73, 0x6f, 0x72,
	0x74, 0x69, 0x6e, 0x6f, 0x18, 0x1b, 0x20, 0x01, 0x28, 0x01, 0x48, 0x02, 0x52, 0x07, 0x73, 0x6f,
	0x72, 0x74, 0x69, 0x6e, 0x6f, 0x88, 0x01, 0x01, 0x12, 0x3b, 0x0a, 0x17, 0x62, 0x65, 0x6e, 0x63,
	0x68, 0x6d, 0x61, 0x72, 0x6b, 0x5f, 0x70, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x5f, 0x72, 0x65, 0x74,
	0x75, 0x72, 0x6e, 0x18, 0x1c, 0x20, 0x01, 0x28, 0x01, 0x48, 0x03, 0x52, 0x15, 0x62, 0x65, 0x6e,
	0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x52, 0x65, 0x74, 0x75,
	0x72, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x36, 0x0a, 0x14, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61,
	0x72, 0x6b, 0x5f, 0x76, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x18, 0x1d, 0x20,
	0x01, 0x28, 0x01, 0x48, 0x04, 0x52, 0x13, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b,
	0x56, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x88, 0x01, 0x01, 0x12, 0x19, 0x0a,
	0x05, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x18, 0x1e, 0x20, 0x01, 0x28, 0x01, 0x48, 0x05, 0x52, 0x05,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x88, 0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x62, 0x65, 0x74, 0x61,
	0x18, 0x1f, 0x20, 0x01, 0x28, 0x01, 0x48, 0x06, 0x52, 0x04, 0x62, 0x65, 0x74, 0x61, 0x88, 0x01,
	0x01, 0x12, 0x3b, 0x0a, 0x09, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x20,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75,
	0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x50, 0x6f, 0x73, 0x69, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x09, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42, 0x12,
	0x0a, 0x10, 0x5f, 0x61, 0x6c, 0x67, 0x6f, 0x5f, 0x76, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69,
	0x74, 0x79, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x73, 0x68, 0x61, 0x72, 0x70, 0x65, 0x42, 0x0a, 0x0a,
	0x08, 0x5f, 0x73, 0x6f, 0x72, 0x74, 0x69, 0x6e, 0x6f, 0x42, 0x1a, 0x0a, 0x18, 0x5f, 0x62, 0x65,
	0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x5f, 0x70, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x5f, 0x72,
	0x65, 0x74, 0x75, 0x72, 0x6e, 0x42, 0x17, 0x0a, 0x15, 0x5f, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d,
	0x61, 0x72, 0x6b, 0x5f, 0x76, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x42, 0x08,
	0x0a, 0x06, 0x5f, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x62, 0x65, 0x74,
	0x61, 0x42, 0x33, 0x5a, 0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6c, 0x68, 0x6a, 0x6e, 0x69, 0x6c, 0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76,
	0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74,
	0x65, 0x73, 0x74, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_backtest_execution_proto_rawDescOnce sync.Once
	file_foreverbull_backtest_execution_proto_rawDescData = file_foreverbull_backtest_execution_proto_rawDesc
)

func file_foreverbull_backtest_execution_proto_rawDescGZIP() []byte {
	file_foreverbull_backtest_execution_proto_rawDescOnce.Do(func() {
		file_foreverbull_backtest_execution_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_backtest_execution_proto_rawDescData)
	})
	return file_foreverbull_backtest_execution_proto_rawDescData
}

var file_foreverbull_backtest_execution_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_foreverbull_backtest_execution_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_foreverbull_backtest_execution_proto_goTypes = []any{
	(Execution_Status_Status)(0),  // 0: foreverbull.backtest.Execution.Status.Status
	(*Execution)(nil),             // 1: foreverbull.backtest.Execution
	(*Period)(nil),                // 2: foreverbull.backtest.Period
	(*Execution_Status)(nil),      // 3: foreverbull.backtest.Execution.Status
	(*pb.Date)(nil),               // 4: foreverbull.common.Date
	(*pb1.Position)(nil),          // 5: foreverbull.finance.Position
	(*timestamppb.Timestamp)(nil), // 6: google.protobuf.Timestamp
}
var file_foreverbull_backtest_execution_proto_depIdxs = []int32{
	4, // 0: foreverbull.backtest.Execution.start_date:type_name -> foreverbull.common.Date
	4, // 1: foreverbull.backtest.Execution.end_date:type_name -> foreverbull.common.Date
	3, // 2: foreverbull.backtest.Execution.statuses:type_name -> foreverbull.backtest.Execution.Status
	2, // 3: foreverbull.backtest.Execution.result:type_name -> foreverbull.backtest.Period
	4, // 4: foreverbull.backtest.Period.date:type_name -> foreverbull.common.Date
	5, // 5: foreverbull.backtest.Period.positions:type_name -> foreverbull.finance.Position
	0, // 6: foreverbull.backtest.Execution.Status.status:type_name -> foreverbull.backtest.Execution.Status.Status
	6, // 7: foreverbull.backtest.Execution.Status.occurred_at:type_name -> google.protobuf.Timestamp
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_foreverbull_backtest_execution_proto_init() }
func file_foreverbull_backtest_execution_proto_init() {
	if File_foreverbull_backtest_execution_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_backtest_execution_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Execution); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_backtest_execution_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Period); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_backtest_execution_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*Execution_Status); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_foreverbull_backtest_execution_proto_msgTypes[0].OneofWrappers = []any{}
	file_foreverbull_backtest_execution_proto_msgTypes[1].OneofWrappers = []any{}
	file_foreverbull_backtest_execution_proto_msgTypes[2].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_foreverbull_backtest_execution_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_foreverbull_backtest_execution_proto_goTypes,
		DependencyIndexes: file_foreverbull_backtest_execution_proto_depIdxs,
		EnumInfos:         file_foreverbull_backtest_execution_proto_enumTypes,
		MessageInfos:      file_foreverbull_backtest_execution_proto_msgTypes,
	}.Build()
	File_foreverbull_backtest_execution_proto = out.File
	file_foreverbull_backtest_execution_proto_rawDesc = nil
	file_foreverbull_backtest_execution_proto_goTypes = nil
	file_foreverbull_backtest_execution_proto_depIdxs = nil
}
