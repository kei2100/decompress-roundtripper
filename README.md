decompress-roundtripper
=

RoundTripper is an implementation of the http.RoundTripper, that automatically decompresses the response body
according to the Content-Encoding header

Installation
==

```bash
$ go get github.com/kei2100/decompress-roundtripper
```

Example
==

```go
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
	req, _ := http.NewRequest("GET", svr.URL, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, _ := cli.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println(string(b))

	// Output: foobarbaz
}
```
