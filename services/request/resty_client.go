package request

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-resty/resty/v2"
	req "github.com/imroc/req/v3"
	"github.com/skycheung803/go-bypasser"
)

type RestyClient struct {
	client         *resty.Client
	reqClient      *req.Client
	ctx            context.Context
	baseURL        string
	defaultHeaders map[string]string
	proxyStr       string
	timeout        time.Duration
}

func (request *RestyClient) Resty() *resty.Client {
	return request.client
}

// NewClient 创建一个基于 go-bypasser(req/v3) 的客户端。
// 对外继续保留 Resty 风格接口，但底层请求不再走 resty.Transport。
func NewClient(baseURL string, proxyStr string, persistCookies bool, timeout int) *RestyClient {
	if timeout <= 0 {
		timeout = 60
	}
	timeoutDuration := time.Duration(timeout) * time.Second

	defaultHeaders := map[string]string{
		"accept":             "*/*",
		"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"connection":         "keep-alive",
		"pragma":             "no-cache",
		"priority":           "u=1,i",
		"sec-ch-ua":          "\"Chromium\";v=\"146\", \"Not-A.Brand\";v=\"24\", \"Google Chrome\";v=\"146\"",
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "\"macOS\"",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
	}

	stateClient := resty.New().
		SetTimeout(timeoutDuration).
		SetHeaders(defaultHeaders)
	if baseURL != "" {
		stateClient.SetBaseURL(baseURL)
	}

	var sharedJar http.CookieJar
	if persistCookies {
		jar, _ := cookiejar.New(nil)
		sharedJar = jar
		stateClient.SetCookieJar(sharedJar)
	}

	baseReqClient := mustNewReqClient(proxyStr, timeoutDuration, baseURL, defaultHeaders, sharedJar)

	return &RestyClient{
		client:         stateClient,
		reqClient:      baseReqClient,
		ctx:            context.Background(),
		baseURL:        baseURL,
		defaultHeaders: defaultHeaders,
		proxyStr:       proxyStr,
		timeout:        timeoutDuration,
	}
}

func (request *RestyClient) WithContext(ctx context.Context) *RestyClient {
	if ctx == nil {
		ctx = context.Background()
	}
	return &RestyClient{
		client:         request.client,
		reqClient:      request.reqClient,
		ctx:            ctx,
		baseURL:        request.baseURL,
		defaultHeaders: request.defaultHeaders,
		proxyStr:       request.proxyStr,
		timeout:        request.timeout,
	}
}

// SetPersistentHeader 设置持久化 Header。
// 除 Cookie 外，其余 Header 会同步到 req 客户端的 common headers。
func (request *RestyClient) SetPersistentHeader(key string, value string) {
	if request.defaultHeaders == nil {
		request.defaultHeaders = make(map[string]string)
	}
	lowerKey := strings.ToLower(key)
	request.defaultHeaders[lowerKey] = value
	if request.client != nil {
		request.client.SetHeader(key, value)
	}
	if request.reqClient != nil && lowerKey != "cookie" {
		request.reqClient.SetCommonHeader(key, value)
	}
}

func mustNewReqClient(proxyStr string, timeout time.Duration, baseURL string, defaultHeaders map[string]string, jar http.CookieJar) *req.Client {
	opts := []bypasser.BypasserOption{
		bypasser.WithInsecureSkipVerify(true),
	}
	if proxyStr != "" {
		opts = append(opts, bypasser.WithProxy(proxyStr))
	}

	bypass, err := bypasser.NewBypasser(opts...)
	if err != nil {
		panic(err)
	}

	rt, ok := bypass.Transport.(*bypasser.StandardRoundTripper)
	if !ok || rt.Client == nil {
		panic("go-bypasser did not return a StandardRoundTripper client")
	}

	client := rt.Client
	client.SetTimeout(timeout)
	client.SetRedirectPolicy(req.DefaultRedirectPolicy())
	if baseURL != "" {
		client.SetBaseURL(baseURL)
	}
	for k, v := range defaultHeaders {
		if strings.ToLower(k) == "cookie" {
			continue
		}
		client.SetCommonHeader(k, v)
	}
	if jar != nil {
		client.SetCookieJar(jar)
	}
	return client
}

func (request *RestyClient) newRequestClient(redirectCount int) *req.Client {
	client := request.reqClient.Clone()
	if request.baseURL != "" {
		client.SetBaseURL(request.baseURL)
	}
	client.SetTimeout(request.timeout)

	switch {
	case redirectCount == 0:
		client.SetRedirectPolicy(req.NoRedirectPolicy())
	case redirectCount > 0:
		client.SetRedirectPolicy(req.MaxRedirectPolicy(redirectCount))
	default:
		client.SetRedirectPolicy(req.DefaultRedirectPolicy())
	}

	return client
}

