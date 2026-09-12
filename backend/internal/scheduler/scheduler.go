package scheduler

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/livehl/mirrorhub/internal/config"
	"github.com/livehl/mirrorhub/internal/metrics"
	"github.com/livehl/mirrorhub/internal/ratelimit"
)

type Priority int

const (
	PriorityInteractive Priority = 0
	PriorityResume      Priority = 1
	PriorityPrefetch    Priority = 2
)

func (p Priority) String() string {
	switch p {
	case PriorityInteractive:
		return "P0"
	case PriorityResume:
		return "P1"
	default:
		return "P2"
	}
}

type TaskInfo struct {
	ID        string    `json:"id"`
	Platform  string    `json:"platform"`
	URL       string    `json:"url"`
	Priority  string    `json:"priority"`
	Boosted   bool      `json:"boosted"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	QueuedAt  time.Time `json:"queued_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	WaitMs    int64     `json:"wait_ms"`
	Error     string    `json:"error,omitempty"`

	// countsActive 表示 Begin 时是否计入 p0Active（仅 P0 交互）。
	// 暂停时把运行中 P2 改标为 P1 不改此标记，避免 End 误减。
	countsActive bool
}

type Scheduler struct {
	mu           sync.Mutex
	tasks        map[string]*TaskInfo
	order        []string
	limiters     *ratelimit.Registry
	cfgFn        func() config.SchedulerConfig
	log          *zap.Logger
	p0Active     atomic.Int64
	nextID       atomic.Uint64
	resumeCount  atomic.Int64
}

func New(lim *ratelimit.Registry, cfgFn func() config.SchedulerConfig, log *zap.Logger) *Scheduler {
	return &Scheduler{
		tasks:    map[string]*TaskInfo{},
		limiters: lim,
		cfgFn:    cfgFn,
		log:      log,
	}
}

func (s *Scheduler) Begin(platform, rawURL string, prio Priority, boost bool) string {
	seq := s.nextID.Add(1)
	id := rawURL + "#" + strconv.FormatUint(seq, 10)
	now := time.Now()
	prioLabel := prio.String()
	if boost && prio == PriorityInteractive {
		prioLabel = "P0+"
	}
	// 若预取正被 pause，新预取任务记为 P1 Resume（仅展示/恢复优先级）
	if prio == PriorityPrefetch {
		if pl := s.limiters.Get(platform); pl != nil && pl.PrefetchPaused() {
			prio = PriorityResume
			prioLabel = PriorityResume.String()
			s.resumeCount.Add(1)
		}
	}
	// 仅真实交互（P0）计入 p0Active。
	// 若把 P1 Resume 也算进去，暂停期间入队的预取会顶住 p0Active，导致永远无法 resumeOnIdle。
	countsActive := prio == PriorityInteractive
	info := &TaskInfo{
		ID:           id,
		Platform:     platform,
		URL:          rawURL,
		Priority:     prioLabel,
		Boosted:      boost,
		Status:       "queued",
		CreatedAt:    now,
		QueuedAt:     now,
		UpdatedAt:    now,
		WaitMs:       0,
		countsActive: countsActive,
	}
	s.mu.Lock()
	s.tasks[id] = info
	if boost || prio == PriorityInteractive || prio == PriorityResume {
		s.order = append([]string{id}, s.order...)
	} else {
		s.order = append(s.order, id)
	}
	s.trimOrderLocked()
	s.refreshTaskMetricsLocked()
	s.mu.Unlock()

	if countsActive {
		s.p0Active.Add(1)
		metrics.SchedulerP0Active.Set(float64(s.p0Active.Load()))
		s.pausePrefetchIfNeeded(platform)
	}
	return id
}

// MarkRunning 将 queued 任务推进为 running，并记录开始时间（用于 wait_ms）。
func (s *Scheduler) MarkRunning(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok || t == nil {
		return
	}
	if t.Status != "queued" {
		return
	}
	now := time.Now()
	t.Status = "running"
	t.StartedAt = now
	t.UpdatedAt = now
	t.WaitMs = now.Sub(t.QueuedAt).Milliseconds()
	s.refreshTaskMetricsLocked()
}

func (s *Scheduler) trimOrderLocked() {
	for len(s.order) > 500 {
		removed := false
		for i := len(s.order) - 1; i >= 0; i-- {
			id := s.order[i]
			t, ok := s.tasks[id]
			if !ok || t.Status == "done" || t.Status == "error" || t.Status == "cancelled" {
				s.order = append(s.order[:i], s.order[i+1:]...)
				if ok {
					s.releaseActiveLocked(t) // 防御：异常路径未 End 时避免泄漏
					delete(s.tasks, id)
				}
				removed = true
				break
			}
		}
		if !removed {
			break
		}
	}
}

func (s *Scheduler) releaseActiveLocked(t *TaskInfo) {
	if t == nil || !t.countsActive {
		return
	}
	t.countsActive = false
	n := s.p0Active.Add(-1)
	if n < 0 {
		s.p0Active.Store(0)
		n = 0
	}
	metrics.SchedulerP0Active.Set(float64(n))
}

