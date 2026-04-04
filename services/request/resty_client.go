package request

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/andybalholm/brotli"
	"github.com/go-resty/resty/v2"
	"github.com/skycheung803/go-bypasser"
)

type RestyClient struct {
	client *resty.Client
}

func (request *RestyClient) Resty() *resty.Client {
	return request.client
}

// NewClient 创建一个基于 uTLS 指纹与 HTTP/2 指纹的 Resty 客户端
// baseURL 不为空则设置默认 BaseURL；proxyStr 不为空则启用 HTTP 代理（仅 HTTP/1.1）
// persistCookies 启用持久化 Cookie；followRedirect 启用重定向跟随；timeout 设置超时时间（秒，0 或负数则默认 60 秒）
func NewClient(baseURL string, proxyStr string, persistCookies bool, timeout int) *RestyClient {
	rc := resty.New()

	if baseURL != "" {
		rc.SetBaseURL(baseURL)
	}

	if persistCookies {
		jar, _ := cookiejar.New(nil)
		rc.SetCookieJar(jar)
	}

	// 设置请求超时时间，如果传入 0 或负数则默认 60 秒
	if timeout <= 0 {
		timeout = 60
	}
	rc.SetTimeout(time.Duration(timeout) * time.Second)

	// 统一设置客户端默认请求头（调用级 headers 可覆盖），字段按字母顺序排列
	rc.SetHeader("accept", "*/*")
	rc.SetHeader("accept-language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	rc.SetHeader("connection", "keep-alive")
	rc.SetHeader("pragma", "no-cache")
	rc.SetHeader("priority", "u=1,i")
	rc.SetHeader("sec-ch-ua", "\"Chromium\";v=\"146\", \"Not-A.Brand\";v=\"24\", \"Google Chrome\";v=\"146\"")
	rc.SetHeader("sec-ch-ua-mobile", "?0")
	rc.SetHeader("sec-ch-ua-platform", "\"macOS\"")
	rc.SetHeader("sec-fetch-dest", "empty")
	rc.SetHeader("sec-fetch-mode", "cors")
	rc.SetHeader("sec-fetch-site", "same-origin")
	rc.SetHeader("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")

	// 初始化 go-bypasser 替代原有的 spoofed-round-tripper
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

	rc.SetTransport(bypass.Transport)

	return &RestyClient{client: rc}
}

// fillResponseBody 使用反射强制填充响应体
// 当 Resty 因为重定向策略错误而提前返回时，它可能不会读取 Body
// 此方法手动读取 RawResponse.Body 并回填到 resty.Response 的私有 body 字段中
func (request *RestyClient) fillResponseBody(resp *resty.Response) {
	if resp == nil || resp.RawResponse == nil {
		return
	}
	// 如果已经有 body 内容，则不处理
	if len(resp.Body()) > 0 {
		return
	}

	// 读取底层 Body
	bodyBytes, err := io.ReadAll(resp.RawResponse.Body)
	if err != nil {
		return
	}
	resp.RawResponse.Body.Close()
	// 重置 Body 以便后续可能得读取
	resp.RawResponse.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 使用反射设置私有字段 body
	v := reflect.ValueOf(resp).Elem()
	f := v.FieldByName("body")
	if f.IsValid() {
		// 必须使用 UnsafeAddr 获取未导出字段的地址
		rf := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
		rf.SetBytes(bodyBytes)
	}

	// 设置 size 字段
	s := v.FieldByName("size")
	if s.IsValid() {
		rs := reflect.NewAt(s.Type(), unsafe.Pointer(s.UnsafeAddr())).Elem()
		rs.SetInt(int64(len(bodyBytes)))
	}
}

// makeReq 构造带可选请求头的 resty.Request
// 功能：基于客户端创建请求对象，并在传入 headers 时进行设置
// 返回：带有请求头的请求对象
func (request *RestyClient) makeReq(headers map[string]string, cookies []*http.Cookie) *resty.Request {
	req := request.client.R()
	if len(headers) > 0 {
		req = req.SetHeaders(headers)
	}
	if len(cookies) > 0 {
		req = req.SetCookies(cookies)
	}
	return req
}

// doWithEncodingFallback 封装请求发送并在出现压缩相关错误时进行一次降级重试
// 逻辑：首次请求失败且错误包含 gzip/zstd/brotli/magic number mismatch 时，设置 accept-encoding 为 identity 重试一次
func (request *RestyClient) doWithEncodingFallback(headers map[string]string, cookies []*http.Cookie, allowRedirect bool, do func(*resty.Request) (*resty.Response, error)) (*resty.Response, error) {
	req := request.makeReq(headers, cookies)
	if allowRedirect {
		request.client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
	} else {
		// 使用 http.ErrUseLastResponse 确保 302 响应被返回且 Body 可读，而不是报错
		request.client.SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}))
	}
	resp, err := do(req)

	// 尝试补救响应体（特别是当重定向被禁用导致报错时）
	request.fillResponseBody(resp)

	if err == nil {
		return resp, nil
	}
	s := err.Error()
	if strings.Contains(s, "gzip: invalid header") || strings.Contains(s, "magic number mismatch") || strings.Contains(s, "zstd") || strings.Contains(s, "brotli") {
		h2 := map[string]string{}
		for k, v := range headers {
			if strings.ToLower(k) != "accept-encoding" {
				h2[k] = v
			}
		}
		h2["Accept-Encoding"] = "identity"
		req2 := request.makeReq(h2, cookies)
		resp2, err2 := do(req2)
		request.fillResponseBody(resp2)
		if err2 == nil {
			return resp2, nil
		}
	}
	return resp, err
}

