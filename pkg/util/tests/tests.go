package tests

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// SetKlogV sets the logging flags when unit tests are run
func SetKlogV(level int) {
	l := strconv.Itoa(level)
	if f := flag.Lookup("v"); f != nil {
		f.Value.Set(l)
	}
	if f := flag.Lookup("logtostderr"); f != nil {
		f.Value.Set("true")
	}
}

// ReadBodyAndClose properly handles the reading of body and closing the reader
func ReadBodyAndClose(bodyReader io.ReadCloser) ([]byte, error) {
	defer bodyReader.Close()
	var body []byte
	if data, err := ioutil.ReadAll(bodyReader); err == nil {
		body = data
	} else {
		return nil, err
	}
	return body, nil
}

// CheckResponseErrorMessage reads and closes the response body payload (if any), and asserts
// that it matches the expectedMessage argument
func CheckResponseErrorMessage(t *testing.T, resp *http.Response, expectedMessage string) {
	assert.Equal(t, int64(len(expectedMessage)), resp.ContentLength)
	if body, err := ReadBodyAndClose(resp.Body); err == nil {
		assert.Equal(t, expectedMessage+"\n", string(body))
	} else {
		t.Error(err.Error())
	}
}

// RandHighPort return a free port in the range[1024,65535)
func RandHighPort() (randPort int) {
	for {
		randPort = 1024 + rand.Intn(1<<16-1024)
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", randPort))
		if err == nil {
			ln.Close()
			break
		}
	}
	return
}