func (s *Scheduler) End(id string, err error) {
	s.mu.Lock()
	t, ok := s.tasks[id]
	if ok {
		// 已取消的任务不被后续 End 覆盖成 done/error
		if t.Status == "cancelled" {
			s.releaseActiveLocked(t)
			s.refreshTaskMetricsLocked()
			s.mu.Unlock()
			s.resumePrefetchIfIdle()
			return
		}
		t.UpdatedAt = time.Now()
		if err != nil {
			if isCancelErr(err) {
				t.Status = "cancelled"
				t.Error = ""
			} else {
				t.Status = "error"
				t.Error = err.Error()
				metrics.ErrorsTotal.WithLabelValues("task").Inc()
			}
		} else {
			t.Status = "done"
		}
		s.releaseActiveLocked(t)
	}
	s.refreshTaskMetricsLocked()
	s.mu.Unlock()
	s.resumePrefetchIfIdle()
}

func isCancelErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "context cancelled")
}

func isPrefetchCancellable(prio string) bool {
	return prio == PriorityPrefetch.String() || prio == PriorityResume.String()
}

// URLOf 按任务 id 取 URL；不存在则 ok=false。
func (s *Scheduler) URLOf(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok || t == nil {
		return "", false
	}
	return t.URL, true
}

func (s *Scheduler) pausePrefetchIfNeeded(platform string) {
	cfg := s.cfgFn()
	if !cfg.InteractivePriority {
		return
	}
	if cfg.Prefetch.OnInteractive != "pause" {
		return
	}
	if pl := s.limiters.Get(platform); pl != nil {
		if changed := pl.SetPrefetchPaused(true); changed {
			metrics.PrefetchPaused.Set(1)
			s.log.Info("prefetch paused due to interactive traffic", zap.String("platform", platform))
		}
		// 运行中的 P2 标记为 P1（仅改展示/恢复优先级，不计入 p0Active）
		s.mu.Lock()
		for _, t := range s.tasks {
			if t.Status == "running" && t.Priority == PriorityPrefetch.String() {
				t.Priority = PriorityResume.String()
				s.resumeCount.Add(1)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Scheduler) resumePrefetchIfIdle() {
	cfg := s.cfgFn()
	if !cfg.Prefetch.ResumeOnIdle {
		return
	}
	if s.p0Active.Load() > 0 {
		return
	}
	for _, snap := range s.limiters.Snapshots() {
		name, _ := snap["platform"].(string)
		if pl := s.limiters.Get(name); pl != nil {
			if pl.SetPrefetchPaused(false) {
				metrics.PrefetchPaused.Set(0)
			}
		}
	}
}

func (s *Scheduler) InteractiveActive() int64 {
	return s.p0Active.Load()
}

func (s *Scheduler) ResumeCount() int64 {
	return s.resumeCount.Load()
}

func (s *Scheduler) List() []TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskInfo, 0, len(s.order))
	now := time.Now()
	for i := len(s.order) - 1; i >= 0; i-- {
		if t, ok := s.tasks[s.order[i]]; ok {
			cp := *t
			if cp.Status == "running" && !cp.StartedAt.IsZero() {
				cp.WaitMs = cp.StartedAt.Sub(cp.QueuedAt).Milliseconds()
			} else if cp.Status == "queued" {
				cp.WaitMs = now.Sub(cp.QueuedAt).Milliseconds()
			}
			out = append(out, cp)
		}
	}
	return out
}

func (s *Scheduler) refreshTaskMetricsLocked() {
	metrics.Tasks.Reset()
	for _, t := range s.tasks {
		metrics.Tasks.WithLabelValues(t.Status, t.Priority).Inc()
	}
}

// CancelPrefetch 仅取消 P1/P2 预取类任务，返回 URL 供上层取消下载 context。
func (s *Scheduler) CancelPrefetch(idOrURL string) (taskURL string, ok bool) {
	s.mu.Lock()
	mark := func(t *TaskInfo) {
		t.Status = "cancelled"
		t.UpdatedAt = time.Now()
		t.Error = ""
		s.releaseActiveLocked(t)
	}
	if t, found := s.tasks[idOrURL]; found {
		if (t.Status == "running" || t.Status == "queued") && isPrefetchCancellable(t.Priority) {
			mark(t)
			taskURL, ok = t.URL, true
		}
	} else {
		for _, tid := range s.order {
			tt := s.tasks[tid]
			if tt == nil || tt.URL != idOrURL {
				continue
			}
			if !isPrefetchCancellable(tt.Priority) {
				continue
			}
			if tt.Status != "running" && tt.Status != "queued" {
				continue
			}
			mark(tt)
			taskURL, ok = tt.URL, true
			break
		}
	}
	if ok {
		s.refreshTaskMetricsLocked()
	}
	s.mu.Unlock()
	if ok {
		s.resumePrefetchIfIdle()
	}
	return taskURL, ok
}

// ClearFinished 清理已结束任务（done/error/cancelled），返回清理数量。
func (s *Scheduler) ClearFinished() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	kept := s.order[:0]
	for _, id := range s.order {
		t, ok := s.tasks[id]
		if !ok {
			continue
		}
		if t.Status == "done" || t.Status == "error" || t.Status == "cancelled" {
			s.releaseActiveLocked(t) // 防御：异常路径未 End 时避免泄漏
			delete(s.tasks, id)
			n++
			continue
		}
		kept = append(kept, id)
	}
	s.order = kept
	s.refreshTaskMetricsLocked()
	return n
}
