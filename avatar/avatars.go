package avatar

import (
	"flywheel/bizerror"
	"flywheel/client/s3"
	"flywheel/session"
	"io"
	"io/ioutil"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/fundwit/go-commons/types"
)

func DetailAvatar(id types.ID, s *session.Session) ([]byte, error) {
	r, err := s3.GetObjectFunc("avatars/"+id.String()+".png", s)
	if err != nil {
		if serErr, ok := err.(oss.ServiceError); ok && serErr.Code == "NoSuchKey" {
			return nil, bizerror.ErrNotFound
		}
		return nil, err
	}
	return ioutil.ReadAll(r)
}

func CreateAvatar(id types.ID, r io.Reader, s *session.Session) error {
	if id != s.Identity.ID {
		return bizerror.ErrForbidden
	}

	return s3.PutObjectFunc("avatars/"+id.String()+".png", r, s)
}
