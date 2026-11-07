package generator

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 构建上下文秘密隔离：compose 的构建上下文是项目根，Dockerfile 又整目录 COPY backend/ 与 frontend/，
// 没被 .dockerignore 排除的运行秘密会进入构建上下文和 builder 层缓存。
//
// 依赖里没有 Docker ignore 的语义解析库（不为测试引入），这里不自造匹配器，只钉住规则文本、
// 例外范围和先后顺序；规则在 Docker 里实际排除 / 保留了哪些路径，由真实 docker build 另行验证。

// dockerignoreBaseRules 原有的排除规则（Git、依赖与构建产物、日志、SQL、文档等），不能被删掉或弱化。
var dockerignoreBaseRules = []string{
	".git", ".gitignore", ".idea", ".vscode",
	"**/node_modules", "**/dist", "**/build", "**/release",
	"backend/logs", "backend/*.local", "backend/build",
	"docs", "scripts/sql",
	"*.md", ".DS_Store",
}

// dockerignoreSecretRules 运行秘密与本机私有文件的排除规则。Docker 的规则从上下文根按路径匹配，
// 不带 **/ 的只管根目录，所以都要带 **/ 才能覆盖根 / backend / frontend 各层。
var dockerignoreSecretRules = []string{
	"**/.env*",
	"**/*.local",
	"**/*account*.txt",
	"**/browser-credentials.json",
}

// dockerignoreAllowedNegations 唯一允许的例外：前端生产构建要读的两份非敏感配置，逐个文件放行。
var dockerignoreAllowedNegations = []string{"!frontend/.env", "!frontend/.env.production"}

// generatedEnvFiles 生成项目里应有的全部 .env* 文件。新增 .env* 文件时必须同时确认 .dockerignore 的处理。
var generatedEnvFiles = []string{"backend/.env.example", "frontend/.env", "frontend/.env.production"}

// dockerignoreRules 取出 .dockerignore 的规则行（去掉注释和空行，保持原顺序）。
func dockerignoreRules(content string) []string {
	var rules []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rules = append(rules, line)
	}
	return rules
}

// TestDockerignore_RuntimeSecretsExcluded 三种组合生成的 .dockerignore 与模板一致：排除各层 .env、本机私有覆盖与
// 账号 / 浏览器凭据文件，只放行 frontend/.env 与 frontend/.env.production 两个文件，原有排除规则保持不变。
func TestDockerignore_RuntimeSecretsExcluded(t *testing.T) {
	tmpl := readFile(t, filepath.Join("..", "..", "templates", "project", ".dockerignore"))

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
			outDir := filepath.Join(t.TempDir(), "demo-dockerignore")
			if _, err := Generate(Options{
				ProjectName: "demo-dockerignore",
				Module:      "example.com/demo-dockerignore",
				OutputDir:   outDir,
				Components:  tc.optional,
			}); err != nil {
				t.Fatalf("生成失败: %v", err)
			}

			content := readFile(t, filepath.Join(outDir, ".dockerignore"))
			if content != tmpl {
				t.Fatalf("生成的 .dockerignore 应与模板一致:\n%s", content)
			}
			rules := dockerignoreRules(content)

			for _, want := range append(append([]string{}, dockerignoreBaseRules...), dockerignoreSecretRules...) {
				if !slices.Contains(rules, want) {
					t.Errorf(".dockerignore 缺少排除规则 %q", want)
				}
			}

			// 例外只能是这两个精确文件，不能整体放行 frontend/.env* 或其它路径
			var negations []string
			for _, r := range rules {
				if strings.HasPrefix(r, "!") {
					negations = append(negations, r)
				}
			}
			if !slices.Equal(negations, dockerignoreAllowedNegations) {
				t.Errorf(".dockerignore 的例外应恰好是 %v，实际 %v", dockerignoreAllowedNegations, negations)
			}

			// Docker 按「最后一条命中的规则」定去留：例外必须排在 .env / .local 排除之后才生效，
			// 并且放在最后，免得后面的规则又把这两个构建输入排除掉
			n := len(dockerignoreAllowedNegations)
			if len(rules) < n || !slices.Equal(rules[len(rules)-n:], dockerignoreAllowedNegations) {
				t.Errorf(".dockerignore 的两条例外应放在最后，实际规则顺序 %v", rules)
			}
			for _, secret := range dockerignoreSecretRules {
				if i := slices.Index(rules, secret); i >= len(rules)-n {
					t.Errorf("排除规则 %q 应在例外之前", secret)
				}
			}

			// 例外指向的就是生成项目里真实存在的前端配置；backend/.env.example 不放行
			var envFiles []string
			err := filepath.WalkDir(outDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasPrefix(d.Name(), ".env") {
					rel, _ := filepath.Rel(outDir, path)
					envFiles = append(envFiles, filepath.ToSlash(rel))
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(envFiles)
			if !slices.Equal(envFiles, generatedEnvFiles) {
				t.Errorf("生成项目的 .env* 文件应为 %v，实际 %v；新增 .env* 文件须同时确认 .dockerignore 的处理", generatedEnvFiles, envFiles)
			}
		})
	}
}
