package limiter

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rubiojr/rapi/internal/test"
)

func TestLimiterWrapping(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	writer := new(bytes.Buffer)

	for _, limits := range []struct {
		upstream   int
		downstream int
	}{
		{0, 0},
		{42, 0},
		{0, 42},
		{42, 42},
	} {
		limiter := NewStaticLimiter(limits.upstream*1024, limits.downstream*1024)

		mustWrapUpstream := limits.upstream > 0
		test.Equals(t, limiter.Upstream(reader) != reader, mustWrapUpstream)
		test.Equals(t, limiter.UpstreamWriter(writer) != writer, mustWrapUpstream)

		mustWrapDownstream := limits.downstream > 0
		test.Equals(t, limiter.Downstream(reader) != reader, mustWrapDownstream)
		test.Equals(t, limiter.DownstreamWriter(writer) != writer, mustWrapDownstream)
	}
}

type tracedReadCloser struct {
	io.Reader
	Closed bool
}

func newTracedReadCloser(rd io.Reader) *tracedReadCloser {
	return &tracedReadCloser{Reader: rd}
}

func (r *tracedReadCloser) Close() error {
	r.Closed = true
	return nil
}

func TestRoundTripperReader(t *testing.T) {
	limiter := NewStaticLimiter(42*1024, 42*1024)
	data := make([]byte, 1234)
	_, err := io.ReadFull(rand.Reader, data)
	test.OK(t, err)

	var send *tracedReadCloser = newTracedReadCloser(bytes.NewReader(data))
	var recv *tracedReadCloser

	rt := limiter.Transport(roundTripper(func(req *http.Request) (*http.Response, error) {
		buf := new(bytes.Buffer)
		_, err := io.Copy(buf, req.Body)
		if err != nil {
			return nil, err
		}
		err = req.Body.Close()
		if err != nil {
			return nil, err
		}

		recv = newTracedReadCloser(bytes.NewReader(buf.Bytes()))
		return &http.Response{Body: recv}, nil
	}))

	res, err := rt.RoundTrip(&http.Request{Body: send})
	test.OK(t, err)

	out := new(bytes.Buffer)
	n, err := io.Copy(out, res.Body)
	test.OK(t, err)
	test.Equals(t, int64(len(data)), n)
	test.OK(t, res.Body.Close())

	test.Assert(t, send.Closed, "request body not closed")
	test.Assert(t, recv.Closed, "result body not closed")
	test.Assert(t, bytes.Equal(data, out.Bytes()), "data ping-pong failed")
}

func TestRoundTripperCornerCases(t *testing.T) {
	limiter := NewStaticLimiter(42*1024, 42*1024)

	rt := limiter.Transport(roundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	}))

	res, err := rt.RoundTrip(&http.Request{})
	test.OK(t, err)
	test.Assert(t, res != nil, "round tripper returned no response")

	rt = limiter.Transport(roundTripper(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("error")
	}))

	_, err = rt.RoundTrip(&http.Request{})
	test.Assert(t, err != nil, "round tripper lost an error")
}
