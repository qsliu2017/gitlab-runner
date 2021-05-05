package s3

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/minio/minio-go/v6/pkg/s3utils"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type s3Adapter struct {
	timeout    time.Duration
	config     *common.CacheS3Config
	objectName string
	client     minioClient
}

func (a *s3Adapter) GetDownloadURL() *url.URL {
	URL, err := a.client.PresignedGetObject(context.Background(), a.config.BucketName, a.objectName, a.timeout, nil)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func (a *s3Adapter) GetUploadURL() *url.URL {
	URL, err := a.client.PresignedPutObject(context.Background(), a.config.BucketName, a.objectName, a.timeout)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func (a *s3Adapter) GetUploadHeaders() http.Header {
	return nil
}

func (a *s3Adapter) GetGoCloudURL() *url.URL {
	// Go Cloud omits the object name from the URL. Since object storage
	// providers use the URL host for the bucket name, we attach the
	// object name to avoid having to pass another parameter.
	u := &url.URL{
		Scheme: "s3",
		Host:   a.config.BucketName,
		Path:   a.objectName,
	}

	q := u.Query()
	q.Set("endpoint", a.config.GetEndpoint())
	q.Set("region", a.config.BucketLocation)
	q.Set("disableSSL", strconv.FormatBool(a.config.Insecure))
	q.Set("s3ForcePathStyle", strconv.FormatBool(!isVirtualHostSupported(a.config)))
	u.RawQuery = q.Encode()

	return u
}

func (a *s3Adapter) GetUploadEnv() map[string]string {
	return nil
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	s3 := config.S3
	if s3 == nil {
		return nil, fmt.Errorf("missing S3 configuration")
	}

	client, err := newMinioClient(s3)
	if err != nil {
		return nil, fmt.Errorf("error while creating S3 cache storage client: %w", err)
	}

	a := &s3Adapter{
		config:     s3,
		timeout:    timeout,
		objectName: objectName,
		client:     client,
	}

	return a, nil
}

func isVirtualHostSupported(c *common.CacheS3Config) bool {
	// We assume this is the default AWS server, so it must support virtual hosts.
	if c.ServerAddress == "" {
		return true
	}

	scheme := "https"
	if c.Insecure {
		scheme = "http"
	}

	u := url.URL{Scheme: scheme, Host: c.ServerAddress}

	// Since we don't have a `PathStyle` config option, we use Minio's utility class
	// to auto-detect whether the endpoint supports this.
	return s3utils.IsVirtualHostSupported(u, c.BucketName)
}

func init() {
	err := cache.Factories().Register("s3", New)
	if err != nil {
		panic(err)
	}
}
