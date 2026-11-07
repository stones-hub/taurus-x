package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 模板依赖的 taurus-pro-common 正式版本：已公开发布，升级时与 components.go 一起改。
const (
	commonPackage = "github.com/stones-hub/taurus-pro-common"
	commonRelease = "v0.4.0"
)

// TestCommonComponentPublishedRelease 组件表里的 common 必须是必选组件并指向已发布的正式版本，不能是旧版或伪版本。
func TestCommonComponentPublishedRelease(t *testing.T) {
	c, ok := components.Find("common")
	if !ok {
		t.Fatal("组件表缺少 common")
	}
	if c.Package != commonPackage || c.Version != commonRelease {
		t.Errorf("common 组件应为 %s %s，实际 %s %s", commonPackage, commonRelease, c.Package, c.Version)
	}
	if !c.Required {
		t.Error("common 组件应为必选")
	}
}

// TestGenerate_CommonRequire 只渲染（不 tidy / wire）：三种组合生成的 go.mod 都恰好 require 一次
// common 正式版本且不带 replace。完整构建与测试由 TestGenerate_Combinations 承担。
func TestGenerate_CommonRequire(t *testing.T) {
	cases := []struct {
		name     string
		optional []string
	}{
		{"只选必选组件", nil},
		{"典型组合 grpc+opentelemetry", []string{"grpc", "opentelemetry"}},
		{"全部组件", components.OptionalNames()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "demo-common")
			result, err := Generate(Options{
				ProjectName: "demo-common",
				Module:      "example.com/demo-common",
				OutputDir:   outDir,
				Components:  tc.optional,
			})
			if err != nil {
				t.Fatalf("生成失败: %v", err)
			}
			if !contains(result.Components, "common") {
				t.Fatalf("common 应总被选中，实际组件 %v", result.Components)
			}
			for _, name := range tc.optional {
				if !contains(result.Components, name) {
					t.Fatalf("应选中 %s，实际组件 %v", name, result.Components)
				}
			}

			goMod := readFile(t, filepath.Join(outDir, "backend", "go.mod"))
			if goModReplace.MatchString(goMod) {
				t.Errorf("生成的 go.mod 不应带 replace:\n%s", goMod)
			}
			if n := strings.Count(goMod, commonPackage+" "); n != 1 {
				t.Errorf("go.mod 里 %s 应出现 1 次，实际 %d:\n%s", commonPackage, n, goMod)
			}
			if !strings.Contains(goMod, "\t"+commonPackage+" "+commonRelease+"\n") {
				t.Errorf("go.mod 应 require %s %s:\n%s", commonPackage, commonRelease, goMod)
			}
		})
	}
}