// decodeResponse 处理响应解压与 JSON 解析
// 功能：自动识别 gzip 压缩并解压；在 result 非空时按 JSON 解析到 result
// 返回：解析错误（成功时为 nil）
func (request *RestyClient) decodeResponse(resp *resty.Response, result interface{}) error {
	if resp == nil {
		return nil
	}
	ct := strings.ToLower(resp.Header().Get("Content-Type"))
	ce := strings.ToLower(resp.Header().Get("Content-Encoding"))
	body := resp.Body()
	if strings.Contains(ce, "gzip") && len(body) > 0 {
		gr, gerr := gzip.NewReader(bytes.NewReader(body))
		if gerr == nil {
			defer gr.Close()
			if dec, derr := io.ReadAll(gr); derr == nil {
				body = dec
				resp.SetBody(body)
			}
		}
	} else if strings.Contains(ce, "deflate") && len(body) > 0 {
		// 处理 deflate 压缩
		dr := flate.NewReader(bytes.NewReader(body))
		defer dr.Close()
		if dec, derr := io.ReadAll(dr); derr == nil {
			body = dec
			resp.SetBody(body)
		}
	} else if strings.Contains(ce, "br") && len(body) > 0 {
		// 处理 brotli 压缩
		br := brotli.NewReader(bytes.NewReader(body))
		if dec, derr := io.ReadAll(br); derr == nil {
			body = dec
			resp.SetBody(body) // 将解压后的 body 写回 response
		}
	}
	if result != nil && (strings.Contains(ct, "application/json") || json.Valid(body)) {
		if err := json.Unmarshal(body, result); err != nil {
			return err
		}
	}
	return nil
}

// RestyGet 发送 GET 请求
func (request *RestyClient) RestyGet(path string, result interface{}, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.Get(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}

	if err := request.decodeResponse(resp, result); err != nil {
		return nil, err
	}

	return resp, err
}

// RestyPost 发送 POST 请求
func (request *RestyClient) RestyPost(path string, data any, result interface{}, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(data).Post(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}

	if err := request.decodeResponse(resp, result); err != nil {
		return nil, err
	}

	return resp, err
}

// RestyPut 发送 PUT 请求
// 功能：发送 PUT，支持请求级 headers 覆盖客户端默认，自动识别 gzip 并解析 JSON
// 返回：响应对象与错误信息
func (request *RestyClient) RestyPut(path string, data any, result interface{}, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(data).Put(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}

	if err := request.decodeResponse(resp, result); err != nil {
		return nil, err
	}

	return resp, err
}

// RestyPatch 发送 PATCH 请求
// 功能：发送 PATCH，支持请求级 headers 覆盖客户端默认，自动识别 gzip 并解析 JSON
// 返回：响应对象与错误信息
func (request *RestyClient) RestyPatch(path string, data any, result interface{}, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(data).Patch(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}

	if err := request.decodeResponse(resp, result); err != nil {
		return nil, err
	}

	return resp, err
}

// RestyDelete 发送 DELETE 请求
// 功能：发送 DELETE，支持请求级 headers 覆盖客户端默认，自动识别 gzip 并解析 JSON
// 返回：响应对象与错误信息
func (request *RestyClient) RestyDelete(path string, result interface{}, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}

	if err := request.decodeResponse(resp, result); err != nil {
		return nil, err
	}

	return resp, err
}

// RestyHead 发送 HEAD 请求
// 功能：发送 HEAD，支持请求级 headers 覆盖客户端默认；HEAD 通常无正文
// 返回：响应对象与错误信息
func (request *RestyClient) RestyHead(path string, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.Head(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}
	return resp, err
}

// RestyOptions 发送 OPTIONS 请求
// 功能：发送 OPTIONS，支持请求级 headers 覆盖客户端默认
// 返回：响应对象与错误信息
func (request *RestyClient) RestyOptions(path string, headers map[string]string, cookies []*http.Cookie, allowRedirect bool) (*resty.Response, error) {
	resp, err := request.doWithEncodingFallback(headers, cookies, allowRedirect, func(r *resty.Request) (*resty.Response, error) {
		return r.Options(path)
	})
	if resp == nil && err != nil {
		return nil, err
	}
	return resp, err
}