func (request *RestyClient) resolveRequestURL(path string) *url.URL {
	if path == "" {
		return nil
	}

	parsedURL, err := url.Parse(path)
	if err != nil {
		return nil
	}
	if parsedURL.IsAbs() {
		return parsedURL
	}
	if request.baseURL == "" {
		return parsedURL
	}

	baseURL, err := url.Parse(request.baseURL)
	if err != nil {
		return parsedURL
	}
	return baseURL.ResolveReference(parsedURL)
}

func cloneCookie(cookie *http.Cookie) *http.Cookie {
	if cookie == nil {
		return nil
	}
	copied := *cookie
	return &copied
}

func isSafeHTTPCookieValue(value string) bool {
	if value == "" {
		return true
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
		switch r {
		case '"', ';', '\\', ',':
			return false
		}
	}
	return true
}

func parseRawCookieHeader(raw string) []*http.Cookie {
	if raw == "" {
		return nil
	}
	var cookies []*http.Cookie
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}
	return cookies
}

func buildCookieHeader(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(parts, "; ")
}

// decodeCompressedBody 按响应头解压正文，兼容项目里依赖明文 HTML/JSON 的解析逻辑。
func decodeCompressedBody(body []byte, contentEncoding string) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch {
	case encoding == "", encoding == "identity":
		return body, nil
	case strings.Contains(encoding, "gzip"):
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case strings.Contains(encoding, "deflate"):
		reader := flate.NewReader(bytes.NewReader(body))
		defer reader.Close()
		return io.ReadAll(reader)
	case strings.Contains(encoding, "br"):
		reader := brotli.NewReader(bytes.NewReader(body))
		return io.ReadAll(reader)
	default:
		return body, nil
	}
}

func (request *RestyClient) prepareCookies(path string, requestCookies []*http.Cookie) ([]*http.Cookie, string) {
	cookieMap := make(map[string]*http.Cookie)
	order := make([]string, 0)
	rawCookieNames := make(map[string]struct{})

	appendCookie := func(cookie *http.Cookie) {
		if cookie == nil || cookie.Name == "" {
			return
		}
		if _, exists := cookieMap[cookie.Name]; !exists {
			order = append(order, cookie.Name)
		}
		cloned := cloneCookie(cookie)
		cookieMap[cookie.Name] = cloned
		if cloned != nil && !isSafeHTTPCookieValue(cloned.Value) {
			rawCookieNames[cloned.Name] = struct{}{}
		}
	}

	parsedURL := request.resolveRequestURL(path)
	if request.client != nil && request.client.GetClient() != nil && request.client.GetClient().Jar != nil && parsedURL != nil {
		for _, cookie := range request.client.GetClient().Jar.Cookies(parsedURL) {
			appendCookie(cookie)
		}
	}
	if request.client != nil {
		for _, cookie := range request.client.Cookies {
			appendCookie(cookie)
		}
	}
	for _, cookie := range requestCookies {
		appendCookie(cookie)
	}

	rawCookies := parseRawCookieHeader(request.defaultHeaders["cookie"])
	for _, cookie := range rawCookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		rawCookieNames[cookie.Name] = struct{}{}
		if _, exists := cookieMap[cookie.Name]; !exists {
			order = append(order, cookie.Name)
		}
		cookieMap[cookie.Name] = cookie
	}

	mergedCookies := make([]*http.Cookie, 0, len(order))
	for _, name := range order {
		if cookie := cookieMap[name]; cookie != nil {
			mergedCookies = append(mergedCookies, cloneCookie(cookie))
		}
	}

	if len(rawCookieNames) == 0 {
		return mergedCookies, ""
	}
	return mergedCookies, buildCookieHeader(mergedCookies)
}

func (request *RestyClient) buildReqRequest(client *req.Client, path string, headers map[string]string, cookies []*http.Cookie) *req.Request {
	r := client.R().SetContext(request.ctx)
	if len(headers) > 0 {
		r.SetHeaders(headers)
	}

	mergedCookies, rawCookieHeader := request.prepareCookies(path, cookies)
	if rawCookieHeader != "" {
		r.SetHeader("Cookie", rawCookieHeader)
	} else if len(mergedCookies) > 0 {
		r.SetCookies(mergedCookies...)
	}
	return r
}

