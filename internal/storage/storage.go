package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type BlobStorage interface {
	VerifyBuckets(ctx context.Context) error
	ListResults(ctx context.Context) (*[]Object, error)
	GetResultInfo(ctx context.Context, name string) (*Object, error)
	ListIngestions(ctx context.Context) (*[]Object, error)
}

type Object struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

func NewMinioStorage() (BlobStorage, error) {
	client, err := minio.New(environment.GetMinioURL(), &minio.Options{
		Creds:  credentials.NewStaticV4(environment.GetMinioAccessKey(), environment.GetMinioSecretKey(), ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	storage := &MinioStorage{client: client}
	return storage, nil
}

type MinioStorage struct {
	client *minio.Client
}

func (s *MinioStorage) VerifyBuckets(ctx context.Context) error {
	buckets := []string{"backtest-results", "backtest-ingestions"}
	for _, bucket := range buckets {
		exists, err := s.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("error checking if bucket exists: %w", err)
		}

		if !exists {
			err = s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			if err != nil {
				return fmt.Errorf("error creating bucket: %w", err)
			}
		}
	}
	return nil
}

func (s *MinioStorage) ListResults(ctx context.Context) (*[]Object, error) {
	objects := s.client.ListObjects(ctx, "backtest-results", minio.ListObjectsOptions{})
	results := []Object{}
	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}

		results = append(results, Object{
			Name:         object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
		})
	}
	return &results, nil
}

func (s *MinioStorage) GetResultInfo(ctx context.Context, name string) (*Object, error) {
	object, err := s.client.StatObject(ctx, "backtest-results", name, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	result := Object{
		Name:         object.Key,
		Size:         object.Size,
		LastModified: object.LastModified,
	}
	return &result, nil
}

func (s *MinioStorage) ListIngestions(ctx context.Context) (*[]Object, error) {
	objects := s.client.ListObjects(ctx, "backtest-ingestions", minio.ListObjectsOptions{})
	results := []Object{}
	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}

		results = append(results, Object{
			Name:         object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
		})
	}
	return &results, nil
}

func NewLocalStorage() (*LocalStorage, error) {
	return &LocalStorage{CreatedBuckets: []string{}}, nil
}

type LocalStorage struct {
	CreatedBuckets []string
}

func (s *LocalStorage) VerifyBucket(ctx context.Context, bucket string) error {
	s.CreatedBuckets = append(s.CreatedBuckets, bucket)
	return nil
}
