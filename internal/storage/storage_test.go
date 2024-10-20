package storage

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
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
		cmd := exec.Command("dd", "if=/dev/urandom", "of="+file, "bs=1", "count=1024") //nolint: gosec
		_, err := cmd.Output()
		test.Require().NoError(err)
	}

	for _, file := range test.backtestIngestionFiles {
		cmd := exec.Command("dd", "if=/dev/urandom", "of="+file, "bs=1", "count=1024") //nolint: gosec
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

	s, err := NewMinioStorage(context.TODO())
	test.Require().NoError(err)
	test.storage = s.(*MinioStorage)
}

func (test *StorageTest) TearDownTest() {
}

func TestStorage(t *testing.T) {
	suite.Run(t, new(StorageTest))
}

func (test *StorageTest) TestStorage() {
	test.Run("ListOBjects", func() {
		objects, err := test.storage.ListObjects(context.Background(), ResultsBucket)
		test.Require().NoError(err)
		test.Require().Empty(*objects)
	})
	test.Run("Create and Get Object", func() {
		metadata := map[string]string{
			"Key":  "value",
			"Key2": "value2",
		}

		_, err := test.storage.CreateObject(context.Background(), ResultsBucket, "123.pickle", WithMetadata(metadata))
		test.Require().NoError(err)

		object, err := test.storage.GetObject(context.Background(), ResultsBucket, "123.pickle")
		test.Require().NoError(err)
		test.Require().NotNil(object)
		test.Require().Equal(metadata, object.Metadata)

		objects, err := test.storage.ListObjects(context.Background(), ResultsBucket)
		test.Require().NoError(err)
		test.NoError((*objects)[0].Refresh())
		test.Require().Len(*objects, 1)
		test.Require().Equal(metadata, (*objects)[0].Metadata)
	})
}

func (test *StorageTest) TestObject() {
	object, err := test.storage.CreateObject(context.Background(), ResultsBucket, "123.pickle")
	test.Require().NoError(err)

	test.Run("PresignedGetURL", func() {
		url, err := object.PresignedGetURL()
		test.Require().NoError(err)
		test.Require().NotEmpty(url)
	})
	test.Run("PresignedPutURL", func() {
		url, err := object.PresignedPutURL()
		test.Require().NoError(err)
		test.Require().NotEmpty(url)
	})
	test.Run("Upload Example", func() {
		metadata := map[string]string{
			"Symbols": "AAPL,MSFT",
			"Start":   "2020-01-01",
			"End":     "2020-01-02",
			"Status":  "created",
		}

		object, err := test.storage.CreateObject(context.Background(), IngestionsBucket, "yeah",
			WithMetadata(metadata))
		test.Require().NoError(err)
		test.Require().NotNil(object)
		test.Require().Equal(metadata, object.Metadata)

		url, err := object.PresignedPutURL()
		test.Require().NoError(err)
		test.Require().NotEmpty(url)

		// Upload 123.pibckle using the presigned URL    // Read the local file
		fileData, err := os.ReadFile("123.pickle")
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(context.TODO(), http.MethodPut, url, bytes.NewReader(fileData))
		if err != nil {
			return
		}

		req.Header.Set("Content-Type", "application/octet-stream")

		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			test.T().Log("Error uploading file:", err)
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			test.T().Log("Upload failed with status code:", resp.StatusCode)
			return
		}

		test.Require().NoError(object.Refresh())
		test.Empty(object.Metadata)
		test.NoError(object.SetMetadata(context.TODO(), map[string]string{"Status": "uploaded"}))
		test.Require().NoError(object.Refresh())
		test.Require().Equal(map[string]string{"Status": "uploaded"}, object.Metadata)
		test.Positive(object.Size)
	})
}
