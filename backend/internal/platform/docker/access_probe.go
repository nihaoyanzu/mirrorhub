package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	dockerhandler "github.com/livehl/mirrorhub/internal/handlers/docker"
	"github.com/livehl/mirrorhub/internal/platform"
)

func (p *DockerPlatform) ProbeAccess(env platform.AccessProbeEnv) []platform.AccessProbeCheck {
	base := strings.TrimRight(strings.TrimSpace(env.Cfg.Upstream), "/")
	if base == "" {
		base = "https://registry-1.docker.io"
	}
	authBase := strings.TrimSpace(env.Cfg.MetadataUpstream)
	if authBase == "" {
		authBase = "https://auth.docker.io"
	}

	const repo = "library/hello-world"
	const tag = "latest"
	tokens := dockerhandler.NewTokenSource(authBase, dockerhandler.AuthServiceFromRegistry(base), env.Client)

	t0 := time.Now()
	body, manifestURL, err := fetchProbeManifest(env, tokens, base, repo, tag)
	ms := time.Since(t0).Milliseconds()
	if err != nil {
		return []platform.AccessProbeCheck{{
			Name: "manifest", OK: false, MS: ms,
			Detail: fmt.Sprintf("%s: %v", manifestURL, err),
		}}
	}

	// manifest list → 再拉第一个子 manifest，才能拿到 layer digest
	if dig := firstListChildDigest(body); dig != "" {
		child, childURL, err := fetchProbeManifest(env, tokens, base, repo, dig)
		if err == nil && len(child) > 0 {
			body = child
			manifestURL = childURL
		}
	}

	checks := []platform.AccessProbeCheck{{
		Name: "manifest", OK: true, MS: ms,
		Detail: fmt.Sprintf("%s → 已拉取 manifest %d 字节", manifestURL, len(body)),
	}}

	digest := firstBlobDigest(body)
	if digest == "" {
		checks = append(checks, platform.AccessProbeCheck{
			Name: "download", OK: false, MS: 0,
			Detail: "manifest 中未找到 layer/config digest",
		})
		return checks
	}
	blobURL := dockerhandler.RegistryBlobURL(base, repo, digest)
	hdr := http.Header{}
	if tok, err := tokens.Bearer(env.Ctx, repo); err == nil && tok != "" {
		hdr.Set("Authorization", "Bearer "+tok)
	}
	checks = append(checks, platform.ProbeDownloadSample(env.Ctx, env.Client, "download", blobURL, hdr))
	return checks
}

func fetchProbeManifest(env platform.AccessProbeEnv, tokens *dockerhandler.TokenSource, registryBase, repo, reference string) ([]byte, string, error) {
	u := dockerhandler.RegistryManifestURL(registryBase, repo, reference)
	try := func(withAuth bool) (int, []byte, error) {
		hdr := http.Header{}
		hdr.Set("Accept", dockerhandler.DefaultManifestAccept())
		if withAuth {
			if tok, err := tokens.Bearer(env.Ctx, repo); err == nil && tok != "" {
				hdr.Set("Authorization", "Bearer "+tok)
			}
		}
		return platform.ProbeGETHeaders(env.Ctx, env.Client, u, 2<<20, hdr)
	}

	status, body, err := try(true)
	if err != nil {
		return nil, u, err
	}
	if status == http.StatusUnauthorized {
		tokens.Invalidate(repo)
		status, body, err = try(true)
		if err != nil {
			return nil, u, err
		}
	}
	if status >= 400 {
		status, body, err = try(false)
		if err != nil {
			return nil, u, err
		}
	}
	if status >= 400 {
		return nil, u, fmt.Errorf("HTTP %d", status)
	}
	if len(body) == 0 {
		return nil, u, fmt.Errorf("空 manifest")
	}
	return body, u, nil
}

func firstListChildDigest(manifest []byte) string {
	var doc struct {
		Manifests []struct {
			Digest    string `json:"digest"`
			MediaType string `json:"mediaType"`
		} `json:"manifests"`
	}
	if json.Unmarshal(manifest, &doc) != nil || len(doc.Manifests) == 0 {
		return ""
	}
	for _, m := range doc.Manifests {
		mt := strings.ToLower(m.MediaType)
		if strings.Contains(mt, "manifest.list") || strings.Contains(mt, "image.index") {
			continue
		}
		if d := strings.TrimSpace(m.Digest); d != "" {
			return d
		}
	}
	return strings.TrimSpace(doc.Manifests[0].Digest)
}

func firstBlobDigest(manifest []byte) string {
	var doc struct {
		Config *struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if json.Unmarshal(manifest, &doc) != nil {
		return ""
	}
	for _, l := range doc.Layers {
		if d := strings.TrimSpace(l.Digest); d != "" {
			return d
		}
	}
	if doc.Config != nil {
		if d := strings.TrimSpace(doc.Config.Digest); d != "" {
			return d
		}
	}
	return ""
}
