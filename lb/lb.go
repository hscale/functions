package lb

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/golang/groupcache/consistenthash"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type ProxyDirector func(*consistenthash.Map) func(*http.Request)

func DefaultDirector(ch *consistenthash.Map) func(req *http.Request) {
	var i int64
	return func(req *http.Request) {
		buf := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(buf)
		buf.Reset()
		buf.WriteString(req.URL.Path)
		buf.WriteString("??")
		b := strconv.AppendInt(buf.Bytes(), atomic.AddInt64(&i, 1), 10)

		target := ch.Get(string(b))
		req.URL.Scheme = "http"
		req.URL.Host = target
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}
}

// ConsistentHashReverseProxy returns a new ReverseProxy that routes
// URLs to the scheme, host, and base path provided in by a consistent hash
// algorithm. If the target's path is "/base" and the incoming request was for
// "/dir", the target request will be for /base/dir.
// ConsistentHashReverseProxy does not rewrite the Host header.
func ConsistentHashReverseProxy(ctx context.Context, director ProxyDirector, nodes []string) *httputil.ReverseProxy {
	ch := consistenthash.New(len(nodes), nil)
	ch.Add(nodes...)
	return &httputil.ReverseProxy{
		Director:  director(ch),
		Transport: NewRoundTripper(ctx, nodes),
	}
}
