package support

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
)

type Response struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]any
	List   []any
}

func (r Response) String(key string) string {
	value, _ := r.Body[key].(string)
	return value
}

type Client struct {
	tb      testing.TB
	baseURL string
	http    *http.Client
	jar     *cookiejar.Jar
}

func NewClient(tb testing.TB, baseURL string) *Client {
	tb.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		tb.Fatal(err)
	}
	return &Client{tb: tb, baseURL: baseURL, jar: jar, http: &http.Client{Jar: jar}}
}

type RequestOptions struct {
	JSON        any
	RawBody     string
	ContentType string
	Form        *bytes.Buffer
	FormType    string
	Headers     map[string]string
}

func (c *Client) Do(method, path string, options RequestOptions) Response {
	c.tb.Helper()

	var body io.Reader
	contentType := options.ContentType
	switch {
	case options.JSON != nil:
		encoded, err := json.Marshal(options.JSON)
		if err != nil {
			c.tb.Fatal(err)
		}
		body = bytes.NewReader(encoded)
		contentType = "application/json"
	case options.RawBody != "":
		body = strings.NewReader(options.RawBody)
	case options.Form != nil:
		body = options.Form
		contentType = options.FormType
	}

	request, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		c.tb.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for key, value := range options.Headers {
		request.Header.Set(key, value)
	}

	response, err := c.http.Do(request)
	if err != nil {
		c.tb.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		c.tb.Fatal(err)
	}
	result := Response{Status: response.StatusCode, Header: response.Header, Raw: raw}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		result.Body = object
	}
	var list []any
	if json.Unmarshal(raw, &list) == nil {
		result.List = list
	}
	return result
}

func (c *Client) Get(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodGet, path, firstOptions(options))
}

func (c *Client) Post(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodPost, path, firstOptions(options))
}

func (c *Client) Patch(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodPatch, path, firstOptions(options))
}

func (c *Client) Delete(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodDelete, path, firstOptions(options))
}

func (c *Client) HasCookie(name string) bool {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		c.tb.Fatal(err)
	}
	for _, cookie := range c.jar.Cookies(parsed) {
		if cookie.Name == name {
			return true
		}
	}
	return false
}

func firstOptions(options []RequestOptions) RequestOptions {
	if len(options) == 0 {
		return RequestOptions{}
	}
	return options[0]
}

type ImageFormOptions struct {
	FieldName string
	Filename  string
	Type      string
	Size      int
}

func CreateImageForm(options ImageFormOptions) RequestOptions {
	if options.FieldName == "" {
		options.FieldName = "avatar"
	}
	if options.Filename == "" {
		options.Filename = "avatar.png"
	}
	if options.Type == "" {
		options.Type = "image/png"
	}
	if options.Size == 0 {
		options.Size = 128
	}

	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="`+options.FieldName+`"; filename="`+options.Filename+`"`)
	header.Set("Content-Type", options.Type)
	part, err := writer.CreatePart(header)
	if err != nil {
		panic(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte{1}, options.Size)); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return RequestOptions{Form: buffer, FormType: writer.FormDataContentType()}
}

func EmptyMultipartForm() RequestOptions {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return RequestOptions{Form: buffer, FormType: writer.FormDataContentType()}
}