func (request *RestyClient) adaptReqResponse(path string, method string, data any, headers map[string]string, cookies []*http.Cookie, resp *req.Response) (*resty.Response, error) {
	if resp == nil {
		return nil, nil
	}

	body, err := resp.ToBytes()
	if err != nil && resp.Response == nil {
		return nil, err
	}

	rawResponse := resp.Response
	if rawResponse != nil {
		decodedBody, decodeErr := decodeCompressedBody(body, rawResponse.Header.Get("Content-Encoding"))
		if decodeErr != nil {
			return nil, decodeErr
		}
		body = decodedBody
		rawResponse.Body = io.NopCloser(bytes.NewReader(body))
		rawResponse.Header.Del("Content-Encoding")
		rawResponse.ContentLength = int64(len(body))
	}

	restyReq := request.client.R()
	restyReq.Method = method
	restyReq.URL = path
	restyReq.Body = data
	restyReq.Header = make(http.Header)
	for k, v := range headers {
		restyReq.Header.Set(k, v)
	}
	mergedCookies, rawCookieHeader := request.prepareCookies(path, cookies)
	if rawCookieHeader != "" {
		restyReq.Header.Set("Cookie", rawCookieHeader)
	} else {
		restyReq.Cookies = mergedCookies
	}

	restyResp := &resty.Response{
		Request:     restyReq,
		RawResponse: rawResponse,
	}
	restyResp.SetBody(body)
	return restyResp, err
}

func (request *RestyClient) decodeResponse(resp *resty.Response, result any) error {
	if resp == nil || result == nil {
		return nil
	}
	body := resp.Body()
	if len(body) == 0 {
		return nil
	}
	ct := strings.ToLower(resp.Header().Get("Content-Type"))
	if strings.Contains(ct, "application/json") || json.Valid(body) {
		if err := json.Unmarshal(body, result); err != nil {
			return err
		}
	}
	return nil
}

func (request *RestyClient) execute(method string, path string, data any, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	client := request.newRequestClient(redirectCount)

	doRequest := func(extraHeaders map[string]string) (*resty.Response, error) {
		r := request.buildReqRequest(client, path, extraHeaders, cookies)
		if data != nil {
			r.SetBody(data)
		}

		var (
			resp *req.Response
			err  error
		)
		switch method {
		case http.MethodGet:
			resp, err = r.Get(path)
		case http.MethodPost:
			resp, err = r.Post(path)
		case http.MethodPut:
			resp, err = r.Put(path)
		case http.MethodPatch:
			resp, err = r.Patch(path)
		case http.MethodDelete:
			resp, err = r.Delete(path)
		case http.MethodHead:
			resp, err = r.Head(path)
		case http.MethodOptions:
			resp, err = r.Options(path)
		default:
			return nil, fmt.Errorf("unsupported method: %s", method)
		}

		restyResp, adaptErr := request.adaptReqResponse(path, method, data, extraHeaders, cookies, resp)
		if err != nil && errors.Is(err, http.ErrUseLastResponse) {
			err = nil
		}
		if adaptErr != nil && err == nil {
			err = adaptErr
		}
		return restyResp, err
	}

	resp, err := doRequest(headers)
	if err == nil && resp != nil && strings.Contains(strings.ToLower(resp.Header().Get("Content-Encoding")), "zstd") {
		err = fmt.Errorf("zstd body requires identity fallback")
	}
	if err == nil {
		if decodeErr := request.decodeResponse(resp, result); decodeErr != nil {
			return nil, decodeErr
		}
		return resp, nil
	}

	errStr := err.Error()
	if strings.Contains(errStr, "gzip") || strings.Contains(errStr, "magic number mismatch") || strings.Contains(errStr, "zstd") || strings.Contains(errStr, "brotli") || strings.Contains(errStr, "flate") {
		h2 := map[string]string{}
		for k, v := range headers {
			if strings.ToLower(k) != "accept-encoding" {
				h2[k] = v
			}
		}
		h2["Accept-Encoding"] = "identity"

		resp2, err2 := doRequest(h2)
		if err2 == nil {
			if decodeErr := request.decodeResponse(resp2, result); decodeErr != nil {
				return nil, decodeErr
			}
			return resp2, nil
		}
		if resp2 != nil {
			return resp2, err2
		}
	}

	if resp != nil {
		return resp, err
	}
	return nil, err
}

func (request *RestyClient) RestyGet(path string, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodGet, path, nil, result, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyPost(path string, data any, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodPost, path, data, result, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyPut(path string, data any, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodPut, path, data, result, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyPatch(path string, data any, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodPatch, path, data, result, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyDelete(path string, result any, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodDelete, path, nil, result, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyHead(path string, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodHead, path, nil, nil, headers, cookies, redirectCount)
}

func (request *RestyClient) RestyOptions(path string, headers map[string]string, cookies []*http.Cookie, redirectCount int) (*resty.Response, error) {
	return request.execute(http.MethodOptions, path, nil, nil, headers, cookies, redirectCount)
}
