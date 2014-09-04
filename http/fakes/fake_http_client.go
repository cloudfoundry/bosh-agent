package fakes

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

type FakeClient struct {
	StatusCode             int
	RetriesBeforeChange    int
	NewStatusCode          int
	CallCount              int
	Error                  error
	returnNilResponse      bool
	keepFlippingStatusCode bool
	RequestBodies          []string
	Requests               []*http.Request
	responseMessage        string
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type stringReadCloser struct {
	reader io.Reader
	closed bool
}

func (s *stringReadCloser) Close() error {
	s.closed = true
	return nil
}

func (s *stringReadCloser) Read(p []byte) (n int, err error) {
	if s.closed {
		return 0, errors.New("already closed")
	}

	return s.reader.Read(p)
}

func NewFakeClient() (fakeClient *FakeClient) {
	fakeClient = &FakeClient{}
	return
}

func (c *FakeClient) SetMessage(message string) {
	c.responseMessage = message
}

func (c *FakeClient) SetNilResponse() {
	c.returnNilResponse = true
}

func (c *FakeClient) KeepFlippingStatusCode(tries int, statusCode int, newStatusCode int) {
	c.keepFlippingStatusCode = true
	c.RetriesBeforeChange = tries
	c.StatusCode = statusCode
	c.NewStatusCode = newStatusCode
}

func (c *FakeClient) FlipStatusCode(tries int, statusCode int, newStatusCode int) {
	c.keepFlippingStatusCode = false
	c.RetriesBeforeChange = tries
	c.StatusCode = statusCode
	c.NewStatusCode = newStatusCode
}

func (c *FakeClient) Do(req *http.Request) (resp *http.Response, err error) {
	c.CallCount++

	if !c.returnNilResponse {
		if c.RetriesBeforeChange != 0 && c.CallCount%c.RetriesBeforeChange == 0 {
			if c.keepFlippingStatusCode {
				c.flipStatus()
			} else if c.NewStatusCode != 0 {
				c.flipStatus()
				c.NewStatusCode = 0
			}
		}

		resp = &http.Response{Body: &stringReadCloser{bytes.NewBufferString(c.responseMessage), false}}
		resp.StatusCode = c.StatusCode
	}

	err = c.Error

	if req.Body != nil {
		buf := make([]byte, 1024)
		n, _ := req.Body.Read(buf)
		c.RequestBodies = append(c.RequestBodies, string(buf[0:n]))
	}
	c.Requests = append(c.Requests, req)

	return
}

func (c *FakeClient) flipStatus() {
	tmp := c.StatusCode
	c.StatusCode = c.NewStatusCode
	c.NewStatusCode = tmp
}
