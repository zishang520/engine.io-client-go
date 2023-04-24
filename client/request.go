package client

import (
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/zishang520/engine.io/types"
)

type Response struct {
	*http.Response

	BodyBuffer types.BufferInterface
}

type Options struct {
	Method          string
	Headers         map[string]string
	Compress        bool
	Timeout         time.Duration
	Body            io.Reader
	Jar             http.CookieJar
	TLSClientConfig *tls.Config
}

type Request struct {
	uri     string
	options *Options
}

// Request constructor
func NewRequest(uri string, opts *Options) (*Response, error) {
	r := &Request{}

	r.uri = uri
	r.options = opts

	return r.create()
}

func (r *Request) create() (res *Response, _ error) {
	_client := &http.Client{}
	if r.options.Jar != nil {
		_client.Jar = r.options.Jar
	}
	if r.options.TLSClientConfig != nil {
		_client.Transport = &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: r.options.TLSClientConfig,
		}
	}
	if r.options.Timeout > 0 {
		_client.Timeout = r.options.Timeout
	}
	request, err := http.NewRequest(strings.ToUpper(r.options.Method), r.uri, r.options.Body)
	if err != nil {
		return nil, err
	}
	if r.options.Headers != nil {
		for key, value := range r.options.Headers {
			request.Header.Set(key, value)
		}
	}
	if _, HasContentType := request.Header["Content-Type"]; r.options.Body != nil && !HasContentType {
		request.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	}
	request.Header.Set("Accept", "*/*")
	if r.options.Compress {
		request.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}

	response, err := _client.Do(request)
	if err != nil {
		return nil, err
	}

	res = &Response{Response: response}

	// apparently, Body can be nil in some cases
	if response.Body != nil {
		defer response.Body.Close()

		body := types.NewStringBuffer(nil)
		switch response.Header.Get("Content-Encoding") {
		case "gzip":
			gz, err := gzip.NewReader(response.Body)
			if err != nil {
				return nil, err
			}
			defer gz.Close()
			io.Copy(body, gz)
			response.Header.Del("Content-Encoding")
			response.Header.Del("Content-Length")
			response.ContentLength = -1
			response.Uncompressed = true
		case "deflate":
			fl := flate.NewReader(response.Body)
			defer fl.Close()
			io.Copy(body, fl)
			response.Header.Del("Content-Encoding")
			response.Header.Del("Content-Length")
			response.ContentLength = -1
			response.Uncompressed = true
		case "br":
			br := brotli.NewReader(response.Body)
			io.Copy(body, br)
			response.Header.Del("Content-Encoding")
			response.Header.Del("Content-Length")
			response.ContentLength = -1
			response.Uncompressed = true
		default:
			io.Copy(body, response.Body)
		}
		res.BodyBuffer = body
	} else {
		res.BodyBuffer = nil
	}
	return res, nil
}
