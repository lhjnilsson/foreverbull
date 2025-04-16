package stream_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RepositoryTest struct {
	suite.Suite

	db         *pgxpool.Pool
	repository *stream.Repository
}

func (test *RepositoryTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{Postgres: true})

	var err error
	test.db, err = pgxpool.New(context.TODO(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.repository = stream.NewRepository(test.db)
}

func (test *RepositoryTest) TearDownTest() {
}

func (test *RepositoryTest) SetupSubTest() {
	test.Require().NoError(stream.RecreateTables(context.TODO(), test.db))
}

func TestRepository(t *testing.T) {
	suite.Run(t, new(RepositoryTest))
}

func (test *RepositoryTest) TestCreateMessage() {
	test.Run("Create", func() {
		msg := pb.Message{
			Id:        "id",
			Module:    "module",
			Component: "component",
			Method:    "method",
			Payload:   []byte("{'key': 'value'}"),
		}
		err := test.repository.CreateMessage(context.TODO(), &msg)
		test.Require().NoError(err)
	})
	test.Run("CreateDuplicate", func() {
		msg := pb.Message{
			Id:        "id",
			Module:    "module",
			Component: "component",
			Method:    "method",
			Payload:   []byte("payload"),
		}
		err := test.repository.CreateMessage(context.TODO(), &msg)
		test.Require().NoError(err)

		err = test.repository.CreateMessage(context.TODO(), &msg)
		test.Require().Error(err)
	})
}

func (test *RepositoryTest) TestCreateMessageState() {
	createMessage := func() {
		msg := pb.Message{
			Id:        "id",
			Module:    "module",
			Component: "component",
			Method:    "method",
			Payload:   []byte("payload"),
		}
		err := test.repository.CreateMessage(context.TODO(), &msg)
		test.Require().NoError(err)
	}
	test.Run("Create", func() {
		createMessage()

		err := test.repository.CreateMessageState(context.TODO(), "id", &pb.State{
			State: pb.State_STATE_CREATED,
		})
		test.Require().NoError(err)
	})
}

func (test *RepositoryTest) TestGetMessage() {
	createMessage := func() {
		msg := pb.Message{
			Id:        "id",
			Module:    "module",
			Component: "component",
			Method:    "method",
			Payload:   []byte("payload"),
		}
		err := test.repository.CreateMessage(context.TODO(), &msg)
		test.Require().NoError(err)
	}
	createMessageState := func(state *pb.State) {
		err := test.repository.CreateMessageState(context.TODO(), "id", state)
		test.Require().NoError(err)
	}
	test.Run("GetMessage", func() {
		createMessage()
		createMessageState(&pb.State{
			State:      pb.State_STATE_CREATED,
			OccurredAt: timestamppb.Now(),
		})

		msg, states, err := test.repository.GetMessage(context.TODO(), "id")
		test.Require().NoError(err)

		test.Require().NotNil(msg)
		test.Require().Len(states, 1)
		test.Require().Equal(pb.State_STATE_CREATED, states[0].State)
	})
	test.Run("GetMessageNotFound", func() {
		msg, states, err := test.repository.GetMessage(context.TODO(), "id")
		test.Require().Error(err)
		test.Require().Nil(msg)
		test.Require().Empty(states)
	})
	test.Run("MultipleStates", func() {
		createMessage()
		createMessageState(&pb.State{
			State:      pb.State_STATE_PUBLISHED,
			OccurredAt: timestamppb.Now(),
		})
		createMessageState(&pb.State{
			State:      pb.State_STATE_RECEIVED,
			OccurredAt: timestamppb.Now(),
		})
		createMessageState(&pb.State{
			State:      pb.State_STATE_ERROR,
			Error:      func() *string { s := "error"; return &s }(),
			OccurredAt: timestamppb.Now(),
		})

		msg, states, err := test.repository.GetMessage(context.TODO(), "id")
		test.Require().NoError(err)
		test.Require().NotNil(msg)
		test.Require().Len(states, 3)
		test.Require().Equal(pb.State_STATE_ERROR, states[0].State)
		test.Require().Equal(pb.State_STATE_RECEIVED, states[1].State)
		test.Require().Equal(pb.State_STATE_PUBLISHED, states[2].State)
		test.Require().Equal("error", *states[0].Error)
	})
}
