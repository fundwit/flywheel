package common

import "bytes"

func StringReader(str string) *bytes.Reader {
	return bytes.NewReader([]byte(str))
}
