// Package http provides tools for integrating HTTP request handling within Go-based web servers and clients with Starlark scripting capabilities.
// This enables dynamic inspection and modification of HTTP requests and responses through scripts, enhancing flexibility and control over processing.
//
// Migrated from: https://github.com/qri-io/starlib/tree/master/http with modifications.
package http

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	itn "github.com/1set/starlet/internal"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the expected name for this Module when used in starlark's load() function, eg: load('http', 'get')
const ModuleName = "http"

var (
	// UserAgent is the default user agent for http requests, override with a custom value before calling LoadModule.
	UserAgent = "Starlet-http-client/" + itn.StarletVersion
	// TimeoutSecond is the default timeout in seconds for http requests, override with a custom value before calling LoadModule.
	TimeoutSecond = 30.0
	// SkipInsecureVerify controls whether to skip TLS verification, override with a custom value before calling LoadModule.
	SkipInsecureVerify = false
	// DisableRedirect controls whether to follow redirects, override with a custom value before calling LoadModule.
	DisableRedirect = false
	// Client is the http client used to create the http module, override with a custom client before calling LoadModule.
	Client *http.Client
	// Guard is a global RequestGuard used in LoadModule, override with a custom implementation before calling LoadModule.
	Guard RequestGuard
	// ConfigLock is a global lock for settings, use it to ensure thread safety when setting.
	ConfigLock sync.RWMutex
)

// RequestGuard controls access to http by checking before making requests
// if Allowed returns an error the request will be denied
type RequestGuard interface {
	Allowed(thread *starlark.Thread, req *http.Request) (*http.Request, error)
}

// LoadModule creates an http Module
func LoadModule() (starlark.StringDict, error) {
	return NewModule().LoadModule()
}

// Module defines the actual HTTP module with methods for making requests.
type Module struct {
	cli *http.Client
	rg  RequestGuard
}

// NewModule creates a new http module with default settings.
func NewModule() *Module {
	m := &Module{}
	if Client != nil {
		m.cli = Client
	}
	if Guard != nil {
		m.rg = Guard
	}
	return m
}

// SetClient sets the http client for this module, useful for setting custom clients for testing or multiple loadings.
func (m *Module) SetClient(c *http.Client) {
	m.cli = c
}

// SetGuard sets the request guard for this module, useful for setting custom guards for testing or multiple loadings.
func (m *Module) SetGuard(g RequestGuard) {
	m.rg = g
}

// LoadModule creates an http Module.
func (m *Module) LoadModule() (starlark.StringDict, error) {
	return starlark.StringDict{
		ModuleName: m.Struct(),
	}, nil
}

// Struct returns this module's supported methods as a starlark Struct
func (m *Module) Struct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, m.StringDict())
}

var (
	supportedMethods = []string{"get", "put", "post", "postForm", "delete", "head", "patch", "options"}
)

// StringDict returns all module methods in a starlark.StringDict
func (m *Module) StringDict() starlark.StringDict {
	sd := make(starlark.StringDict, len(supportedMethods))
	for _, name := range supportedMethods {
		sd[name] = starlark.NewBuiltin(ModuleName+"."+name, m.reqMethod(name))
	}
	sd["call"] = starlark.NewBuiltin(ModuleName+".call", m.callMethod)
	sd["set_timeout"] = starlark.NewBuiltin(ModuleName+".set_timeout", setRequestTimeout)
	sd["get_timeout"] = starlark.NewBuiltin(ModuleName+".get_timeout", getRequestTimeout)
	return sd
}

// setRequestTimeout sets the timeout for http requests
func setRequestTimeout(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var timeout types.FloatOrInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "timeout", &timeout); err != nil {
		return nil, err
	}
	if timeout < 0 {
		return nil, fmt.Errorf("%s: timeout must be non-negative", b.Name())
	}
	// update the global TimeoutSecond variable which influences all future HTTP requests.
	ConfigLock.Lock()
	defer ConfigLock.Unlock()
	TimeoutSecond = float64(timeout)
	return starlark.None, nil
}

