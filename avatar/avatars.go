package avatar

import (
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/session"
	"io"
	"io/ioutil"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/fundwit/go-commons/types"
)

var (
	AvatarBucket  *oss.Bucket
	GetObjectFunc func(string, ...oss.Option) (io.ReadCloser, error)
	PutObjectFunc func(string, io.Reader, ...oss.Option) error
)

func Bootstrap() {
	var err error
	AvatarBucket, err = common.BuildBucketFromEnv()
	if err != nil {
		panic(err)
	}

	GetObjectFunc = AvatarBucket.GetObject
	PutObjectFunc = AvatarBucket.PutObject
}

func DetailAvatar(id types.ID) ([]byte, error) {
	r, err := GetObjectFunc("avatars/" + id.String() + ".png")
	if err != nil {
		if serErr, ok := err.(oss.ServiceError); ok && serErr.Code == "NoSuchKey" {
			return nil, bizerror.ErrNotFound
		}
		return nil, err
	}
	return ioutil.ReadAll(r)
}

func CreateAvatar(id types.ID, r io.Reader, sec *session.Context) error {
	if id != sec.Identity.ID {
		return bizerror.ErrForbidden
	}

	return PutObjectFunc("avatars/"+id.String()+".png", r)
}
