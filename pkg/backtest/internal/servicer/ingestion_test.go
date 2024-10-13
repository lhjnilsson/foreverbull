package servicer

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	pb_internal "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type IngestionServerTest struct {
	suite.Suite

	stream  *stream.MockStream
	storage *storage.MockStorage

	pgx *pgxpool.Pool

	listner *bufconn.Listener
	server  *grpc.Server
	client  pb.IngestionServicerClient
}

func TestIngestionServerTest(t *testing.T) {
	suite.Run(t, new(IngestionServerTest))
}

func (suite *IngestionServerTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *IngestionServerTest) TearDownSuite() {
}

func (suite *IngestionServerTest) SetupSubTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))

	suite.listner = bufconn.Listen(1024 * 1024)
	suite.server = grpc.NewServer()

	suite.stream = new(stream.MockStream)
	suite.storage = new(storage.MockStorage)
	server := NewIngestionServer(suite.stream, suite.storage, suite.pgx)
	pb.RegisterIngestionServicerServer(suite.server, server)
	go func() {
		suite.server.Serve(suite.listner)
	}()

	conn, err := grpc.DialContext(context.Background(), "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return suite.listner.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	suite.Require().NoError(err)
	suite.client = pb.NewIngestionServicerClient(conn)
}

func (suite *IngestionServerTest) TearDownSubTest() {
}

func (suite *IngestionServerTest) TestUpdateIngestion() {
	suite.Run("nothing stored", func() {
		suite.storage.On("CreateObject", mock.Anything, storage.IngestionsBucket,
			mock.Anything, mock.Anything).Return(nil, nil)
		suite.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return(nil)

		rsp, err := suite.client.UpdateIngestion(context.TODO(), &pb.UpdateIngestionRequest{})
		suite.Require().Error(err)
		suite.Require().Nil(rsp)
	})
	suite.Run("stored", func() {
		db := repository.Backtest{Conn: suite.pgx}
		ctx := context.TODO()
		_, err := db.Create(ctx, "nasdaq", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 06, Day: 01}, []string{"AAPL", "MSFT"}, nil)
		suite.NoError(err)
		_, err = db.Create(ctx, "nyse", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 04, Day: 01}, []string{"IBM", "GE"}, nil)
		suite.NoError(err)

		suite.storage.On("CreateObject", mock.Anything, storage.IngestionsBucket,
			mock.Anything, mock.Anything).Return(nil, nil)
		suite.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return(nil)

		rsp, err := suite.client.UpdateIngestion(context.TODO(), &pb.UpdateIngestionRequest{})
		suite.Require().NoError(err)
		suite.Require().NotNil(rsp)
	})
}

func (suite *IngestionServerTest) TestGetCurrentIngestion() {
	type TestCase struct {
		storedObjects []storage.Object
		expected      *pb.Ingestion
	}

	testCases := []TestCase{
		{
			storedObjects: []storage.Object{},
			expected:      nil,
		},
		{
			storedObjects: []storage.Object{
				{
					LastModified: time.Now(),
					Metadata: map[string]string{
						"Symbols":    "AAPL,MSFT",
						"Start_date": "2021-01-01",
						"End_date":   "2021-01-02",
						"Status":     pb.IngestionStatus_READY.String(),
					},
				},
			},
			expected: &pb.Ingestion{
				Symbols:   []string{"AAPL", "MSFT"},
				StartDate: &pb_internal.Date{Year: 2021, Month: 01, Day: 01},
				EndDate:   &pb_internal.Date{Year: 2021, Month: 01, Day: 02},
			},
		},
		{
			storedObjects: []storage.Object{
				{
					LastModified: time.Now().Add(-time.Hour),
					Metadata: map[string]string{
						"Symbols":    "TSLA,MMM",
						"Start_date": "2021-01-01",
						"End_date":   "2021-01-02",
						"Status":     pb.IngestionStatus_INGESTING.String(),
					},
				},
				{
					LastModified: time.Now(),
					Metadata: map[string]string{
						"Symbols":    "AAPL,MSFT",
						"Start_date": "2021-01-01",
						"End_date":   "2021-01-02",
						"Status":     pb.IngestionStatus_READY.String(),
					},
				},
				{
					LastModified: time.Now().Add(-24 * time.Hour),
					Metadata: map[string]string{
						"Symbols":    "X,Y,Z",
						"Start_date": "2021-01-01",
						"End_date":   "2021-01-02",
						"Status":     pb.IngestionStatus_INGESTING.String(),
					},
				},
			},
			expected: &pb.Ingestion{
				Symbols:   []string{"AAPL", "MSFT"},
				StartDate: &pb_internal.Date{Year: 2021, Month: 01, Day: 01},
				EndDate:   &pb_internal.Date{Year: 2021, Month: 01, Day: 02},
			},
		},
	}

	for i, tc := range testCases {
		suite.Run(fmt.Sprintf("test-%d", i), func() {
			suite.storage.On("ListObjects", mock.Anything, storage.IngestionsBucket).Return(&tc.storedObjects, nil)

			rsp, err := suite.client.GetCurrentIngestion(context.TODO(), &pb.GetCurrentIngestionRequest{})
			suite.Require().NoError(err)
			suite.Require().NotNil(rsp)
			suite.Require().Equal(tc.expected, rsp.Ingestion)
		})
	}
}
