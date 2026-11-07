package generator

import (
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 模板依赖的 taurus-pro-milvus 正式版本：已公开发布、带只读恢复与观察回调接口。
// 生成项目文档里写的版本号都按它断言，升级时与 components.go 一起改。
const (
	milvusPackage = "github.com/stones-hub/taurus-pro-milvus"
	milvusRelease = "v0.3.0"
)

// goModReplace 匹配 go.mod 里的 replace 指令（单行或块形式）。
var goModReplace = regexp.MustCompile(`(?m)^\s*replace\b`)

// TestMilvusComponentPublishedRelease 组件表里的 milvus 必须指向已发布的正式版本，不能是旧版或伪版本。
func TestMilvusComponentPublishedRelease(t *testing.T) {
	c, ok := components.Find("milvus")
	if !ok {
		t.Fatal("组件表缺少 milvus")
	}
	if c.Package != milvusPackage || c.Version != milvusRelease {
		t.Errorf("milvus 组件应为 %s %s，实际 %s %s", milvusPackage, milvusRelease, c.Package, c.Version)
	}
}

// TestGenerate_MilvusTemplatesRender 只渲染模板（不 tidy / wire），尽早发现 Milvus 相关模板的解析错误：
// 模板在未选 milvus 时也会被解析，Go 复合字面量里的连续花括号会被 text/template 当成指令。
// Milvus 相关、wire 不会重写的生成文件还要已按 gofmt 格式化（用 go/format 比对，等价于 gofmt -l 无输出）；
// wire_gen.go 会被 wire 重写，用户实际拿到的版本由 TestGenerate_Combinations 的 assertGoFilesFormatted 检查。
// 不替代 TestGenerate_Combinations 的完整构建与测试，只补一道不依赖网络和 Go 依赖解析的回归。
func TestGenerate_MilvusTemplatesRender(t *testing.T) {
	gofmtChecked := map[string]bool{
		"components.go": true, "milvus_provider_test.go": true, "milvus_read_log_test.go": true, "wire_rollback_test.go": true,
	}
	cases := []struct {
		name     string
		optional []string
	}{
		{"只选必选组件", nil},
		{"全部组件", components.OptionalNames()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "demo-milvus")
			result, err := Generate(Options{
				ProjectName: "demo-milvus",
				Module:      "example.com/demo-milvus",
				OutputDir:   outDir,
				Components:  tc.optional,
			})
			if err != nil {
				t.Fatalf("生成失败: %v", err)
			}
			selected := contains(result.Components, "milvus")
			if selected != (tc.optional != nil) {
				t.Fatalf("milvus 选中情况不符：实际 %t", selected)
			}

			// 正式依赖：选了 milvus 时 go.mod 恰好 require 一次已发布版本；任何组合都不带 replace
			goMod := readFile(t, filepath.Join(outDir, "backend", "go.mod"))
			if goModReplace.MatchString(goMod) {
				t.Errorf("生成的 go.mod 不应带 replace:\n%s", goMod)
			}
			wantReqs := 0
			if selected {
				wantReqs = 1
			}
			if n := strings.Count(goMod, milvusPackage+" "); n != wantReqs {
				t.Errorf("go.mod 里 %s 应出现 %d 次，实际 %d:\n%s", milvusPackage, wantReqs, n, goMod)
			}
			if selected && !strings.Contains(goMod, "\t"+milvusPackage+" "+milvusRelease+"\n") {
				t.Errorf("go.mod 应 require %s %s:\n%s", milvusPackage, milvusRelease, goMod)
			}

			// internal/taurus 下渲染出的 Go 文件都必须能被 go/parser 解析，且不残留模板指令
			taurusDir := filepath.Join(outDir, "backend", "internal", "taurus")
			entries, err := os.ReadDir(taurusDir)
			if err != nil {
				t.Fatal(err)
			}
			fset := token.NewFileSet()
			formatChecked := 0
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
					continue
				}
				path := filepath.Join(taurusDir, e.Name())
				src := readFile(t, path)
				if strings.Contains(src, "{{") {
					t.Errorf("%s 残留模板指令", e.Name())
				}
				if _, err := parser.ParseFile(fset, path, src, parser.ParseComments); err != nil {
					t.Errorf("%s 不是合法的 Go 源码: %v", e.Name(), err)
				}
				if !gofmtChecked[e.Name()] {
					continue
				}
				formatChecked++
				if formatted, err := format.Source([]byte(src)); err != nil {
					t.Errorf("%s 无法被 gofmt 解析: %v", e.Name(), err)
				} else if string(formatted) != src {
					t.Errorf("%s 未按 gofmt 格式化", e.Name())
				}
			}
			if formatChecked != len(gofmtChecked) {
				t.Errorf("应对 %d 个 Milvus 相关文件做 gofmt 比对，实际只找到 %d 个", len(gofmtChecked), formatChecked)
			}

			if selected {
				readLogTest := readFile(t, filepath.Join(taurusDir, "milvus_read_log_test.go"))
				// 两处嵌套字面量拆成多行后仍保持原内容
				for _, want := range []string{
					"TypeParams: []*commonpb.KeyValuePair{\n\t\t\t\t\t{Key: \"dim\", Value: \"2\"},\n\t\t\t\t}},",
					"FieldsData: []*schemapb.FieldData{\n\t\t\t{\n\t\t\t\tType: schemapb.DataType_Int64, FieldName: \"id\", FieldId: 100,",
				} {
					if !strings.Contains(readLogTest, want) {
						t.Errorf("milvus_read_log_test.go 缺少 %q", want)
					}
				}
			}
			// 真实的日志 provider、观察回调、wire 传参与离线测试（未选时三个测试文件只有包声明）
			assertMilvusFailFast(t, outDir, result.Components)
		})
	}
}
