package pb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TimeToProtoTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func DateToDateString(date *Date) string {
	return fmt.Sprintf("%d-%02d-%02d", date.Year, date.Month, date.Day)
}

func DateStringToDate(date string) *Date {
	splitted := strings.Split(date, "-")

	year, err := strconv.Atoi(splitted[0])
	if err != nil {
		log.Err(err).Msg("Fail to parse year")
		return &Date{Year: 0, Month: 0, Day: 0}
	}

	month, err := strconv.Atoi(splitted[1])
	if err != nil {
		log.Err(err).Msg("Fail to parse month")
		return &Date{Year: 0, Month: 0, Day: 0}
	}

	day, err := strconv.Atoi(splitted[2])
	if err != nil {
		log.Err(err).Msg("Fail to parse day")
		return &Date{Year: 0, Month: 0, Day: 0}
	}

	return &Date{
		Year:  int32(year),
		Month: int32(month),
		Day:   int32(day),
	}
}

func DateToTime(date *Date) time.Time {
	return time.Date(int(date.Year), time.Month(date.Month), int(date.Day), 0, 0, 0, 0, time.UTC)
}

func GoTimeToDate(t time.Time) *Date {
	return &Date{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}
}
