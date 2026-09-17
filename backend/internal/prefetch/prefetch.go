package prefetch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/livehl/mirrorhub/internal/cache"
	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/downloader"
	pypihandler "github.com/livehl/mirrorhub/internal/handlers/pypi"
	"github.com/livehl/mirrorhub/internal/scheduler"
)

type Service struct {
	cfg   *config.Manager
	dl    *downloader.Engine
	sched *scheduler.Scheduler
	log   *zap.Logger

	mu     sync.Mutex
	manual []string
	cancel map[string]context.CancelFunc
	root   context.Context
}

func New(cfg *config.Manager, dl *downloader.Engine, sched *scheduler.Scheduler, log *zap.Logger) *Service {
	return &Service{
		cfg:    cfg,
		dl:     dl,
		sched:  sched,
		log:    log,
		cancel: map[string]context.CancelFunc{},
		root:   context.Background(),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.root = ctx
	go s.loop(ctx)
}

// Enqueue 接受包文件 URL，或 PEP 508 风格规格（如 requests>=2.0、<3）
func (s *Service) Enqueue(items []string) {
	s.mu.Lock()
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		s.manual = append(s.manual, it)
	}
	s.mu.Unlock()
	go s.drainManual(s.root)
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.drainManual(ctx)
		}
	}
}

func (s *Service) drainManual(ctx context.Context) {
	s.mu.Lock()
	items := append([]string{}, s.manual...)
	s.manual = nil
	s.mu.Unlock()
	for _, item := range items {
		urls, err := s.expandItem(ctx, item)
		if err != nil {
			s.failResolve(item, err)
			continue
		}
		if len(urls) == 0 {
			s.failResolve(item, fmt.Errorf("未找到匹配文件"))
			continue
		}
		s.log.Info("prefetch resolved", zap.String("item", item), zap.Int("files", len(urls)))
		for _, u := range urls {
			s.startOne(ctx, u)
		}
	}
}

func (s *Service) failResolve(item string, err error) {
	taskID := s.sched.Begin("pypi", item, scheduler.PriorityPrefetch, false, false)
	s.sched.MarkRunning(taskID)
	s.sched.End(taskID, err)
	s.log.Warn("prefetch resolve failed", zap.String("item", item), zap.Error(err))
}

func (s *Service) expandItem(ctx context.Context, item string) ([]string, error) {
	if strings.HasPrefix(item, "http://") || strings.HasPrefix(item, "https://") {
		return []string{item}, nil
	}
	cfg := s.cfg.Get()
	pf := cfg.Scheduler.Prefetch
	plats := pf.TargetPlatformList()
	env := pypihandler.TargetEnv{Python: pf.TargetPythonVersions(), Platforms: plats}

	type node struct {
		spec  string
		depth int
	}
	queue := []node{{spec: item, depth: 0}}
	seenPkg := map[string]struct{}{}
	var allURLs []string
	urlSeen := map[string]struct{}{}

	for len(queue) > 0 {
		if len(seenPkg) >= pf.MaxPackages {
			s.log.Warn("prefetch closure hit max packages", zap.Int("max", pf.MaxPackages))
			break
		}
		cur := queue[0]
		queue = queue[1:]
		req, err := pypihandler.ParseRequirement(cur.spec)
		if err != nil {
			if cur.depth == 0 {
				return nil, err
			}
			continue
		}
		name := pypihandler.NormalizeName(req.Name)
		if !pypihandler.IsPlausiblePackageName(name) {
			if cur.depth == 0 {
				return nil, fmt.Errorf("无效或非包名: %s", req.Raw)
			}
			continue
		}
		if _, ok := seenPkg[name]; ok {
			continue
		}
		seenPkg[name] = struct{}{}

		files, metaURL, err := s.resolvePackageFiles(ctx, req, pf)
		if err != nil {
			if cur.depth == 0 {
				return nil, err
			}
			s.log.Warn("prefetch dep resolve skip", zap.String("spec", cur.spec), zap.Error(err))
			continue
		}
		for _, u := range files {
			if _, ok := urlSeen[u]; ok {
				continue
			}
			urlSeen[u] = struct{}{}
			allURLs = append(allURLs, u)
		}

		// 依赖深度不设上限；仅 MaxPackages 限制闭包规模。无 metadata 则无法展开依赖。
		if metaURL == "" {
			s.log.Warn("prefetch skip deps: no metadata URL",
				zap.String("spec", cur.spec),
				zap.Int("files", len(files)),
				zap.Int("depth", cur.depth),
			)
			continue
		}
		deps, err := s.fetchRequires(ctx, metaURL, cfg)
		if err != nil {
			s.log.Warn("prefetch metadata skip", zap.String("url", metaURL), zap.Error(err))
			continue
		}
		for _, dep := range deps {
			reqPart, marker := pypihandler.SplitReqMarker(dep)
			if !pypihandler.EvalMarker(marker, env) {
				continue
			}
			if _, err := pypihandler.ParseRequirement(reqPart); err != nil {
				continue
			}
			queue = append(queue, node{spec: reqPart, depth: cur.depth + 1})
		}
	}
	return allURLs, nil
}

