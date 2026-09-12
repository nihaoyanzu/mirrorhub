package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/livehl/mirrorhub/internal/config"
)

func TestAcquireConnReservesForInteractive(t *testing.T) {
	pl := newPlatformLimiter("global", config.RateLimitConfig{
		BandwidthMbps:  50,
		MaxConcurrent:  20,
		MaxConnections: 20, // reserved=8 → prefetch limit=12
	})
	if pl.connReserved != 8 {
		t.Fatalf("reserved=%d want 8", pl.connReserved)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	const prefetchN = 12
	for i := 0; i < prefetchN; i++ {
		if err := pl.AcquireConn(ctx, false); err != nil {
			t.Fatalf("prefetch conn %d: %v", i, err)
		}
	}
	// 第 13 个预取应阻塞/超时
	errCh := make(chan error, 1)
	go func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer ccancel()
		errCh <- pl.AcquireConn(cctx, false)
	}()
	if err := <-errCh; err == nil {
		t.Fatal("预取不应占满预留连接")
	}

	// 交互仍可获取
	ictx, icancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer icancel()
	if err := pl.AcquireConn(ictx, true); err != nil {
		t.Fatalf("交互应能拿到预留连接: %v", err)
	}
}

func TestSetPrefetchPausedChanged(t *testing.T) {
	pl := newPlatformLimiter("global", config.RateLimitConfig{MaxConnections: 10, MaxConcurrent: 4})
	if !pl.SetPrefetchPaused(true) {
		t.Fatal("首次 pause 应 changed")
	}
	if pl.SetPrefetchPaused(true) {
		t.Fatal("重复 pause 不应 changed")
	}
	if !pl.SetPrefetchPaused(false) {
		t.Fatal("resume 应 changed")
	}
}

func TestRegistryGlobalBucket(t *testing.T) {
	r := NewRegistry(config.RateLimitConfig{
		BandwidthMbps:  12,
		MaxConcurrent:  7,
		MaxConnections: 40,
	}, 0.25)

	a := r.Get("pypi")
	b := r.Get("npm")
	if a == nil || b == nil {
		t.Fatal("Get 应返回全局桶")
	}
	if a != b {
		t.Fatal("不同平台名应指向同一全局桶")
	}
	if a.name != "global" {
		t.Fatalf("name=%s want global", a.name)
	}
	if r.IdleRatio() != 0.25 {
		t.Fatalf("idle=%v want 0.25", r.IdleRatio())
	}

	snaps := r.Snapshots()
	if len(snaps) != 1 {
		t.Fatalf("snapshots=%d want 1", len(snaps))
	}
	if snaps[0]["platform"] != "global" {
		t.Fatalf("snapshot platform=%v", snaps[0]["platform"])
	}
	if snaps[0]["bandwidth_mbps"] != 12.0 {
		t.Fatalf("bandwidth=%v want 12", snaps[0]["bandwidth_mbps"])
	}
	if snaps[0]["max_concurrent"] != 7 {
		t.Fatalf("max_concurrent=%v want 7", snaps[0]["max_concurrent"])
	}
}

func TestRegistryUpdatePreservesPaused(t *testing.T) {
	r := NewRegistry(config.RateLimitConfig{BandwidthMbps: 10, MaxConcurrent: 4, MaxConnections: 16}, 0.3)
	pl := r.Get("pypi")
	if !pl.SetPrefetchPaused(true) {
		t.Fatal("pause 应 changed")
	}
	r.Update(config.RateLimitConfig{
		BandwidthMbps:  20,
		MaxConcurrent:  8,
		MaxConnections: 32,
	})
	pl2 := r.Get("anything")
	if pl2 == pl {
		t.Fatal("Update 应换新 limiter")
	}
	if !pl2.PrefetchPaused() {
		t.Fatal("Update 应保留 prefetchPaused")
	}
	if pl2.cfg.BandwidthMbps != 20 {
		t.Fatalf("cfg 未更新: %+v", pl2.cfg)
	}
	snaps := r.Snapshots()
	if len(snaps) != 1 || snaps[0]["platform"] != "global" {
		t.Fatalf("snapshots=%v", snaps)
	}
}

func TestEffectiveBandwidthMbpsWindows(t *testing.T) {
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)

	cfg := config.RateLimitConfig{BandwidthMbps: 0}
	if cfg.EffectiveBandwidthMbps(base) != 0 {
		t.Fatal("默认 0 应全速")
	}

	cfg = config.RateLimitConfig{BandwidthMbps: 50}
	if cfg.EffectiveBandwidthMbps(base) != 50 {
		t.Fatal("无窗口应用默认")
	}

	cfg = config.RateLimitConfig{
		BandwidthMbps: 0,
		Windows: []config.RateLimitWindow{
			{Start: "09:00", End: "18:00", BandwidthMbps: 20},
		},
	}
	if got := cfg.EffectiveBandwidthMbps(base); got != 20 {
		t.Fatalf("工作时段 want 20 got %v", got)
	}
	night := time.Date(2026, 9, 12, 22, 0, 0, 0, time.Local)
	if got := cfg.EffectiveBandwidthMbps(night); got != 0 {
		t.Fatalf("窗外应全速 got %v", got)
	}

	cfg = config.RateLimitConfig{
		BandwidthMbps: 30,
		Windows: []config.RateLimitWindow{
			{Start: "09:00", End: "18:00", BandwidthMbps: 0},
		},
	}
	if got := cfg.EffectiveBandwidthMbps(base); got != 0 {
		t.Fatalf("窗口 0 应覆盖默认 got %v", got)
	}
	if got := cfg.EffectiveBandwidthMbps(night); got != 30 {
		t.Fatalf("窗外用默认 30 got %v", got)
	}
}

func TestInteractiveWaitNeverLimited(t *testing.T) {
	pl := newPlatformLimiter("global", config.RateLimitConfig{
		BandwidthMbps:  0.1,
		MaxConcurrent:  4,
		MaxConnections: 16,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	// 大块若走交互桶会超时；交互应立即返回
	if err := pl.WaitNClass(ctx, 8<<20, ClassInteractive); err != nil {
		t.Fatalf("交互应全速: %v", err)
	}
}

func TestUpdateHotSwapEffective(t *testing.T) {
	r := NewRegistry(config.RateLimitConfig{BandwidthMbps: 0, MaxConcurrent: 4, MaxConnections: 16}, 0.3)
	pl0 := r.Get("pypi")
	if pl0.cfg.BandwidthLimitedAt(time.Now()) {
		t.Fatal("初始应不限速")
	}
	r.Update(config.RateLimitConfig{BandwidthMbps: 15, MaxConcurrent: 4, MaxConnections: 16})
	pl1 := r.Get("pypi")
	if pl1 == pl0 {
		t.Fatal("Update 应立即换桶")
	}
	if !pl1.cfg.BandwidthLimitedAt(time.Now()) {
		t.Fatal("Update 后应立即限速")
	}
	if snap := pl1.Snapshot(); snap["bandwidth_limited"] != true {
		t.Fatalf("snapshot 未立即反映: %v", snap)
	}
}
