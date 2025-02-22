/*
*
*	Ddosify - Load testing tool for any web system.
*   Copyright (C) 2021  Ddosify (https://ddosify.com)
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU Affero General Public License as published
*   by the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU Affero General Public License for more details.
*
*   You should have received a copy of the GNU Affero General Public License
*   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*
 */

package requester

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.ddosify.com/ddosify/core/types"
	"golang.org/x/net/http2"
)

func TestInit(t *testing.T) {
	s := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Timeout: types.DefaultTimeout,
	}
	p, _ := url.Parse("https://127.0.0.1:80")
	ctx := context.TODO()

	h := &HttpRequester{}
	h.Init(ctx, s, p, false, nil)

	if !reflect.DeepEqual(h.packet, s) {
		t.Errorf("Expected %v, Found %v", s, h.packet)
	}
	if !reflect.DeepEqual(h.proxyAddr, p) {
		t.Errorf("Expected %v, Found %v", p, h.proxyAddr)
	}
	if !reflect.DeepEqual(h.ctx, ctx) {
		t.Errorf("Expected %v, Found %v", ctx, h.ctx)
	}
}

func TestInitClient(t *testing.T) {
	p, _ := url.Parse("https://127.0.0.1:80")
	ctx := context.TODO()

	// Basic Client
	s := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Timeout: types.DefaultTimeout,
	}
	expectedTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	expectedTr := &http.Transport{
		TLSClientConfig:   expectedTLS,
		Proxy:             http.ProxyURL(p),
		DisableKeepAlives: false,
	}
	expectedClient := &http.Client{
		Transport: expectedTr,
		Timeout:   time.Duration(types.DefaultTimeout) * time.Second,
	}

	// Client with custom data
	sWithCustomData := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Timeout: types.DefaultTimeout,
		Custom: map[string]interface{}{
			"disable-redirect":    true,
			"keep-alive":          false,
			"disable-compression": true,
			"hostname":            "dummy.com",
		},
	}
	expectedTLSCustomData := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "dummy.com",
	}
	expectedTrCustomData := &http.Transport{
		TLSClientConfig:    expectedTLSCustomData,
		Proxy:              http.ProxyURL(p),
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	expectedClientWithCustomData := &http.Client{
		Transport: expectedTrCustomData,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Duration(types.DefaultTimeout) * time.Second,
	}

	// H2 Client
	sHTTP2 := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Timeout: types.DefaultTimeout,
		Custom: map[string]interface{}{
			"h2": true,
		},
	}
	expectedTLSHTTP2 := &tls.Config{
		InsecureSkipVerify: true,
	}
	expectedTrHTTP2 := &http.Transport{
		TLSClientConfig:   expectedTLSHTTP2,
		Proxy:             http.ProxyURL(p),
		DisableKeepAlives: false,
	}
	http2.ConfigureTransport(expectedTrHTTP2)
	expectedClientHTTP2 := &http.Client{
		Transport: expectedTrHTTP2,
		Timeout:   time.Duration(types.DefaultTimeout) * time.Second,
	}

	// Sub Tests
	tests := []struct {
		name         string
		scenarioItem types.ScenarioStep
		proxy        *url.URL
		ctx          context.Context
		tls          *tls.Config
		transport    *http.Transport
		client       *http.Client
	}{
		{"Basic", s, p, ctx, expectedTLS, expectedTr, expectedClient},
		{"Custom", sWithCustomData, p, ctx, expectedTLSCustomData, expectedTrCustomData, expectedClientWithCustomData},
		{"HTTP2", sHTTP2, p, ctx, expectedTLSHTTP2, expectedTrHTTP2, expectedClientHTTP2},
	}

	for _, test := range tests {
		tf := func(t *testing.T) {
			h := &HttpRequester{}
			h.Init(test.ctx, test.scenarioItem, test.proxy, false, nil)

			transport := h.client.Transport.(*http.Transport)
			tls := transport.TLSClientConfig

			// TLS Assert (Also check HTTP2 vs HTTP)
			if !reflect.DeepEqual(test.tls, tls) {
				t.Errorf("\nTLS Expected %#v, \nFound %#v", test.tls, tls)
			}

			// Transport Assert
			if reflect.TypeOf(test.transport) != reflect.TypeOf(transport) {
				// Compare HTTP2 configured transport vs HTTP transport
				t.Errorf("Transport Type Expected %#v, Found %#v", test.transport, transport)
			}

			pFunc := transport.Proxy == nil
			expectedPFunc := test.transport.Proxy == nil
			if pFunc != expectedPFunc {
				t.Errorf("Proxy Expected %v, Found %v", expectedPFunc, pFunc)
			}
			if test.transport.DisableKeepAlives != transport.DisableKeepAlives {
				t.Errorf("DisableKeepAlives Expected %v, Found %v", test.transport.DisableKeepAlives, transport.DisableKeepAlives)
			}
			if test.transport.DisableCompression != transport.DisableCompression {
				t.Errorf("DisableCompression Expected %v, Found %v",
					test.transport.DisableCompression, transport.DisableCompression)
			}

			// Client Assert
			if test.client.Timeout != h.client.Timeout {
				t.Errorf("Timeout Expected %v, Found %v", test.client.Timeout, h.client.Timeout)
			}

			crFunc := h.client.CheckRedirect == nil
			expectedCRFunc := test.client.CheckRedirect == nil
			if expectedCRFunc != crFunc {
				t.Errorf("CheckRedirect Expected %v, Found %v", expectedCRFunc, crFunc)
			}

		}
		t.Run(test.name, tf)
	}
}

