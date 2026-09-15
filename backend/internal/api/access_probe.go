package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
)

type accessTestReq struct {
	UpstreamProxy    *string `json:"upstream_proxy"`
	Upstream         *string `json:"upstream"`
	FileUpstream     *string `json:"file_upstream"`
	MetadataUpstream *string `json:"metadata_upstream"`
}

type accessTestCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Detail  string `json:"detail"`
	MS      int64  `json:"ms"`
}

// postAccessTest 用表单草稿（未保存也可）探测上下游连通性。
func (s *Server) postAccessTest(w http.ResponseWriter, r *http.Request) {
	var body accessTestReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := s.cfg.Get()
	pypi := cfg.Platforms["pypi"]

	upstreamProxy := cfg.Server.UpstreamProxy
	if body.UpstreamProxy != nil {
		upstreamProxy = *body.UpstreamProxy
	}
	upstream := pypi.Upstream
	if body.Upstream != nil {
		upstream = *body.Upstream
	}
	fileUpstream := pypi.FileUpstream
	if body.FileUpstream != nil {
		fileUpstream = *body.FileUpstream
	}
	metaUpstream := pypi.MetadataUpstream
	if body.MetadataUpstream != nil {
		metaUpstream = *body.MetadataUpstream
	}

	client := newAccessProbeClient(strings.TrimSpace(upstreamProxy), 12*time.Second)
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	var checks []accessTestCheck
	var indexBody []byte
	indexURL := ""

	// 1) 索引上游
	{
		indexURL = pypihandler.SimpleIndexURL(upstream, "pip")
		t0 := time.Now()
		status, bodyBytes, err := probeGET(ctx, client, indexURL, 512<<10)
		ms := time.Since(t0).Milliseconds()
		if err != nil {
			checks = append(checks, accessTestCheck{
				Name: "index_upstream", OK: false, MS: ms,
				Detail: fmt.Sprintf("%s: %v", indexURL, err),
			})
		} else if status >= 400 {
			checks = append(checks, accessTestCheck{
				Name: "index_upstream", OK: false, MS: ms,
				Detail: fmt.Sprintf("%s → HTTP %d", indexURL, status),
			})
		} else {
			indexBody = bodyBytes
			refs := pypihandler.ExtractArtifactRefs(bodyBytes, indexURL)
			detail := fmt.Sprintf("%s → HTTP %d，解析到 %d 个发行文件", indexURL, status, len(refs))
			ok := len(refs) > 0 || strings.Contains(strings.ToLower(string(bodyBytes)), "pip")
			if !ok {
				detail += "（内容不像包索引）"
			}
			checks = append(checks, accessTestCheck{
				Name: "index_upstream", OK: ok, MS: ms, Detail: detail,
			})
		}
	}

	// 2) 包文件上游（优先用索引里的真实链接）
	{
		t0 := time.Now()
		fileURL, how := pickFileProbeURL(indexBody, indexURL, fileUpstream)
		if fileURL == "" {
			checks = append(checks, accessTestCheck{
				Name: "file_upstream", OK: false, Skipped: false, MS: 0,
				Detail: "无法构造探测 URL（请检查索引上游是否可用）",
			})
		} else {
			status, err := probeHEAD(ctx, client, fileURL)
			ms := time.Since(t0).Milliseconds()
			if err != nil {
				// 部分源不支持 HEAD，回退 Range GET
				status2, _, err2 := probeRangeGET(ctx, client, fileURL, 0, 0)
				ms = time.Since(t0).Milliseconds()
				if err2 != nil {
					checks = append(checks, accessTestCheck{
						Name: "file_upstream", OK: false, MS: ms,
						Detail: fmt.Sprintf("%s（%s）: HEAD %v; GET %v", fileURL, how, err, err2),
					})
				} else if status2 >= 400 && status2 != 206 {
					checks = append(checks, accessTestCheck{
						Name: "file_upstream", OK: false, MS: ms,
						Detail: fmt.Sprintf("%s（%s）→ HTTP %d", fileURL, how, status2),
					})
				} else {
					checks = append(checks, accessTestCheck{
						Name: "file_upstream", OK: true, MS: ms,
						Detail: fmt.Sprintf("%s（%s）→ HTTP %d", fileURL, how, status2),
					})
				}
			} else if status >= 400 {
				checks = append(checks, accessTestCheck{
					Name: "file_upstream", OK: false, MS: ms,
					Detail: fmt.Sprintf("%s（%s）→ HTTP %d", fileURL, how, status),
				})
			} else {
				checks = append(checks, accessTestCheck{
					Name: "file_upstream", OK: true, MS: ms,
					Detail: fmt.Sprintf("%s（%s）→ HTTP %d", fileURL, how, status),
				})
			}
		}
	}

	// 3) 元数据上游（PEP 658）
	{
		metaBase := strings.TrimSpace(metaUpstream)
		if metaBase == "" {
			checks = append(checks, accessTestCheck{
				Name: "metadata_upstream", OK: true, Skipped: true, MS: 0,
				Detail: "未配置，运行时回退到包文件上游",
			})
		} else {
			t0 := time.Now()
			metaURL := pickMetadataProbeURL(indexBody, indexURL, metaBase)
			if metaURL == "" {
				checks = append(checks, accessTestCheck{
					Name: "metadata_upstream", OK: false, MS: 0,
					Detail: "无法从索引构造 .metadata 探测 URL",
				})
			} else {
				status, err := probeHEAD(ctx, client, metaURL)
				if err != nil {
					status, _, err = probeRangeGET(ctx, client, metaURL, 0, 0)
				}
				ms := time.Since(t0).Milliseconds()
				switch {
				case err != nil:
					checks = append(checks, accessTestCheck{
						Name: "metadata_upstream", OK: false, MS: ms,
						Detail: fmt.Sprintf("%s: %v", metaURL, err),
					})
				case status == 404:
					checks = append(checks, accessTestCheck{
						Name: "metadata_upstream", OK: true, MS: ms,
						Detail: fmt.Sprintf("%s → HTTP 404（该源可能不提供 PEP 658，预取依赖闭包会受限）", metaURL),
					})
				case status >= 400:
					checks = append(checks, accessTestCheck{
						Name: "metadata_upstream", OK: false, MS: ms,
						Detail: fmt.Sprintf("%s → HTTP %d", metaURL, status),
					})
				default:
					checks = append(checks, accessTestCheck{
						Name: "metadata_upstream", OK: true, MS: ms,
						Detail: fmt.Sprintf("%s → HTTP %d", metaURL, status),
					})
				}
			}
		}
	}

	// 4) 出站代理（若配置）：随上游探测已覆盖；单独给一条说明
	if strings.TrimSpace(upstreamProxy) != "" {
		okUp := false
		for _, c := range checks {
			if c.Name == "index_upstream" && c.OK {
				okUp = true
				break
			}
		}
		checks = append(checks, accessTestCheck{
			Name: "upstream_proxy", OK: okUp, MS: 0,
			Detail: fmt.Sprintf("经由 %s（与索引探测共用）", strings.TrimSpace(upstreamProxy)),
		})
	}

	allOK := true
	for _, c := range checks {
		if !c.OK && !c.Skipped {
			allOK = false
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     allOK,
		"checks": checks,
	})
}