func (s *Service) resolvePackageFiles(ctx context.Context, req *pypihandler.Requirement, pf config.PrefetchConfig) (files []string, metaURL string, err error) {
	cfg := s.cfg.Get()
	pypi := cfg.Platforms["pypi"]
	indexURL := pypihandler.SimpleIndexURL(pypi.Upstream, req.Name)
	body, _, err := s.dl.FetchSimpleIndex(ctx, indexURL, "pypi", cfg.Cache.IndexTTLSeconds, true)
	if err != nil {
		return nil, "", err
	}
	refs := pypihandler.ExtractArtifactRefs(body, indexURL)
	hrefs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.SHA256 != "" {
			downloader.RememberDigest(ref.URL, ref.SHA256)
		}
		hrefs = append(hrefs, ref.URL)
	}
	ver, matched, err := pypihandler.SelectFiles(hrefs, req)
	if err != nil {
		return nil, "", fmt.Errorf("%s（索引 %s）: %w", req.Raw, indexURL, err)
	}
	tags := mergeWheelTags(pf)
	plats := pf.TargetPlatformList()
	filtered := pypihandler.FilterArtifacts(matched, pf.ArtifactMode, tags, plats)
	if len(filtered) == 0 {
		if len(matched) == 0 {
			return nil, "", fmt.Errorf("未找到匹配文件")
		}
		return nil, "", fmt.Errorf(
			"未找到匹配文件（版本 %s 有 %d 个发行文件，均不符合预取策略 mode=%s python=%v tags=%v platforms=%v；可在「平台与上游」调整目标 Python/平台、补充额外 wheel 标签，或改用 artifact_mode=all）",
			ver, len(matched), pf.ArtifactMode, pf.TargetPythonVersions(), tags, plats,
		)
	}
	s.log.Info("prefetch selected version",
		zap.String("spec", req.Raw),
		zap.String("version", ver),
		zap.Int("files", len(filtered)),
		zap.String("mode", pf.ArtifactMode),
		zap.Strings("python", pf.TargetPythonVersions()),
		zap.Strings("tags", tags),
		zap.Strings("platforms", plats),
	)
	metaURL = pickMetadataURL(filtered)
	return filtered, metaURL, nil
}

func mergeWheelTags(pf config.PrefetchConfig) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(t string) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	// 使用明确的版本列表生成 cp 标签（支持多版本输入）
	for _, t := range pypihandler.CPTagsFromVersions(pf.TargetPythonVersions()) {
		add(t)
	}
	for _, t := range pf.ExtraWheelTags {
		add(t)
	}
	return out
}

func pickMetadataURL(files []string) string {
	var wheel, anyFile string
	for _, u := range files {
		clean := stripURLNoise(u)
		fn := strings.ToLower(path.Base(clean))
		if strings.HasSuffix(fn, ".metadata") {
			return clean
		}
		if anyFile == "" {
			anyFile = clean
		}
		if strings.HasSuffix(fn, ".whl") {
			if strings.Contains(fn, "-none-any") {
				return clean + ".metadata"
			}
			if wheel == "" {
				wheel = clean
			}
		}
	}
	if wheel != "" {
		return wheel + ".metadata"
	}
	if anyFile != "" && strings.HasSuffix(strings.ToLower(path.Base(anyFile)), ".whl") {
		return anyFile + ".metadata"
	}
	return ""
}