func TestInitRequest(t *testing.T) {
	p, _ := url.Parse("https://127.0.0.1:80")
	ctx := context.TODO()

	// Invalid request
	sInvalid := types.ScenarioStep{
		ID:      1,
		Method:  ":31:31:#",
		URL:     "https://test.com",
		Payload: "payloadtest",
	}

	// Basic request
	s := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Payload: "payloadtest",
	}
	expected, _ := http.NewRequest(s.Method, s.URL, bytes.NewBufferString(s.Payload))
	expected.Close = false
	expected.Header = make(http.Header)

	// Request with auth
	sWithAuth := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Payload: "payloadtest",
		Auth: types.Auth{
			Username: "test",
			Password: "123",
		},
	}
	expectedWithAuth, _ := http.NewRequest(sWithAuth.Method, sWithAuth.URL, bytes.NewBufferString(sWithAuth.Payload))
	expectedWithAuth.Close = false
	expectedWithAuth.Header = make(http.Header)
	expectedWithAuth.SetBasicAuth(sWithAuth.Auth.Username, sWithAuth.Auth.Password)

	// Request With Headers
	sWithHeaders := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.localhost",
		Payload: "payloadtest",
		Auth: types.Auth{
			Username: "test",
			Password: "123",
		},
		Headers: map[string][]string{
			"Header1":    []string{"Value1"},
			"Header2":    []string{"Value2"},
			"User-Agent": []string{"Firefox"},
			"Host":       []string{"test.com"},
			"set-cookie": []string{"sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Path=/; HttpOnly",
				"sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Path=/; Secure; HttpOnly",
				"sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Domain=www.example.com; Path=/; HttpOnly",
				"sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Domain=www.example.com; Path=/; Secure; HttpOnly"},
			"Set-Cookie": []string{"TS014e0f1c=018a033c988d7329a2fb249c3f62e7e2b69a4fe64911c1b9d417455acffdcaea1dbd515bdf5ba18419c725ed17b849de327f83034d; Path=/; Secure; HTTPOnly",
				"TS01825d59=018a033c988d7329a2fb249c3f62e7e2b69a4fe64911c1b9d417455acffdcaea1dbd515bdf5ba18419c725ed17b849de327f83034d; path=/; domain=www.mega-bank.nl; HTTPonly; Secure"},
		},
	}
	expectedWithHeaders, _ := http.NewRequest(sWithHeaders.Method,
		sWithHeaders.URL, bytes.NewBufferString(sWithHeaders.Payload))
	expectedWithHeaders.Close = false
	expectedWithHeaders.Header = make(http.Header)
	expectedWithHeaders.Header.Add("Header1", "Value1")
	expectedWithHeaders.Header.Add("Header2", "Value2")
	expectedWithHeaders.Header.Add("User-Agent", "Firefox")
	expectedWithHeaders.Header.Add("set-cookie", "sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Path=/; HttpOnly")
	expectedWithHeaders.Header.Add("set-cookie", "sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Path=/; Secure; HttpOnly")
	expectedWithHeaders.Header.Add("set-cookie", "sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Domain=www.example.com; Path=/; HttpOnly")
	expectedWithHeaders.Header.Add("set-cookie", "sessionid=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:10 GMT; Domain=www.example.com; Path=/; Secure; HttpOnly")
	expectedWithHeaders.Header.Add("Set-Cookie", "TS014e0f1c=018a033c988d7329a2fb249c3f62e7e2b69a4fe64911c1b9d417455acffdcaea1dbd515bdf5ba18419c725ed17b849de327f83034d; Path=/; Secure; HTTPOnly")
	expectedWithHeaders.Header.Add("Set-Cookie", "TS01825d59=018a033c988d7329a2fb249c3f62e7e2b69a4fe64911c1b9d417455acffdcaea1dbd515bdf5ba18419c725ed17b849de327f83034d; path=/; domain=www.example.com; HTTPonly; Secure")
	expectedWithHeaders.Host = "test.com"
	expectedWithHeaders.SetBasicAuth(sWithHeaders.Auth.Username, sWithHeaders.Auth.Password)

	// Request keep-alive condition
	sWithoutKeepAlive := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://test.com",
		Payload: "payloadtest",
		Auth: types.Auth{
			Username: "test",
			Password: "123",
		},
		Headers: map[string]string{
			"Header1": "Value1",
			"Header2": "Value2",
		},
		Custom: map[string]interface{}{
			"keep-alive": false,
		},
	}
	expectedWithoutKeepAlive, _ := http.NewRequest(sWithoutKeepAlive.Method,
		sWithoutKeepAlive.URL, bytes.NewBufferString(sWithoutKeepAlive.Payload))
	expectedWithoutKeepAlive.Close = true
	expectedWithoutKeepAlive.Header = make(http.Header)
	expectedWithoutKeepAlive.Header.Set("Header1", "Value1")
	expectedWithoutKeepAlive.Header.Set("Header2", "Value2")
	expectedWithoutKeepAlive.SetBasicAuth(sWithoutKeepAlive.Auth.Username, sWithoutKeepAlive.Auth.Password)

	// Sub Tests
	tests := []struct {
		name         string
		scenarioItem types.ScenarioStep
		shouldErr    bool
		request      *http.Request
	}{
		{"Invalid", sInvalid, true, nil},
		{"Basic", s, false, expected},
		{"WithAuth", sWithAuth, false, expectedWithAuth},
		{"WithHeaders", sWithHeaders, false, expectedWithHeaders},
		{"WithoutKeepAlive", sWithoutKeepAlive, false, expectedWithoutKeepAlive},
	}

	for _, test := range tests {
		tf := func(t *testing.T) {
			h := &HttpRequester{}
			err := h.Init(ctx, test.scenarioItem, p, false, nil)

			if test.shouldErr {
				if err == nil {
					t.Errorf("Should be errored")
				}
			} else {
				if err != nil {
					t.Errorf("Errored: %v", err)
				}

				// TODOcorr: we use tempValidUrl for correlation for now
				// if !reflect.DeepEqual(h.request.URL, test.request.URL) {
				// 	t.Errorf("URL Expected: %#v, Found: \n%#v", test.request.URL, h.request.URL)
				// }
				// if !reflect.DeepEqual(h.request.Host, test.request.Host) {
				// 	t.Errorf("Host Expected: %#v, Found: \n%#v", test.request.Host, h.request.Host)
				// }
				if !reflect.DeepEqual(h.request.Body, test.request.Body) {
					t.Errorf("Body Expected: %#v, Found: \n%#v", test.request.Body, h.request.Body)
				}
				if !reflect.DeepEqual(h.request.Header, test.request.Header) {
					t.Errorf("Header Expected: %#v, Found: \n%#v", test.request.Header, h.request.Header)
				}
				if !reflect.DeepEqual(h.request.Close, test.request.Close) {
					t.Errorf("Close Expected: %#v, Found: \n%#v", test.request.Close, h.request.Close)
				}
			}
		}
		t.Run(test.name, tf)
	}
}

