package common

import (
	"fmt"
	"os"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

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
	cli, err := oss.New(endpoint, accesskey, secretKey)
	if err != nil {
		return nil, err
	}

	bucket, err := cli.Bucket(bucketName)
	if err != nil {
		return nil, err
	}
	return bucket, nil
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
