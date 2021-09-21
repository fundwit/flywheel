package avatar

import (
	"bytes"
	"flywheel/bizerror"
	"flywheel/session"
	"io"
	"io/ioutil"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func TestDetailAvatar(t *testing.T) {
	GetObjectFunc = func(key string, o ...oss.Option) (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader([]byte(key + "=>hello world"))), nil
	}

	t.Run("Show be able to get avatar detail", func(t *testing.T) {
		r, err := DetailAvatar(123456)
		if string(r) != "avatars/123456.png=>hello world" || err != nil {
			t.Errorf("DetailAvatar(...) = (%v, %v), wants: 'avatars/123456.png=>hello world', nil", string(r), err)
		}
	})

	GetObjectFunc = func(key string, o ...oss.Option) (io.ReadCloser, error) {
		return nil, oss.ServiceError{Code: "NoSuchKey"}
	}
	t.Run("Show not found error when avatar not found", func(t *testing.T) {
		r, err := DetailAvatar(123456)
		if string(r) != "" || err != bizerror.ErrNotFound {
			t.Errorf("DetailAvatar(...) = (%v, %v), wants: '', %v", r, err, bizerror.ErrNotFound)
		}
	})
}

func TestCreateAvatar(t *testing.T) {
	var store string
	PutObjectFunc = func(s string, r io.Reader, o ...oss.Option) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		store = s + "=>" + string(b)
		return nil
	}

	t.Run("Show be able to save avatar by self", func(t *testing.T) {
		store = ""
		err := CreateAvatar(123456, bytes.NewReader([]byte("hello world")), &session.Session{Identity: session.Identity{ID: 123456}})
		if store != "avatars/123456.png=>hello world" || err != nil {
			t.Errorf("CreateAvatar(by self) = %v, %s, wants: nil, 'avatars/123456.png=>hello world'", err, store)
		}
	})

	t.Run("Show not be able to save avatar by other", func(t *testing.T) {
		store = ""
		err := CreateAvatar(123456, bytes.NewReader([]byte("hello world")), &session.Session{Identity: session.Identity{ID: 123}})
		if store != "" || err != bizerror.ErrForbidden {
			t.Errorf("CreateAvatar(by other) = %v, %s, wants: %v, ''", err, store, bizerror.ErrForbidden)
		}
	})
}