func TestSendOnDebugModePopulatesDebugInfo(t *testing.T) {
	ctx := context.TODO()
	// Basic request
	payload := "reqbodypayload"
	s := types.ScenarioStep{
		ID:      1,
		Method:  http.MethodGet,
		URL:     "https://ddosify.com",
		Payload: payload,
		Headers: map[string]string{"X": "y"},
	}

	expectedUrl := "https://ddosify.com"
	expectedMethod := http.MethodGet
	expectedRequestHeaders := http.Header{"X": {"y"}}
	expectedRequestBody := []byte(payload)

	tf := func(t *testing.T) {
		h := &HttpRequester{}
		debug := true
		var proxy *url.URL
		_ = h.Init(ctx, s, proxy, debug, nil)
		envs := map[string]interface{}{}
		res := h.Send(envs)

		if expectedMethod != res.Method {
			t.Errorf("Method Expected %#v, Found: \n%#v", expectedMethod, res.Method)
		}
		if expectedUrl != res.Url {
			t.Errorf("Url Expected %#v, Found: \n%#v", expectedUrl, res.Url)
		}
		if !bytes.Equal(expectedRequestBody, res.ReqBody) {
			t.Errorf("RequestBody Expected %#v, Found: \n%#v", expectedRequestBody,
				res.ReqBody)
		}
		if !reflect.DeepEqual(expectedRequestHeaders, res.ReqHeaders) {
			t.Errorf("RequestHeaders Expected %#v, Found: \n%#v", expectedRequestHeaders,
				res.ReqHeaders)
		}

	}
	t.Run("populate-debug-info", tf)
}

