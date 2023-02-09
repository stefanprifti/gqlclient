package gqlclient_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stefanprifti/gqlclient"
)

type Language struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Native string `json:"native"`
}

type Country struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Native    string     `json:"native"`
	Phone     string     `json:"phone"`
	Capital   string     `json:"capital"`
	Languages []Language `json:"languages"`
}

func TestQuery(t *testing.T) {
	type request struct {
		query     string
		variables map[string]interface{}
	}

	type response struct {
		Body   io.ReadCloser
		Err    error
		Status int
	}

	cases := []struct {
		name          string
		request       request
		response      response
		gqlResp       string
		expected      Country
		expectedErr   error
		tokenProvider gqlclient.TokenProvider
	}{
		{
			name: "success",
			request: request{
				query: `
					query Country($code: ID!) {
						country(code: $code) {
							code
							name
							native
							phone
							capital
							languages {
								code
								name
								native
							}
						}
					}`,
				variables: map[string]interface{}{"code": "AL"},
			},
			response: response{
				Body: io.NopCloser(strings.NewReader(`
					{
						"data": {
							"country": {
							"code": "AL",
							"name": "Albania",
							"native": "Shqipëria",
							"phone": "355",
							"capital": "Tirana",
							"languages": [
								{
								"code": "sq",
								"name": "Albanian",
								"native": "Shqip"
								}
							]
							}
						}
					}`)),
				Status: http.StatusOK,
			},
			expected: Country{
				Code:    "AL",
				Name:    "Albania",
				Native:  "Shqipëria",
				Phone:   "355",
				Capital: "Tirana",
				Languages: []Language{
					{
						Code:   "sq",
						Name:   "Albanian",
						Native: "Shqip",
					},
				},
			},
		},
		{
			name: "success with token provider",
			request: request{
				query: `
					query Country($code: ID!) {
						country(code: $code) {
							code
							name
							native
							phone
							capital
							languages {
								code
								name
								native
							}
						}
					}`,
				variables: map[string]interface{}{"code": "AL"},
			},
			response: response{
				Body: io.NopCloser(strings.NewReader(`
					{
						"data": {
							"country": {
							"code": "AL",
							"name": "Albania",
							"native": "Shqipëria",
							"phone": "355",
							"capital": "Tirana",
							"languages": [
								{
								"code": "sq",
								"name": "Albanian",
								"native": "Shqip"
								}
							]
							}
						}
					}`)),
				Status: http.StatusOK,
			},
			expected: Country{
				Code:    "AL",
				Name:    "Albania",
				Native:  "Shqipëria",
				Phone:   "355",
				Capital: "Tirana",
				Languages: []Language{
					{
						Code:   "sq",
						Name:   "Albanian",
						Native: "Shqip",
					},
				},
			},
			tokenProvider: &mockTokenProvider{
				getTokenFunc: func() (string, error) {
					return "token", nil
				},
			},
		},
		{
			name: "maximum token refresh attempts reached",
			request: request{
				query: `
					query Country($code: ID!) {
						country(code: $code) {
							code
							name
							native
							phone
							capital
							languages {
								code
								name
								native
							}
						}
					}`,
				variables: map[string]interface{}{"code": "AL"},
			},
			response: response{
				Body:   io.NopCloser(strings.NewReader(``)),
				Status: http.StatusUnauthorized,
			},
			tokenProvider: &mockTokenProvider{
				getTokenFunc: func() (string, error) {
					return "", errors.New("error")
				},
			},
			expectedErr: errors.New("failed to retry, max retry count reached"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := struct {
				Country Country `json:"country"`
			}{}

			client := gqlclient.New(gqlclient.Options{
				Endpoint: "/query",
				HTTPClient: &http.Client{
					Transport: &mockGQLRoundTripper{
						roundTripFunc: func(req *http.Request) (*http.Response, error) {
							return &http.Response{
								StatusCode: tc.response.Status,
								Body:       tc.response.Body,
							}, tc.response.Err
						},
					},
				},
				TokenProvider: tc.tokenProvider,
			})
			err := client.Query(context.Background(), tc.request.query, tc.request.variables, &resp)

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("expected error %v, got %v", tc.expectedErr, err)
				}
				return
			}

			if !reflect.DeepEqual(resp.Country, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, resp.Country)
			}
		})
	}
}

type mockGQLRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockGQLRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path != "/query" {
		return nil, fmt.Errorf("expected path to be /query, got %s", req.URL.Path)
	}

	return m.roundTripFunc(req)
}

type mockTokenProvider struct {
	getTokenFunc func() (string, error)
}

func (m *mockTokenProvider) Token() (string, error) {
	return m.getTokenFunc()
}
