package stream

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/stretchr/testify/suite"
)

type RepositoryTest struct {
	suite.Suite

	db         *pgxpool.Pool
	repository repository
}

func (test *RepositoryTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{Postgres: true})

	var err error
	test.db, err = pgxpool.New(context.TODO(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.repository = repository{db: test.db}
}

func (test *RepositoryTest) TearDownTest() {
}

func (test *RepositoryTest) SetupSubTest() {
	test.Require().NoError(RecreateTables(context.TODO(), test.db))
}

func TestRepository(t *testing.T) {
	suite.Run(t, new(RepositoryTest))
}

func (test *RepositoryTest) TestUpdatePublishedAndGetMessage() {
	createMessage := func(_ *testing.T) *message {
		orchestrationName := "test_orchestration"
		orchestrationID := "test_orchestration_id"
		OrchestrationStep := "test_step"
		OrchestrationStepNumber := 0
		OrchestrationFallbackStep := false
		msg := message{
			OrchestrationName:         &orchestrationName,
			OrchestrationID:           &orchestrationID,
			OrchestrationStep:         &OrchestrationStep,
			OrchestrationStepNumber:   &OrchestrationStepNumber,
			OrchestrationFallbackStep: &OrchestrationFallbackStep,
			Module:                    "test_module",
			Component:                 "test_component",
			Method:                    "test_method",
			Payload:                   nil,
		}
		test.Require().NoError(test.repository.CreateMessage(context.TODO(), &msg))
		test.Require().NotNil(msg.ID)

		return &msg
	}

	type TestCase struct {
		Status        MessageStatus
		ExpectMessage bool
	}

	testCases := []TestCase{
		{
			Status:        MessageStatusPublished,
			ExpectMessage: true,
		},
		{
			Status:        MessageStatusCreated,
			ExpectMessage: false,
		},
		{
			Status:        MessageStatusReceived,
			ExpectMessage: false,
		},
		{
			Status:        MessageStatusComplete,
			ExpectMessage: false,
		},
		{
			Status:        MessageStatusError,
			ExpectMessage: false,
		},
	}
	for _, testCase := range testCases {
		test.Run(string(testCase.Status), func() {
			msg := createMessage(test.T())
			test.Require().NoError(
				test.repository.UpdateMessageStatus(context.TODO(), *msg.ID, testCase.Status, nil),
			)

			msg, err := test.repository.UpdatePublishedAndGetMessage(context.TODO(), *msg.ID)
			if testCase.ExpectMessage {
				test.Require().NoError(err)
				test.NotNil(msg)
				test.Equal(MessageStatusReceived, msg.StatusHistory[0].Status)
			} else {
				test.Require().Error(err)
				test.Nil(msg)
			}
		})
	}
}

func (test *RepositoryTest) TestGetNextOrchestrationCommands() {
	createBaseOrchestration := func(_ *testing.T) *MessageOrchestration {
		baseOrchestration := NewMessageOrchestration("repository_test")

		msg1, err := NewMessage("service", "service", "start", nil)
		test.Require().NoError(err)
		msg2, err := NewMessage("service", "service", "start", nil)
		test.Require().NoError(err)
		baseOrchestration.AddStep("start", []Message{msg1, msg2})

		msg1, err = NewMessage("service", "service", "sanity_check", nil)
		test.Require().NoError(err)
		baseOrchestration.AddStep("sanity", []Message{msg1})

		msg1, err = NewMessage("backtest", "session", "run", nil)
		test.Require().NoError(err)
		baseOrchestration.AddStep("run", []Message{msg1})

		msg1, err = NewMessage("service", "instance", "stop", nil)
		test.Require().NoError(err)
		msg2, err = NewMessage("service", "instance", "stop", nil)
		test.Require().NoError(err)
		baseOrchestration.AddStep("stop", []Message{msg1, msg2})

		msg1, err = NewMessage("service", "instance", "stop", nil)
		test.Require().NoError(err)
		msg2, err = NewMessage("service", "instance", "stop", nil)
		test.Require().NoError(err)
		baseOrchestration.SettFallback([]Message{msg1, msg2})

		for _, step := range baseOrchestration.Steps {
			for _, cmd := range step.Commands {
				msg := cmd.(*message)
				err := test.repository.CreateMessage(context.TODO(), msg)
				test.Require().NoError(err)
			}
		}

		for _, cmd := range baseOrchestration.FallbackStep.Commands {
			msg := cmd.(*message)
			err := test.repository.CreateMessage(context.TODO(), msg)
			test.Require().NoError(err)
		}

		return baseOrchestration
	}

	isTrue := true
	isFalse := false

	type TestCase struct {
		Name             string
		CurrentStep      int
		ExpectedMessages *[]message
		StoredData       string
	}

	testCases := []TestCase{
		{
			Name:       "initial",
			StoredData: ``,
		},
		{
			Name:        "current step -1",
			CurrentStep: -1,
			StoredData:  ``,
			ExpectedMessages: &[]message{
				{},
				{},
			},
		},
		{
			Name:        "service started successfully",
			CurrentStep: 0,
			ExpectedMessages: &[]message{
				{OrchestrationFallbackStep: &isFalse, Module: "service", Component: "service", Method: "sanity_check"},
			},
			StoredData: `
UPDATE message SET status='COMPLETE' WHERE orchestration_step_number=0;
`,
		},
		{
			Name:        "One start failed",
			CurrentStep: 0,
			StoredData: `
UPDATE message SET status='COMPLETE' WHERE orchestration_step_number=0;
UPDATE message set status='ERROR' WHERE id IN (
	SELECT id FROM message where orchestration_step_number=0 limit 1
)`,
			ExpectedMessages: &[]message{
				{OrchestrationFallbackStep: &isTrue},
				{OrchestrationFallbackStep: &isTrue},
			},
		},
		{
			Name:        "Run failed",
			CurrentStep: 2,
			StoredData: `
UPDATE message SET status='COMPLETE' WHERE orchestration_step_number=0;
UPDATE message set status='COMPLETE' WHERE orchestration_step_number=1;
UPDATE message set status='ERROR' WHERE orchestration_step_number=2;`,
			ExpectedMessages: &[]message{
				{OrchestrationFallbackStep: &isTrue},
				{OrchestrationFallbackStep: &isTrue},
			},
		},
		{
			Name:        "All steps succeeded",
			CurrentStep: 3,
			StoredData: `
		UPDATE message SET status='COMPLETE' WHERE orchestration_fallback_step=false;`,
			ExpectedMessages: &[]message{},
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.Name, func() {
			baseOrchestration := createBaseOrchestration(test.T())

			_, err := test.db.Exec(context.TODO(), testCase.StoredData)
			test.Require().NoError(err)

			commands, err := test.repository.GetNextOrchestrationCommands(context.TODO(),
				baseOrchestration.OrchestrationID, testCase.CurrentStep)
			test.Require().NoError(err)

			if testCase.ExpectedMessages == nil {
				test.Empty(*commands)
			} else {
				test.Require().NotNil(commands)
				test.Equal(len(*testCase.ExpectedMessages), len(*commands))
			}
		})
	}
}
