package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestMinioClientFollowsMinioSecure(t *testing.T) {
	cases := []struct {
		value  string
		scheme string
	}{
		{value: "false", scheme: "http"},
		{value: "", scheme: "http"},
		{value: "true", scheme: "https"},
		{value: "1", scheme: "https"},
	}
	for _, tc := range cases {
		t.Run("MINIO_SECURE="+tc.value, func(t *testing.T) {
			t.Setenv(environment.MinioURL, "minio.example:9000")
			t.Setenv(environment.MinioAccessKey, "access")
			t.Setenv(environment.MinioSecretKey, "secret")
			t.Setenv(environment.MinioSecure, tc.value)

			client, err := newMinioClient()
			require.NoError(t, err)
			require.Equal(t, tc.scheme, client.EndpointURL().Scheme)
		})
	}
}