// getRequestTimeout returns the current timeout for http requests
func getRequestTimeout(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// check the arguments: no arguments
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	ConfigLock.RLock()
	defer ConfigLock.RUnlock()
	return starlark.Float(TimeoutSecond), nil
}

// callMethod is a general function for making http requests which takes the method name and arguments.
func (m *Module) callMethod(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// check the arguments, the first argument is the method name
	var fv types.StringOrBytes
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: missing method name", b.Name())
	}
	if err := fv.Unpack(args[0]); err != nil {
		return nil, fmt.Errorf("%s: for method name: %s", b.Name(), err)
	}
	// call the method with the given name
	method := strings.ToLower(fv.GoString())
	for _, name := range supportedMethods {
		if method == name {
			return m.reqMethod(name)(thread, b, args[1:], kwargs)
		}
	}
	return nil, fmt.Errorf("unsupported method: %s", method)
}

// reqMethod is a factory function for generating starlark builtin functions for different http request methods
func (m *Module) reqMethod(method string) func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			getDefaultDict = func() *types.NullableDict { return types.NewNullableDict(starlark.NewDict(0)) }
			urlv           starlark.String
			params         = getDefaultDict()                   // default None, expect Dict
			headers        = getDefaultDict()                   // default None, expect Dict
			auth           starlark.Tuple                       // default empty Tuple, expect Tuple of two strings
			body           = types.NewNullableStringOrBytes("") // default None, expect string
			jsonBody       starlark.Value                       // default None, expect JSON serializable object
			formBody       = getDefaultDict()                   // default None, expect Dict
			formEncoding   starlark.String                      // default empty string, expect string
			timeout        = types.FloatOrInt(TimeoutSecond)
			allowRedirect  = starlark.Bool(!DisableRedirect)
			verifySSL      = starlark.Bool(!SkipInsecureVerify)
		)

		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "url", &urlv, "params?", params, "headers", headers, "body", body, "json_body", &jsonBody, "form_body", formBody, "form_encoding", &formEncoding,
			"auth?", &auth, "timeout?", &timeout, "allow_redirects?", &allowRedirect, "verify?", &verifySSL); err != nil {
			return nil, err
		}

		rawURL, err := AsString(urlv)
		if err != nil {
			return nil, err
		}
		if err = setQueryParams(&rawURL, params.Value()); err != nil {
			return nil, err
		}

		// hack for postForm
		if method == "postForm" {
			method = "post"
			formEncoding = formEncodingURL
		}

		req, err := http.NewRequest(strings.ToUpper(method), rawURL, nil)
		if err != nil {
			return nil, err
		}
		if m.rg != nil {
			req, err = m.rg.Allowed(thread, req)
			if err != nil {
				return nil, err
			}
		}

		if err = setHeaders(req, headers.Value()); err != nil {
			return nil, err
		}
		if err = setAuth(req, auth); err != nil {
			return nil, err
		}
		if err = setBody(req, body, formBody.Value(), formEncoding, jsonBody); err != nil {
			return nil, err
		}

		cli := m.getHTTPClient(float64(timeout), bool(allowRedirect), bool(verifySSL))
		res, err := cli.Do(req)
		if err != nil {
			return nil, err
		}

		r := &Response{*res}
		return r.Struct(), nil
	}
}

