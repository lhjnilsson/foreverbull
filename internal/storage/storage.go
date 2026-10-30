package storage

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	PresignedURLExpiry = 24 * time.Hour

	// /minio/health/live can succeed while BucketExists still returns
	// "Server not initialized yet". Ten tries at 300ms is the bound.
	minioInitAttempts = 10
	minioInitDelay    = 300 * time.Millisecond
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

const (
	ResultsBucket    Bucket = "results"
	IngestionsBucket Bucket = "ingestions"
)

type Storage interface {
	ListObjects(ctx context.Context, bucket Bucket) (*[]Object, error)
	GetObject(ctx context.Context, bucket Bucket, name string) (*Object, error)
	CreateObject(ctx context.Context, bucket Bucket, name string,
		opts ...func(*minio.PutObjectOptions) error) (*Object, error)
}

type Object struct {
	client *minio.Client

	Bucket       Bucket
	Name         string
	Size         int64
	LastModified time.Time
	Metadata     map[string]string
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
	url, err := o.client.PresignedGetObject(context.Background(), string(o.Bucket), o.Name, PresignedURLExpiry, nil)
	if err != nil {
		return "", fmt.Errorf("error creating presigned get url: %w", err)
	}

	return url.String(), nil
}

func (o *Object) PresignedPutURL() (string, error) {
	url, err := o.client.PresignedPutObject(context.Background(), string(o.Bucket), o.Name, PresignedURLExpiry)
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

// newMinioClient dials MINIO_URL over TLS when MINIO_SECURE is true.
func newMinioClient() (*minio.Client, error) {
	client, err := minio.New(environment.GetMinioURL(), &minio.Options{
		Creds:  credentials.NewStaticV4(environment.GetMinioAccessKey(), environment.GetMinioSecretKey(), ""),
		Secure: environment.GetMinioSecure(),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating minio client: %w", err)
	}
	return client, nil
}

func NewMinioStorage(ctx context.Context) (Storage, error) {
	client, err := newMinioClient()
	if err != nil {
		return nil, err
	}

	storage := &MinioStorage{client: client}

	for _, bucket := range []Bucket{ResultsBucket, IngestionsBucket} {
		if err := ensureBucket(ctx, client, bucket); err != nil {
			return nil, err
		}
	}

	return storage, nil
}

func minioNotInitialized(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Server not initialized yet")
}

func waitMinioInit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	timer := time.NewTimer(minioInitDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryWhileMinioStarts repeats call while MinIO reports that it is still
// starting. Any other error is returned on the first attempt.
func retryWhileMinioStarts(ctx context.Context, op string, call func() error) error {
	var err error

	for attempt := 1; attempt <= minioInitAttempts; attempt++ {
		err = call()
		if !minioNotInitialized(err) {
			break
		}

		if attempt == minioInitAttempts {
			break
		}

		if waitErr := waitMinioInit(ctx); waitErr != nil {
			return fmt.Errorf("%s: %w", op, waitErr)
		}
	}

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket Bucket) error {
	var exists bool

	err := retryWhileMinioStarts(ctx, "error checking if bucket exists", func() error {
		var callErr error
		exists, callErr = client.BucketExists(ctx, string(bucket))

		return callErr
	})
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return retryWhileMinioStarts(ctx, "error creating bucket", func() error {
		return client.MakeBucket(ctx, string(bucket), minio.MakeBucketOptions{})
	})
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

// IngestionReadable reports whether the ingestions object can be read.
// A missing key is false. Other storage errors are returned.
func (s *MinioStorage) IngestionReadable(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, string(IngestionsBucket), key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("%s: %w", IngestionsBucket, err)
}

// CopyIngestion copies one ingestions object onto another key.
// The source key is left in place.
func (s *MinioStorage) CopyIngestion(ctx context.Context, fromKey, toKey string) error {
	_, err := s.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: string(IngestionsBucket),
		Object: toKey,
	}, minio.CopySrcOptions{
		Bucket: string(IngestionsBucket),
		Object: fromKey,
	})
	if err != nil {
		return fmt.Errorf("%s: copy %s to %s: %w", IngestionsBucket, fromKey, toKey, err)
	}
	return nil
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

func (s *MinioStorage) CreateObject(ctx context.Context, bucket Bucket, name string,
	opts ...func(*minio.PutObjectOptions) error,
) (*Object, error) {
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