func stripURLNoise(u string) string {
	clean := u
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	return clean
}

// resolveMetadataUpstreamURL 将制品 URL 上的 .metadata 改写到 MetadataUpstream。
// 国内 file 镜像常不托管 PEP 658 metadata（404），官方 files.pythonhosted.org 才有。
func resolveMetadataUpstreamURL(metaOrFileURL string, pypi config.PlatformConfig) string {
	raw := stripURLNoise(metaOrFileURL)
	if raw == "" {
		return ""
	}
	base := strings.TrimSpace(pypi.MetadataUpstream)
	if base == "" {
		base = strings.TrimSpace(pypi.FileUpstream)
	}
	if base == "" {
		base = "https://files.pythonhosted.org"
	}
	base = strings.TrimRight(base, "/")

	pathPart := raw
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		pathPart = u.Path
	}
	idx := strings.Index(pathPart, "/packages/")
	if idx < 0 {
		// 非 packages 路径则原样返回（已是绝对 metadata URL 时少见）
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			return raw
		}
		return base + "/" + strings.TrimLeft(pathPart, "/")
	}
	pathPart = pathPart[idx:] // /packages/...
	if !strings.HasSuffix(strings.ToLower(pathPart), ".metadata") {
		pathPart += ".metadata"
	}
	return base + pathPart
}

func (s *Service) fetchRequires(ctx context.Context, metaURL string, cfg config.Config) ([]string, error) {
	pypi := cfg.Platforms["pypi"]
	target := resolveMetadataUpstreamURL(metaURL, pypi)
	body, err := s.dl.FetchMetadata(ctx, target, "pypi", cfg.Cache.IndexTTLSeconds, true)
	if err != nil {
		return nil, err
	}
	return pypihandler.ParseRequiresDist(body), nil
}

func (s *Service) startOne(ctx context.Context, rawURL string) {
	s.mu.Lock()
	if _, ok := s.cancel[rawURL]; ok {
		s.mu.Unlock()
		return
	}
	cctx, cancel := context.WithCancel(ctx)
	s.cancel[rawURL] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.cancel, rawURL)
			s.mu.Unlock()
			cancel()
		}()
		cfg := s.cfg.Get()
		pypi := cfg.Platforms["pypi"]
		taskID := s.sched.Begin("pypi", rawURL, scheduler.PriorityPrefetch, false, false)
		_, err := s.dl.GetOrDownload(cctx, downloader.Options{
			URL:            rawURL,
			SourceURL:      rawURL,
			CacheKey:       cache.KeyFromURL(rawURL),
			Concurrency:    pypi.Download.Concurrency,
			ChunkSize:      pypi.Download.ChunkSize,
			MinSize:        pypi.Download.MinSize,
			TTLSeconds:     cfg.Cache.PackageTTLSeconds,
			Platform:       "pypi",
			Prefetch:       true,
			TaskID:         taskID,
			Headers:        http.Header{},
			ChunkTTLHours:  cfg.Cache.ChunkTTLHours,
			Kind:           "package",
			ExpectedSHA256: downloader.LookupDigest(rawURL),
			OnAcquired:     func() { s.sched.MarkRunning(taskID) },
			OnProgress:     func(done, total int64) { s.sched.UpdateProgress(taskID, done, total) },
		})
		s.sched.End(taskID, err)
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			s.log.Warn("prefetch failed", zap.String("url", rawURL), zap.Error(err))
		}
	}()
}

func (s *Service) Cancel(urlOrID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskURL, marked := s.sched.CancelPrefetch(urlOrID)
	keys := []string{urlOrID}
	if taskURL != "" {
		keys = append(keys, taskURL)
	}
	if u, ok := s.sched.URLOf(urlOrID); ok && u != "" {
		keys = append(keys, u)
	}

	stopped := false
	seen := map[string]struct{}{}
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if c, ok := s.cancel[k]; ok {
			c()
			delete(s.cancel, k)
			stopped = true
		}
	}
	return stopped || marked
}
