// +build !integration

package s3

import (
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var defaultTimeout = 1 * time.Hour
var bucketName = "test"
var objectName = "key"
var bucketLocation = "location"

const (
	bucketName     = "test"
	objectName     = "key"
	bucketLocation = "location"
)

func defaultCacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "s3",
		S3: &common.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     bucketName,
			BucketLocation: bucketLocation},
	}
}

type cacheOperationTest struct {
	errorOnMinioClientInitialization bool
	errorOnURLPresigning             bool

	presignedURL *url.URL
	expectedURL  *url.URL
}

func onFakeMinioURLGenerator(tc cacheOperationTest) func() {
	client := new(mockMinioClient)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On("PresignedGetObject", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(tc.presignedURL, err)
	client.
		On("PresignedPutObject", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(tc.presignedURL, err)

	oldNewMinioURLGenerator := newMinioClient
	newMinioClient = func(s3 *common.CacheS3Config) (minioClient, error) {
		if tc.errorOnMinioClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	return func() {
		newMinioClient = oldNewMinioURLGenerator
	}
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) *url.URL,
	tc cacheOperationTest,
) {
	t.Run(operationName, func(t *testing.T) {
		cleanupMinioURLGeneratorMock := onFakeMinioURLGenerator(tc)
		defer cleanupMinioURLGeneratorMock()

		cacheConfig := defaultCacheFactory()

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnMinioClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL)

		headers := adapter.GetUploadHeaders()
		assert.Nil(t, headers)
		assert.Empty(t, adapter.GetUploadEnv())

		url := adapter.GetGoCloudURL()
		assert.NotNil(t, url)
		assert.Equal(t, "s3", url.Scheme)
		assert.Equal(t, bucketName, url.Host)
		assert.Equal(t, objectName, url.Path)

		q := url.Query()
		assert.Equal(t, bucketLocation, q.Get("region"))
		assert.Equal(t, "https://server.com", q.Get("endpoint"))
		assert.Equal(t, "false", q.Get("disableSSL"))
		assert.Equal(t, "true", q.Get("s3ForcePathStyle"))
	})
}

func TestCacheOperation(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)

	tests := map[string]cacheOperationTest{
		"error-on-minio-client-initialization": {
			errorOnMinioClientInitialization: true,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning: true,
			presignedURL:         URL,
			expectedURL:          nil,
		},
		"presigned-url": {
			presignedURL: URL,
			expectedURL:  URL,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetDownloadURL() },
				test,
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetUploadURL() },
				test,
			)
		})
	}
}

func TestCacheOperation_GetGoCloudURL(t *testing.T) {
	tests := map[string]struct {
		s3         *common.CacheS3Config
		disableSSL bool
		pathStyle  bool
	}{
		"defaults": {
			s3: &common.CacheS3Config{
				BucketName: bucketName,
			},
			disableSSL: false,
			pathStyle:  false,
		},
		"defaults with region": {
			s3: &common.CacheS3Config{
				BucketName:     bucketName,
				BucketLocation: bucketLocation,
			},
			disableSSL: false,
			pathStyle:  false,
		},
		"custom server": {
			s3: &common.CacheS3Config{
				BucketName:     bucketName,
				ServerAddress:  "minio.example.com",
				BucketLocation: bucketLocation,
			},
			disableSSL: false,
			pathStyle:  true,
		},
		"insecure custom server": {
			s3: &common.CacheS3Config{
				BucketName:     bucketName,
				ServerAddress:  "minio.example.com",
				BucketLocation: bucketLocation,
				Insecure:       true,
			},
			disableSSL: true,
			pathStyle:  true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cacheConfig := &common.CacheConfig{
				Type: "s3",
				S3:   tt.s3,
			}

			adapter, err := New(cacheConfig, defaultTimeout, objectName)
			require.NoError(t, err)

			url := adapter.GetGoCloudURL()
			assert.NotNil(t, url)
			assert.Equal(t, "s3", url.Scheme)
			assert.Equal(t, tt.s3.BucketName, url.Host)
			assert.Equal(t, objectName, url.Path)

			q := url.Query()
			assert.Equal(t, tt.s3.BucketLocation, q.Get("region"))
			assert.Equal(t, tt.s3.GetEndpoint(), q.Get("endpoint"))

			disableSSL, err := strconv.ParseBool(q.Get("disableSSL"))
			require.NoError(t, err)
			assert.Equal(t, tt.disableSSL, disableSSL)

			pathStyle, err := strconv.ParseBool(q.Get("s3ForcePathStyle"))
			require.NoError(t, err)
			assert.Equal(t, tt.pathStyle, pathStyle)
		})
	}
}

func TestNoConfiguration(t *testing.T) {
	s3Cache := defaultCacheFactory()
	s3Cache.S3 = nil

	adapter, err := New(s3Cache, defaultTimeout, objectName)
	assert.Nil(t, adapter)

	assert.EqualError(t, err, "missing S3 configuration")
}
