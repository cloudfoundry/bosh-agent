package fakes

import (
	"net/http"
)

type FakeMonitRetryable struct {
	AttemptCalled      int
	AttemptIsRetryable bool
	AttemptErrors      []error

	responses []*http.Response
}

func NewFakeMonitRetryable() *FakeMonitRetryable {
	return &FakeMonitRetryable{
		responses:     []*http.Response{},
		AttemptErrors: []error{},
	}
}

func (r *FakeMonitRetryable) Attempt() (bool, error) {
	r.AttemptCalled += 1

	if len(r.AttemptErrors) > 0 {
		err := r.AttemptErrors[0]
		r.AttemptErrors = r.AttemptErrors[1:]
		return r.AttemptIsRetryable, err
	}

	return r.AttemptIsRetryable, nil
}

func (r *FakeMonitRetryable) Response() *http.Response {
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response
}

func (r *FakeMonitRetryable) SetResponse(response *http.Response) {
	r.responses = append(r.responses, response)
}

func (r *FakeMonitRetryable) SetNextResponseStatus(statusCode int, times int) {
	for i := 0; i < times; i++ {
		r.SetResponse(&http.Response{
			StatusCode: statusCode,
		})
	}
}