func (m *Module) getHTTPClient(timeoutSec float64, allowRedirect, verifySSL bool) *http.Client {
	// return existing client if set
	if m.cli != nil {
		return m.cli
	}
	// set timeout to 30 seconds if it's negative
	if timeoutSec < 0 {
		timeoutSec = 30
	}
	cli := &http.Client{Timeout: time.Duration(timeoutSec * float64(time.Second))}
	// skip TLS verification if set
	if !verifySSL {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig.InsecureSkipVerify = true
		cli.Transport = tr
	}
	// disable redirects if set
	if !allowRedirect {
		cli.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return cli
}

// AsString unquotes a starlark string value
func AsString(x starlark.Value) (string, error) {
	return strconv.Unquote(x.String())
}

// Encodings for form data.
// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/POST
const (
	formEncodingMultipart = "multipart/form-data"
	formEncodingURL       = "application/x-www-form-urlencoded"
)

func setQueryParams(rawurl *string, params *starlark.Dict) error {
	keys := params.Keys()
	if len(keys) == 0 {
		return nil
	}

	u, err := url.Parse(*rawurl)
	if err != nil {
		return err
	}

	q := u.Query()
	for _, key := range keys {
		keystr, err := AsString(key)
		if err != nil {
			return err
		}

		val, _, err := params.Get(key)
		if err != nil {
			return err
		}
		if val.Type() != "string" {
			return fmt.Errorf("expected param value for key '%s' to be a string. got: '%s'", key, val.Type())
		}
		valstr, err := AsString(val)
		if err != nil {
			return err
		}

		q.Set(keystr, valstr)
	}

	u.RawQuery = q.Encode()
	*rawurl = u.String()
	return nil
}

func setAuth(req *http.Request, auth starlark.Tuple) error {
	if len(auth) == 0 {
		return nil
	} else if len(auth) == 2 {
		username, err := AsString(auth[0])
		if err != nil {
			return fmt.Errorf("parsing auth username string: %s", err.Error())
		}
		password, err := AsString(auth[1])
		if err != nil {
			return fmt.Errorf("parsing auth password string: %s", err.Error())
		}
		req.SetBasicAuth(username, password)
		return nil
	}
	return fmt.Errorf("expected two values for auth params tuple")
}

func setHeaders(req *http.Request, headers *starlark.Dict) error {
	var (
		keys    = headers.Keys()
		UAKey   = "User-Agent"
		isUASet = false
	)
	for _, key := range keys {
		keystr, err := AsString(key)
		if err != nil {
			return err
		}

		val, _, err := headers.Get(key)
		if err != nil {
			return err
		}
		if val.Type() != "string" {
			return fmt.Errorf("expected param value for key '%s' to be a string. got: '%s'", key, val.Type())
		}
		valstr, err := AsString(val)
		if err != nil {
			return err
		}

		req.Header.Add(keystr, valstr)
		if keystr == UAKey {
			isUASet = true
		}
	}

	if UserAgent != "" && !isUASet {
		req.Header.Set(UAKey, UserAgent)
	}
	return nil
}

func setBody(req *http.Request, body *types.NullableStringOrBytes, formData *starlark.Dict, formEncoding starlark.String, jsonData starlark.Value) error {
	if !body.IsNullOrEmpty() {
		uq := body.GoString()
		req.Body = ioutil.NopCloser(strings.NewReader(uq))
		// Specifying the Content-Length ensures that https://go.dev/src/net/http/transfer.go doesnt specify Transfer-Encoding: chunked which is not supported by some endpoints.
		// This is required when using ioutil.NopCloser method for the request body (see ShouldSendChunkedRequestBody() in the library mentioned above).
		req.ContentLength = int64(len(uq))

		return nil
	}

	if jsonData != nil && jsonData != starlark.None && jsonData.String() != "" {
		req.Header.Set("Content-Type", "application/json")
		data, err := dataconv.MarshalStarlarkJSON(jsonData, 0)
		if err != nil {
			return err
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(data)))
		req.ContentLength = int64(len(data))
	}

	if formData != nil && formData.Len() > 0 {
		type formFI struct{ name, content string }
		formVal := url.Values{}
		formFile := map[string]*formFI{}

		for _, key := range formData.Keys() {
			keystr := dataconv.StarString(key)
			val, _, err := formData.Get(key)
			if err != nil {
				return err
			}

			switch v := val.(type) {
			case starlark.String:
				// for key, value pairs
				formVal.Add(keystr, v.GoString())
			case starlark.Indexable:
				// for key, file paris
				if v.Len() < 2 {
					return fmt.Errorf("expected 2 values for key %s in form_body to be a tuple of (filename, content)", key)
				}
				// extract file name and content
				ffi := &formFI{}
				v0 := v.Index(0)
				v1 := v.Index(1)
				// check types
				if vs, ok := v0.(starlark.String); !ok {
					return fmt.Errorf("expected 1st value for key %s in form_body to be a string. got: %q", key, v0.Type())
				} else {
					ffi.name = vs.GoString()
				}
				if vs, ok := v1.(starlark.String); !ok {
					return fmt.Errorf("expected 2nd value for key %s in form_body to be a string. got: %q", key, v1.Type())
				} else {
					ffi.content = vs.GoString()
				}
				formFile[keystr] = ffi
			default:
				return fmt.Errorf("expected param value for key %s in form_body to be a string or tuple. got: %q", key, val.Type())
			}
		}

		// set form encoding implicitly if not set
		if formEncoding == "" {
			if len(formFile) > 0 {
				formEncoding = formEncodingMultipart
			} else {
				formEncoding = formEncodingURL
			}
		}

		var contentType string
		switch formEncoding {
		case formEncodingURL:
			contentType = formEncodingURL
			req.Body = ioutil.NopCloser(strings.NewReader(formVal.Encode()))
			req.ContentLength = int64(len(formVal.Encode()))

		case formEncodingMultipart:
			var b bytes.Buffer
			mw := multipart.NewWriter(&b)
			defer mw.Close()

			contentType = mw.FormDataContentType()

			for k, values := range formVal {
				for _, v := range values {
					w, err := mw.CreateFormField(k)
					if err != nil {
						return err
					}
					if _, err := w.Write([]byte(v)); err != nil {
						return err
					}
				}
			}

			for k, v := range formFile {
				w, err := mw.CreateFormFile(k, v.name)
				if err != nil {
					return err
				}
				if _, err := w.Write([]byte(v.content)); err != nil {
					return err
				}
			}

			req.Body = ioutil.NopCloser(&b)

		default:
			return fmt.Errorf("unknown form encoding: %s", formEncoding)
		}

		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", contentType)
		}
	}

	return nil
}

