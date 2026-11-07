package generator

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 前端依赖回归：axios@1.20.0 精确依赖的 follow-redirects 必须经定向 override 锁到 1.16.2。
// 1.16.0–1.16.1 处于 GHSA-mrwp-f3q8-3g9p 公告受影响区间；这里只保护锁定版本，不代表已复现 Axios 凭据泄漏。
const (
	followRedirectsOverrideKey = "axios@1.20.0>follow-redirects"
	followRedirectsVersion     = "1.16.2"
	// 由 pnpm 10.34.7 正常生成，与 registry.npmjs.org 公布的 dist.integrity 一致。
	followRedirectsIntegrity = "sha512-/kVP2U8z596Ia0gwFrwyIeo74ta5aVmSeU1FfAUMAmcEP5EP7zydmag3SLiN64NaRKef3lxsq7i/g7ltJQDGow=="
)

// followRedirectsVersionRef 锁内任一处 follow-redirects 版本引用（override、包条目、axios 依赖、快照）。
var followRedirectsVersionRef = regexp.MustCompile(`follow-redirects(?:@|: )(\d+\.\d+\.\d+)`)

func TestGenerate_FrontendFollowRedirectsPinned(t *testing.T) {
	project := "followpin"
	outDir := filepath.Join(tmpRoot, "taurusx-test-follow-redirects")
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatalf("清理输出目录失败: %v", err)
	}
	var log bytes.Buffer
	if _, err := Generate(Options{
		ProjectName: project,
		Module:      "example.com/" + project,
		OutputDir:   outDir,
		Stdout:      &log,
	}); err != nil {
		t.Fatalf("生成失败: %v\n%s", err, log.String())
	}

	var pkg struct {
		Pnpm struct {
			Overrides map[string]string `json:"overrides"`
		} `json:"pnpm"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "frontend", "package.json"))), &pkg); err != nil {
		t.Fatalf("解析生成的 package.json 失败: %v", err)
	}
	if got := pkg.Pnpm.Overrides; len(got) != 1 || got[followRedirectsOverrideKey] != followRedirectsVersion {
		t.Errorf("package.json pnpm.overrides 应只有 %q: %q，实际 %v", followRedirectsOverrideKey, followRedirectsVersion, got)
	}
	// override 键绑定 axios 1.20.0；升级 axios 时必须重新核对该 override
	if got := pkg.Dependencies["axios"]; got != "^1.20.0" {
		t.Errorf("axios 依赖范围变化（%q），须同步重核 %s override", got, followRedirectsOverrideKey)
	}

	lock := readFile(t, filepath.Join(outDir, "frontend", "pnpm-lock.yaml"))
	for _, want := range []string{
		"\noverrides:\n  " + followRedirectsOverrideKey + ": " + followRedirectsVersion + "\n",
		"\n  follow-redirects@" + followRedirectsVersion + ":\n    resolution: {integrity: " + followRedirectsIntegrity + "}\n",
		"\n  axios@1.20.0:\n    dependencies:\n      follow-redirects: " + followRedirectsVersion + "\n",
		"\n  follow-redirects@" + followRedirectsVersion + ": {}\n",
	} {
		if !strings.Contains(lock, want) {
			t.Errorf("pnpm-lock.yaml 缺少 %q", want)
		}
	}
	// 锁里不得残留其它 follow-redirects 版本（包括公告受影响的 1.16.0/1.16.1）
	refs := followRedirectsVersionRef.FindAllStringSubmatch(lock, -1)
	if len(refs) != 4 {
		t.Errorf("pnpm-lock.yaml 中 follow-redirects 版本引用应为 4 处（override、包条目、axios 依赖、快照），实际 %d 处", len(refs))
	}
	for _, ref := range refs {
		if ref[1] != followRedirectsVersion {
			t.Errorf("pnpm-lock.yaml 出现 follow-redirects %s 引用，应只锁 %s", ref[1], followRedirectsVersion)
		}
	}
}
