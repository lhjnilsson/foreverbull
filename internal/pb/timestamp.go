package pb

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TimeToProtoTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}
