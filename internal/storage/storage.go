package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func WithMetadata(metadata map[string]string) func(*minio.PutObjectOptions) error {
	return func(obj *minio.PutObjectOptions) error {
		if obj.UserMetadata == nil {
			obj.UserMetadata = make(map[string]string)
		}

		for k, v := range metadata {
			obj.UserMetadata[k] = v
		}

		return nil
	}
}

type Bucket string

var (
	ResultsBucket    Bucket = "results"
	IngestionsBucket Bucket = "ingestions"
)

type Storage interface {
	ListObjects(ctx context.Context, bucket Bucket) (*[]Object, error)
	GetObject(ctx context.Context, bucket Bucket, name string) (*Object, error)
	CreateObject(ctx context.Context, bucket Bucket, name string, opts ...func(*minio.PutObjectOptions) error) (*Object, error)
}

type Object struct {
	client *minio.Client

	Bucket       Bucket            `json:"bucket"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	LastModified time.Time         `json:"last_modified"`
	Metadata     map[string]string `json:"metadata"`
}

func (o *Object) Refresh() error {
	obj, err := o.client.StatObject(context.Background(), string(o.Bucket), o.Name, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("error refreshing object: %w", err)
	}

	o.Size = obj.Size
	o.LastModified = obj.LastModified
	o.Metadata = obj.UserMetadata

	return nil
}

func (o *Object) PresignedGetURL() (string, error) {
	url, err := o.client.PresignedGetObject(context.Background(), string(o.Bucket), o.Name, time.Hour*24, nil)
	if err != nil {
		return "", fmt.Errorf("error creating presigned get url: %w", err)
	}

	return url.String(), nil
}

func (o *Object) PresignedPutURL() (string, error) {
	url, err := o.client.PresignedPutObject(context.Background(), string(o.Bucket), o.Name, time.Hour*24)
	if err != nil {
		return "", fmt.Errorf("error creating presigned put url: %w", err)
	}

	return url.String(), nil
}

func (o *Object) SetMetadata(ctx context.Context, metadata map[string]string) error {
	for k, v := range metadata {
		o.Metadata[k] = v
	}

	_, err := o.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket:          string(o.Bucket),
		Object:          o.Name,
		UserMetadata:    o.Metadata,
		ReplaceMetadata: true,
	}, minio.CopySrcOptions{
		Bucket: string(o.Bucket),
		Object: o.Name,
	})
	if err != nil {
		return fmt.Errorf("error copying object: %w", err)
	}

	return nil
}

func NewMinioStorage(ctx context.Context) (Storage, error) {
	client, err := minio.New(environment.GetMinioURL(), &minio.Options{
		Creds:  credentials.NewStaticV4(environment.GetMinioAccessKey(), environment.GetMinioSecretKey(), ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating minio client: %w", err)
	}

	storage := &MinioStorage{client: client}

	for _, bucket := range []Bucket{ResultsBucket, IngestionsBucket} {
		exists, err := client.BucketExists(ctx, string(bucket))
		if err != nil {
			return nil, fmt.Errorf("error checking if bucket exists: %w", err)
		}

		if !exists {
			err = client.MakeBucket(ctx, string(bucket), minio.MakeBucketOptions{})
			if err != nil {
				return nil, fmt.Errorf("error creating bucket: %w", err)
			}
		}
	}

	return storage, nil
}

type MinioStorage struct {
	client *minio.Client
}

func (s *MinioStorage) ListObjects(ctx context.Context, bucket Bucket) (*[]Object, error) {
	objects := s.client.ListObjects(ctx, string(bucket), minio.ListObjectsOptions{})
	results := []Object{}

	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}

		results = append(results, Object{
			client: s.client,

			Bucket:       bucket,
			Name:         object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			Metadata:     object.UserMetadata,
		})
	}

	return &results, nil
}

func (s *MinioStorage) GetObject(ctx context.Context, bucket Bucket, name string) (*Object, error) {
	object, err := s.client.StatObject(ctx, string(bucket), name, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting object: %w", err)
	}

	result := Object{
		client: s.client,

		Bucket:       bucket,
		Name:         object.Key,
		Size:         object.Size,
		LastModified: object.LastModified,
		Metadata:     object.UserMetadata,
	}

	return &result, nil
}

func (s *MinioStorage) CreateObject(ctx context.Context, bucket Bucket, name string, opts ...func(*minio.PutObjectOptions) error) (*Object, error) {
	putOptions := minio.PutObjectOptions{}

	for _, opt := range opts {
		if err := opt(&putOptions); err != nil {
			return nil, fmt.Errorf("error applying option: %w", err)
		}
	}

	_, err := s.client.PutObject(ctx, string(bucket), name, nil, 0, putOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating object: %w", err)
	}

	return s.GetObject(ctx, bucket, name)
}
