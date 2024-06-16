package storage

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/suite"
)

type StorageTest struct {
	suite.Suite

	storage *MinioStorage

	backtestResultFiles    []string
	backtestIngestionFiles []string
}

func (test *StorageTest) SetupSuite() {
	test.backtestResultFiles = []string{
		"123.pickle",
		"456.pickle",
		"789.pickle",
	}

	test.backtestIngestionFiles = []string{
		"123.tar.gz",
		"456.tar.gz",
		"789.tar.gz",
	}

	for _, file := range test.backtestResultFiles {
		cmd := exec.Command("dd", "if=/dev/urandom", "of="+file, "bs=1", "count=1024")
		_, err := cmd.Output()
		test.Require().NoError(err)
	}

	for _, file := range test.backtestIngestionFiles {
		cmd := exec.Command("dd", "if=/dev/urandom", "of="+file, "bs=1", "count=1024")
		_, err := cmd.Output()
		test.Require().NoError(err)
	}
}

func (test *StorageTest) TearDownSuite() {
	for _, file := range test.backtestResultFiles {
		err := exec.Command("rm", file).Run()
		test.Require().NoError(err)
	}
	for _, file := range test.backtestIngestionFiles {
		err := exec.Command("rm", file).Run()
		test.Require().NoError(err)
	}
}

func (test *StorageTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Minio: true,
	})

	s, err := NewMinioStorage()
	test.Require().NoError(err)
	test.storage = s.(*MinioStorage)

	err = test.storage.VerifyBuckets(context.TODO())
	test.NoError(err)
}

func (test *StorageTest) TearDownTest() {
}

func TestStorage(t *testing.T) {
	suite.Run(t, new(StorageTest))
}

func (test *StorageTest) TestResults() {

	// Upload sample files
	for i, file := range test.backtestResultFiles {
		_, err := test.storage.client.FPutObject(context.TODO(), "backtest-results", file, file, minio.PutObjectOptions{
			UserMetadata: map[string]string{
				"Backtest_id": fmt.Sprintf("%d", i),
			},
		})
		test.NoError(err)
	}
	test.Run("ListResults", func() {
		results, err := test.storage.ListResults(context.TODO())
		test.NoError(err)
		test.Len(*results, 3)
	})
	test.Run("GetResultInfo", func() {
		for i, file := range test.backtestResultFiles {
			result, err := test.storage.GetResultInfo(context.TODO(), file)
			test.NoError(err)
			test.Equal(file, result.Name)
			test.Equal(int64(1024), result.Size)
			test.NotNil(result.LastModified)
			test.Equal(fmt.Sprintf("%d", i), result.Metadata["Backtest_id"])
		}
	})
}

func (test *StorageTest) TestIngestions() {
	// Upload sample files
	for i, file := range test.backtestIngestionFiles {
		_, err := test.storage.client.FPutObject(context.TODO(), "backtest-ingestions", file, file, minio.PutObjectOptions{
			UserMetadata: map[string]string{
				"Ingestion_id": fmt.Sprintf("%d", i),
			},
		})
		test.NoError(err)
	}
	test.Run("ListIngestions", func() {
		results, err := test.storage.ListIngestions(context.TODO())
		test.NoError(err)
		test.Len(*results, 3)
	})
	test.Run("GetIngestionInfo", func() {
		for i, file := range test.backtestIngestionFiles {
			result, err := test.storage.GetIngestionInfo(context.TODO(), file)
			test.NoError(err)
			test.Equal(file, result.Name)
			test.Equal(int64(1024), result.Size)
			test.NotNil(result.LastModified)
			test.Equal(fmt.Sprintf("%d", i), result.Metadata["Ingestion_id"])
		}
	})
}
