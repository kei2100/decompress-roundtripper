package decompress

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// RoundTripper is an implementation of the http.RoundTripper, that automatically decompresses the response body
// according to the Content-Encoding header
type RoundTripper struct {
	// Wrap is the actual RoundTripper. If Wrap is nil, http.DefaultTransport will be used
	Wrap http.RoundTripper
}

// RoundTrip implements the RoundTrip method of the http.RoundTripper.
// If the response body is compressed, decompress it according to the Content-Encoding header before returning it.
// Supported Content-Encoding is:
//   - gzip
//   - deflate
//   - br
//   - identity
// If an unsupported value is set, ErrUnsupportedEncoding will be returned. You can retrieve the original http.Response from ErrUnsupportedEncoding.
func (r *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := r.Wrap
	if w == nil {
		w = http.DefaultTransport
	}
	res, err := w.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	ce := res.Header.Get("Content-Encoding")
	if len(ce) == 0 {
		return res, nil
	}
	// decompress
	// e.g. `Content-Encoding: deflate, gzip` => decompress `gzip` > `deflate`
	var decompressed bool
	encodings := strings.Split(ce, ",")
	body := res.Body
	for i := len(encodings) - 1; i >= 0; i-- {
		encoding := strings.TrimSpace(encodings[i])
		switch encoding {
		case "gzip":
			decompressed = true
			r, err := gzip.NewReader(body)
			if err != nil {
				return nil, fmt.Errorf("decompress: create gzip reader: %w", err)
			}
			body = &cascadeReadCloser{readFrom: r, cascade: body}
		case "deflate":
			decompressed = true
			r := flate.NewReader(body)
			body = &cascadeReadCloser{readFrom: r, cascade: body}
		case "br":
			decompressed = true
			r := brotli.NewReader(body)
			body = &cascadeReadCloser{readFrom: io.NopCloser(r), cascade: body}
		case "identity", "":
			// nop
		default:
			return nil, &ErrUnsupportedEncoding{Original: res, Encoding: encoding}
		}
	}
	if !decompressed {
		return res, nil
	}
	res.Body = body
	// Refs https://github.com/golang/go/blob/0914646ab91a3157666d845d74d8d9a4a2831e1e/src/net/http/response.go#L89-L96
	// > Uncompressed reports whether the response was sent compressed but
	// > was decompressed by the http package. When true, reading from
	// > Body yields the uncompressed content instead of the compressed
	// > content actually set from the server, ContentLength is set to -1,
	// > and the "Content-Length" and "Content-Encoding" fields are deleted
	// > from the responseHeader. To get the original response from
	// > the server, set Transport.DisableCompression to true.
	res.Uncompressed = true
	res.ContentLength = -1
	res.Header.Del("Content-Encoding")
	res.Header.Del("Content-Length")
	return res, nil
}

// ErrUnsupportedEncoding represents unsupported encoding error
type ErrUnsupportedEncoding struct {
	// original http response
	Original *http.Response
	Encoding string
}

// Error implements the error interface
func (e *ErrUnsupportedEncoding) Error() string {
	return fmt.Sprintf("decompress: unsuported content encoding `%s`", e.Encoding)
}

type cascadeReadCloser struct {
	readFrom io.ReadCloser
	cascade  io.Closer
}

func (c *cascadeReadCloser) Read(p []byte) (int, error) {
	return c.readFrom.Read(p)
}

func (c *cascadeReadCloser) Close() error {
	rerr := c.readFrom.Close()
	cerr := c.cascade.Close()
	if rerr != nil && cerr != nil {
		return fmt.Errorf("%s: %s", rerr.Error(), cerr.Error())
	}
	if rerr != nil {
		return rerr
	}
	if cerr != nil {
		return cerr
	}
	return nil
}
