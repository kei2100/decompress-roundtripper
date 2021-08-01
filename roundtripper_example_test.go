package decompress_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/kei2100/decompress-roundtripper"
)

func ExampleRoundTripper() {
	cli := http.Client{
		// decompress.RoundTripper is automatically decompresses the response body according to the Content-Encoding header
		Transport: &decompress.RoundTripper{},
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		b := gzipBytes([]byte("foobarbaz"))
		w.Write(b)
	}))
	resp, _ := cli.Get(svr.URL)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println(string(b))

	// Output: foobarbaz
}
