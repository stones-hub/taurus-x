package generator

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stones-hub/taurus-x/internal/components"
	"github.com/stones-hub/taurus-x/templates"
)

// 模板依赖的 taurus-pro-storage 正式版本：已公开发布、带 redisx.WithContextTimeoutEnabled，
// 会话期限要靠它约束 Redis socket 读写。这里写死版本，不跟随组件表，升级时与 components.go 一起改。
const (
	storagePackage = "github.com/stones-hub/taurus-pro-storage"
	storageRelease = "v0.3.1"
)

// TestStorageComponentPublishedRelease 组件表里的 storage 必须是必选组件并指向已发布的正式版本，不能是旧版或伪版本。
func TestStorageComponentPublishedRelease(t *testing.T) {
	c, ok := components.Find("storage")
	if !ok {
		t.Fatal("组件表缺少 storage")
	}
	if c.Package != storagePackage || c.Version != storageRelease {
		t.Errorf("storage 组件应为 %s %s，实际 %s %s", storagePackage, storageRelease, c.Package, c.Version)
	}
	if !c.Required {
		t.Error("storage 组件应为必选")
	}
}

// TestGoModReplacePattern goModReplace 必须能识别单行和块形式的 replace，否则不带 replace 的断言会漏判。
func TestGoModReplacePattern(t *testing.T) {
	for _, src := range []string{
		"module x\n\nreplace " + storagePackage + " => ../taurus-pro-storage\n",
		"module x\n\nreplace (\n\t" + storagePackage + " => ../taurus-pro-storage\n)\n",
	} {
		if !goModReplace.MatchString(src) {
			t.Errorf("goModReplace 未识别 replace:\n%s", src)
		}
	}
}

// TestRenderGoMod_StorageRequire 只渲染 backend/go.mod 模板（不写项目树、不 tidy / wire / build）：
// 三种组合都恰好 require 一次 storage 正式版本且不带 replace。完整构建与测试由 TestGenerate_Combinations 承担。
func TestRenderGoMod_StorageRequire(t *testing.T) {
	const goModTmpl = templateRoot + "/backend/go.mod" + tmplSuffix
	content, err := fs.ReadFile(templates.FS, goModTmpl)
	if err != nil {
		t.Fatal(err)
	}
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
			selected := normalizeComponents(tc.optional)
			if !contains(selected, "storage") {
				t.Fatalf("storage 应总被选中，实际组件 %v", selected)
			}
			data, _, err := buildTemplateData(Options{ProjectName: "demo-storage", Module: "example.com/demo-storage"}, selected)
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := render("backend/go.mod"+tmplSuffix, content, data)
			if err != nil {
				t.Fatal(err)
			}

			goMod := string(rendered)
			if goModReplace.MatchString(goMod) {
				t.Errorf("生成的 go.mod 不应带 replace:\n%s", goMod)
			}
			if n := strings.Count(goMod, storagePackage+" "); n != 1 {
				t.Errorf("go.mod 里 %s 应出现 1 次，实际 %d:\n%s", storagePackage, n, goMod)
			}
			if !strings.Contains(goMod, "\t"+storagePackage+" "+storageRelease+"\n") {
				t.Errorf("go.mod 应 require %s %s:\n%s", storagePackage, storageRelease, goMod)
			}
		})
	}
}
