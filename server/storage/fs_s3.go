package storage

import (
	"bytes"
	"io"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

type s3FS struct{}

func (fs *s3FS) Open(bucket string, options url.Values) (FS, error) {
	accountId := options.Get("accountId")
	region := options.Get("region")
	s3Client, err := NewS3Client(&SimpleS3ClientConfig{
		Bucket:    &bucket,
		AccountId: &accountId,
		Region:    &region,
		Log:       log,
	})
	if err != nil {
		return nil, err
	}
	return &s3FSLayer{s3Client}, nil
}

type s3FSLayer struct {
	s3Client SimpleS3Client
}

func (fs *s3FSLayer) Exists(name string) (bool, time.Time, error) {
	var modtime time.Time
	result, err := fs.s3Client.Head(&name)
	if err != nil {
		// http://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
		// https://github.com/awsdocs/aws-doc-sdk-examples/blob/master/go/example_code/extending_sdk/handleServiceErrorCodes.go
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == s3.ErrCodeNoSuchKey {
				return false, modtime, nil
			}
		}
		return false, modtime, err
	}
	modtime = *result.LastModified
	return true, modtime, nil
}

func (fs *s3FSLayer) ReadFile(name string) (io.ReadSeekCloser, error) {
	// Create a file to write the S3 Object contents to.
	result, err := fs.s3Client.Get(&name)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}
	return aws.ReadSeekCloser(bytes.NewReader(data)), nil
}

func (fs *s3FSLayer) WriteFile(name string, content io.Reader) (int64, error) {
	_, err := fs.s3Client.Put(&name, aws.ReadSeekCloser(content))
	if err != nil {
		return 0, err
	}
	result, err := fs.s3Client.Head(&name)
	if err != nil {
		return 0, err
	}
	return aws.Int64Value(result.ContentLength), nil
}

func (fs *s3FSLayer) WriteData(name string, data []byte) error {
	_, err := fs.s3Client.Put(&name, bytes.NewReader(data))
	if err != nil {
		return err
	}
	return nil
}

func init() {
	RegisterFS("s3", &s3FS{})
}