func TestCaptureEnvShouldSetEmptyStringWhenReqFails(t *testing.T) {
	ctx := context.TODO()
	// Failed request
	envName := "ENV_NAME"
	headerKey := "key"
	s := types.ScenarioStep{
		ID:     1,
		Method: http.MethodGet,
		URL:    "https://ddosifyInvalid.com",
		EnvsToCapture: []types.EnvCaptureConf{{
			JsonPath: new(string),
			Xpath:    new(string),
			RegExp:   &types.RegexCaptureConf{},
			Name:     envName,
			From:     types.Header,
			Key:      &headerKey,
		}},
	}

	expectedExtractedEnvs := map[string]interface{}{
		envName: "",
	}

	// Sub Tests
	tests := []struct {
		name                  string
		scenarioStep          types.ScenarioStep
		expectedExtractedEnvs map[string]interface{}
	}{
		{"ExtractedEnvShouldBeEmptyStringWhenReqFailure", s, expectedExtractedEnvs},
	}

	for _, test := range tests {
		tf := func(t *testing.T) {
			h := &HttpRequester{}
			debug := true
			var proxy *url.URL
			_ = h.Init(ctx, test.scenarioStep, proxy, debug, nil)
			envs := map[string]interface{}{}
			res := h.Send(envs)

			if !reflect.DeepEqual(res.ExtractedEnvs, test.expectedExtractedEnvs) {
				t.Errorf("Extracted env should be set empty string on req failure")
			}
		}
		t.Run(test.name, tf)
	}
}

func TestAssertions(t *testing.T) {
	t.Parallel()

	// Test server
	firstReqHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Argentina", "Messi")
		w.WriteHeader(http.StatusForbidden)
	}

	rule1 := "equals(status_code,405)"
	rule2 := `equals(headers.Argentina,"Ronaldo")`
	pathFirst := "/json-body"
	mux := http.NewServeMux()
	mux.HandleFunc(pathFirst, firstReqHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	s := types.ScenarioStep{
		ID:         1,
		Method:     "GET",
		URL:        server.URL + pathFirst,
		Assertions: []string{rule1, rule2},
	}

	ctx := context.TODO()
	h := &HttpRequester{}
	h.Init(ctx, s, nil, false, nil)

	res := h.Send(map[string]interface{}{})

	if !strings.EqualFold(res.FailedAssertions[0].Rule, rule1) {
		t.Errorf("rule expected %s, got %s", rule1, res.FailedAssertions[0].Rule)
	}
	if reflect.DeepEqual(res.FailedAssertions[0].Received, 403) {
		t.Errorf("received expected %d, got %v", 403, res.FailedAssertions[0].Received)
	}
	if !strings.EqualFold(res.FailedAssertions[1].Rule, rule2) {
		t.Errorf("rule expected %s, got %s", rule1, res.FailedAssertions[1].Rule)
	}
	if reflect.DeepEqual(res.FailedAssertions[0].Received, "Ronaldo") {
		t.Errorf("received expected %s, got %v", "Ronaldo", res.FailedAssertions[1].Received)
	}
}