func newAccessProbeClient(proxyURL string, timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableCompression = true
	tr.TLSHandshakeTimeout = timeout
	tr.ResponseHeaderTimeout = timeout
	tr.IdleConnTimeout = timeout
	tr.DialContext = (&net.Dialer{Timeout: timeout}).DialContext
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		tr.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	} else if u, err := url.Parse(proxyURL); err == nil && u.Scheme != "" && u.Host != "" {
		tr.Proxy = http.ProxyURL(u)
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func probeGET(ctx context.Context, client *http.Client, rawURL string, maxBody int) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "mirrorhub-access-test/1.0")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBody)))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func probeHEAD(ctx context.Context, client *http.Client, rawURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mirrorhub-access-test/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return resp.StatusCode, nil
}

func probeRangeGET(ctx context.Context, client *http.Client, rawURL string, start, end int) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "mirrorhub-access-test/1.0")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func pickFileProbeURL(indexBody []byte, indexURL, fileUpstream string) (string, string) {
	refs := pypihandler.ExtractArtifactRefs(indexBody, indexURL)
	for _, ref := range refs {
		u := strings.TrimSpace(ref.URL)
		if u == "" {
			continue
		}
		low := strings.ToLower(u)
		if strings.Contains(low, ".whl") || strings.HasSuffix(low, ".tar.gz") || strings.HasSuffix(low, ".zip") {
			return u, "索引链接"
		}
	}
	if len(refs) > 0 && strings.TrimSpace(refs[0].URL) != "" {
		return refs[0].URL, "索引链接"
	}
	base := strings.TrimRight(strings.TrimSpace(fileUpstream), "/")
	if base == "" {
		return "", ""
	}
	return base + "/", "文件上游根路径"
}

func pickMetadataProbeURL(indexBody []byte, indexURL, metaUpstream string) string {
	refs := pypihandler.ExtractArtifactRefs(indexBody, indexURL)
	var wheel string
	for _, ref := range refs {
		u := strings.TrimSpace(ref.URL)
		if strings.Contains(strings.ToLower(u), ".whl") {
			wheel = u
			break
		}
	}
	if wheel == "" {
		return ""
	}
	_, _, clean := pypihandler.ParseLinkDigest(wheel)
	wheel = clean

	metaBase := strings.TrimRight(strings.TrimSpace(metaUpstream), "/")
	if i := strings.Index(wheel, "/packages/"); i >= 0 {
		return metaBase + wheel[i:] + ".metadata"
	}
	// 相对路径拼到 meta 根
	if u, err := url.Parse(wheel); err == nil && u.Path != "" {
		p := u.Path
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		if strings.Contains(p, "/packages/") {
			return metaBase + p[strings.Index(p, "/packages/"):] + ".metadata"
		}
		return metaBase + path.Dir(p) + "/" + path.Base(p) + ".metadata"
	}
	return wheel + ".metadata"
}
