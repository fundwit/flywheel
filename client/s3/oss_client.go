package s3

import (
	"flywheel/session"
	"fmt"
	"io"
	"os"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

var (
	AvatarBucket  *oss.Bucket
	GetObjectFunc func(string, *session.Session, ...oss.Option) (io.ReadCloser, error)
	PutObjectFunc func(string, io.Reader, *session.Session, ...oss.Option) error
)

func Bootstrap() {
	var err error
	AvatarBucket, err = BuildBucketFromEnv()
	if err != nil {
		panic(err)
	}

	GetObjectFunc = GetObject
	PutObjectFunc = PutObject
}

func BuildBucketFromEnv() (*oss.Bucket, error) {
	endpoint := os.ExpandEnv(os.Getenv("OSS_ENDPOINT"))
	if endpoint == "" {
		endpoint = "dummy"
	}
	accessKey := os.Getenv("OSS_ACCESS_KEY")
	secretKey := os.Getenv("OSS_SECRET_KEY")
	bucket := os.Getenv("OSS_BUCKET")
	if bucket == "" {
		bucket = "flywheel"
	}
	return BuildBucket(endpoint, accessKey, secretKey, bucket)
}

func BuildBucket(endpoint, accesskey, secretKey, bucketName string) (*oss.Bucket, error) {
	// endpoint http://oss-cn-hangzhou.aliyuncs.com
	cli, err := oss.New(endpoint, accesskey, secretKey, oss.HTTPClient(nil))
	if err != nil {
		return nil, err
	}

	bucket, err := cli.Bucket(bucketName)
	if err != nil {
		return nil, err
	}
	return bucket, nil
}

func GetObject(key string, s *session.Session, opts ...oss.Option) (io.ReadCloser, error) {
	var childSpan *opentracing.Span
	if s.Context != nil {
		parentSpan := opentracing.SpanFromContext(s.Context)
		if parentSpan != nil {
			tracer := parentSpan.Tracer()
			sp := tracer.StartSpan("get-object-async", opentracing.ChildOf(parentSpan.Context()))
			sp.SetTag("object-key", key)
			childSpan = &sp
			defer sp.Finish()
		}
	}

	r, err := AvatarBucket.GetObject(key, opts...)
	if childSpan != nil {
		ext.Error.Set(*childSpan, err != nil)
	}
	return r, err
}

func PutObject(key string, r io.Reader, s *session.Session, opts ...oss.Option) error {
	var childSpan *opentracing.Span
	if s.Context != nil {
		parentSpan := opentracing.SpanFromContext(s.Context)
		if parentSpan != nil {
			tracer := parentSpan.Tracer()
			sp := tracer.StartSpan("put-object-async", opentracing.ChildOf(parentSpan.Context()))
			sp.SetTag("object-key", key)
			childSpan = &sp
			defer sp.Finish()
		}
	}

	err := AvatarBucket.PutObject(key, r, opts...)
	if childSpan != nil {
		ext.Error.Set(*childSpan, err != nil)
	}
	return err
}

func ListObject(bucket *oss.Bucket, prefix string) error {
	marker := oss.Marker("")
	pre := oss.Prefix(prefix)
	for {
		// default page size is 100
		r, err := bucket.ListObjects(marker, pre)
		if err != nil {
			return err
		}
		for _, o := range r.Objects {
			fmt.Println("Bucket: ", o.Key)
		}
		if r.IsTruncated {
			pre = oss.Prefix(r.Prefix)
			marker = oss.Marker(r.NextMarker)
		} else {
			break
		}
	}
	return nil
}
