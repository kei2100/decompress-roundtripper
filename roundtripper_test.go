package decompress_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/andybalholm/brotli"

	"github.com/kei2100/decompress-roundtripper"
)

type stubRoundTripper struct {
	response *http.Response
	err      error
}

func (s *stubRoundTripper) RoundTrip(_req *http.Request) (*http.Response, error) {
	return s.response, s.err
}

func TestRoundTripper_RoundTrip(t *testing.T) {
	tt := []struct {
		title                      string
		resp                       *http.Response
		wantBody                   string
		wantDecompressed           bool
		wantErrUnsupportedEncoding bool
	}{
		{
			title:            "non Content-Encoding",
			resp:             newResponse(t, []byte("foobarbaz"), ""),
			wantBody:         "foobarbaz",
			wantDecompressed: false,
		},
		{
			title:            "gzip",
			resp:             newResponse(t, gzipBytes(t, []byte("foobarbaz")), "gzip"),
			wantBody:         "foobarbaz",
			wantDecompressed: true,
		},
		{
			title:            "deflate",
			resp:             newResponse(t, deflateBytes(t, []byte("foobarbaz")), "deflate"),
			wantBody:         "foobarbaz",
			wantDecompressed: true,
		},
		{
			title:            "br",
			resp:             newResponse(t, brotliBytes(t, []byte("foobarbaz")), "br"),
			wantBody:         "foobarbaz",
			wantDecompressed: true,
		},
		{
			title:            "identity",
			resp:             newResponse(t, []byte("foobarbaz"), "identity"),
			wantBody:         "foobarbaz",
			wantDecompressed: false,
		},
		{
			title: "mixed",
			resp: newResponse(
				t,
				deflateBytes(t, gzipBytes(t, []byte("foobarbaz"))),
				"gzip, deflate"),
			wantBody:         "foobarbaz",
			wantDecompressed: true,
		},
		{
			title:                      "unsupported encoding",
			resp:                       newResponse(t, gzipBytes(t, []byte{1, 2, 3}), "unsupported, gzip"),
			wantErrUnsupportedEncoding: true,
		},
	}
	for i, te := range tt {
		t.Run(fmt.Sprintf("#%d %s", i, te.title), func(t *testing.T) {
			origContentLength64 := te.resp.ContentLength
			origContentEncoding := te.resp.Header.Get("Content-Encoding")
			origContentLength := te.resp.Header.Get("Content-Length")

			dr := decompress.RoundTripper{Wrap: &stubRoundTripper{response: te.resp}}
			req, _ := http.NewRequest("GET", "/", nil)
			resp, err := dr.RoundTrip(req)
			if te.wantErrUnsupportedEncoding {
				var wantErr *decompress.ErrUnsupportedEncoding
				if !errors.As(err, &wantErr) {
					t.Errorf("got %T %v, want ErrUnsupportedEncoding", err, err)
				} else {
					if got, want := wantErr.Encoding, origContentEncoding; got != want {
						t.Errorf("Original Content-Encoding got %v, want %v", got, want)
					}
					if got, want := copyAndReadAll(t, wantErr.Original), copyAndReadAll(t, te.resp); !bytes.Equal(got, want) {
						t.Errorf("Original body bytes got %v, want %v", got, want)
					}
				}
				return
			}
			if resp == nil {
				t.Error("got nil response")
				return
			}
			if got, want := string(copyAndReadAll(t, resp)), te.wantBody; got != want {
				t.Errorf("body got %v, want %v", got, want)
			}
			if te.wantDecompressed {
				if got, want := resp.Uncompressed, true; got != want {
					t.Errorf("Uncomporessed got %v, want %v", got, want)
				}
				if got, want := resp.ContentLength, int64(-1); got != want {
					t.Errorf("ContentLength got %v, want %v", got, want)
				}
				if got, want := resp.Header.Get("Content-Encoding"), ""; got != want {
					t.Errorf("Content-Encoding got %v, want %v", got, want)
				}
				if got, want := resp.Header.Get("Content-Length"), ""; got != want {
					t.Errorf("Content-Length got %v, want %v", got, want)
				}
			} else {
				if got, want := resp.Uncompressed, false; got != want {
					t.Errorf("Uncomporessed got %v, want %v", got, want)
				}
				if got, want := resp.ContentLength, origContentLength64; got != want {
					t.Errorf("ContentLength got %v, want %v", got, want)
				}
				if got, want := resp.Header.Get("Content-Encoding"), origContentEncoding; got != want {
					t.Errorf("Content-Encoding got %v, want %v", got, want)
				}
				if got, want := resp.Header.Get("Content-Length"), origContentLength; got != want {
					t.Errorf("Content-Length got %v, want %v", got, want)
				}
			}
		})
	}
}

func newResponse(t *testing.T, body []byte, contentEncoding string) *http.Response {
	t.Helper()
	h := http.Header{}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if contentEncoding != "" {
		h.Set("Content-Encoding", contentEncoding)
	}
	resp := &http.Response{
		Status:           http.StatusText(200),
		StatusCode:       200,
		Proto:            "HTTP/1.1",
		ProtoMajor:       1,
		ProtoMinor:       1,
		Header:           h,
		Body:             io.NopCloser(bytes.NewBuffer(body)),
		ContentLength:    int64(len(body)),
		TransferEncoding: nil,
		Close:            false,
		Uncompressed:     false,
		Trailer:          nil,
		Request:          nil,
		TLS:              nil,
	}
	return resp
}

func copyAndReadAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	cp := bytes.NewBuffer(b)
	resp.Body = io.NopCloser(cp)
	return cp.Bytes()
}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var dst bytes.Buffer
	w := gzip.NewWriter(&dst)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return dst.Bytes()
}

func deflateBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var dst bytes.Buffer
	w, err := flate.NewWriter(&dst, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return dst.Bytes()
}

func brotliBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var dst bytes.Buffer
	w := brotli.NewWriter(&dst)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return dst.Bytes()
}