// Response represents an HTTP response, wrapping a Go http.Response with Starlark methods.
type Response struct {
	http.Response
}

// Struct turns a response into a *starlark.Struct
func (r *Response) Struct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"url":         starlark.String(r.Request.URL.String()),
		"status_code": starlark.MakeInt(r.StatusCode),
		"headers":     r.HeadersDict(),
		"encoding":    starlark.String(strings.Join(r.TransferEncoding, ",")),
		"body":        starlark.NewBuiltin("body", r.Text),
		"json":        starlark.NewBuiltin("json", r.JSON),
	})
}

// HeadersDict flops
func (r *Response) HeadersDict() *starlark.Dict {
	d := new(starlark.Dict)
	for key, vals := range r.Header {
		if err := d.SetKey(starlark.String(key), starlark.String(strings.Join(vals, ","))); err != nil {
			panic(err)
		}
	}
	return d
}

// Text returns the raw data as a string
func (r *Response) Text(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// reset reader to allow multiple calls
	_ = r.Body.Close()
	r.Body = ioutil.NopCloser(bytes.NewReader(data))

	// wraps as result
	return starlark.String(data), nil
}

// JSON attempts to parse the response body as JSON
func (r *Response) JSON(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// reset reader to allow multiple calls
	_ = r.Body.Close()
	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	// use internal marshaler to support starlark types, returns None on error
	sv, err := dataconv.UnmarshalStarlarkJSON(body)
	if err != nil {
		return starlark.None, nil
	}
	return sv, nil
}
