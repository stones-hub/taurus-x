package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 真实 Docker Compose 解析校验：生成的 docker-compose.yml 必须能被 `docker compose config` 接受。
//
// 字符串断言发现不了插值语法错误（例如把配置库的 ${VAR:default} 误写进 compose，compose 要求
// ${VAR:-default}），所以这里真的调用一次 docker compose。只做解析（config --quiet；勾选 milvus 的正向场景另用
// config --format json 在进程内解析取值），不拉镜像、不起容器、不连 Docker daemon，也不打印渲染结果。
//
// 隔离：显式 --env-file /dev/null，命令环境只有 PATH、临时 HOME / DOCKER_CONFIG 和合成的必填值
// （JWT_SECRET 与勾选 milvus 时的外置 S3 变量，compose 里都是 ${VAR:?...} 必填项），不读本机 .env、不继承开发机环境变量、
// 不读 ~/.docker/config.json（里面可能有登录信息）——只把 compose 插件二进制链接进临时 DOCKER_CONFIG。
//
// 本机没有 docker CLI 或 compose 插件时跳过并写明原因；设置 TAURUSX_REQUIRE_COMPOSE=1 时改为失败，
// 供必须覆盖这一项的验收环境使用。

// composeSyntheticJWTSecret 只用于让 compose 的必填插值通过，不是任何真实密钥。
const composeSyntheticJWTSecret = "compose-config-check-synthetic-not-a-secret-0000"

// composeSyntheticS3 勾选 milvus 时外置 S3 的必填项，只用于让 compose 解析通过：端点用保留的 .invalid 域名，
// 密钥是明显的合成串，都不是任何真实端点或凭据。
var composeSyntheticS3 = []struct{ name, value string }{
	{"MILVUS_S3_ADDRESS", "s3.compose-config-check.invalid"},
	{"MILVUS_S3_ACCESS_KEY_ID", "compose-config-check-synthetic-access-key-id"},
	{"MILVUS_S3_SECRET_ACCESS_KEY", "compose-config-check-synthetic-not-a-secret-s3"},
	{"MILVUS_S3_BUCKET", "compose-config-check-bucket"},
	{"MILVUS_S3_ROOT_PATH", "compose-config-check/milvus"},
	{"MILVUS_S3_REGION", "compose-config-check-region"},
}

// composeChecker 运行隔离环境下的 docker compose。
type composeChecker struct {
	docker string
	env    []string
}

// newComposeChecker 准备隔离的 docker compose 调用环境；条件不满足时按 TAURUSX_REQUIRE_COMPOSE 跳过或失败。
func newComposeChecker(t *testing.T) *composeChecker {
	t.Helper()
	unavailable := func(reason string) {
		t.Helper()
		if os.Getenv("TAURUSX_REQUIRE_COMPOSE") == "1" {
			t.Fatalf("TAURUSX_REQUIRE_COMPOSE=1 但 %s", reason)
		}
		t.Skipf("跳过真实 docker compose 解析校验：%s（设置 TAURUSX_REQUIRE_COMPOSE=1 可改为失败）", reason)
	}

	docker, err := exec.LookPath("docker")
	if err != nil {
		unavailable("PATH 里没有 docker CLI")
	}
	plugin := findComposePlugin()
	if plugin == "" {
		unavailable("找不到 docker compose 插件（docker-compose）")
	}

	home := t.TempDir()
	dockerConfig := filepath.Join(home, ".docker")
	if err := os.MkdirAll(filepath.Join(dockerConfig, "cli-plugins"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(plugin, filepath.Join(dockerConfig, "cli-plugins", "docker-compose")); err != nil {
		t.Fatal(err)
	}
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"DOCKER_CONFIG=" + dockerConfig,
		"JWT_SECRET=" + composeSyntheticJWTSecret,
	}
	for _, v := range composeSyntheticS3 {
		env = append(env, v.name+"="+v.value)
	}
	return &composeChecker{docker: docker, env: env}
}

// without 返回去掉指定变量的副本，用于验证必填项缺失时 compose 拒绝、以及不需要它们的组合照常解析。
func (c *composeChecker) without(names ...string) *composeChecker {
	out := &composeChecker{docker: c.docker}
	for _, kv := range c.env {
		drop := false
		for _, name := range names {
			if strings.HasPrefix(kv, name+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out.env = append(out.env, kv)
		}
	}
	return out
}

// withoutS3 去掉全部外置 S3 合成变量。
func (c *composeChecker) withoutS3() *composeChecker {
	names := make([]string, 0, len(composeSyntheticS3))
	for _, v := range composeSyntheticS3 {
		names = append(names, v.name)
	}
	return c.without(names...)
}

// redact 把合成的密钥值替换掉，失败诊断里不输出它们。
func redact(s string) string {
	s = strings.ReplaceAll(s, composeSyntheticJWTSecret, "<synthetic>")
	for _, v := range composeSyntheticS3 {
		if strings.HasSuffix(v.name, "_KEY_ID") || strings.HasSuffix(v.name, "_SECRET_ACCESS_KEY") {
			s = strings.ReplaceAll(s, v.value, "<synthetic>")
		}
	}
	return s
}

// findComposePlugin 按 Docker CLI 的插件目录约定找 compose 插件，只看文件是否存在，不读任何配置文件。
func findComposePlugin() string {
	var dirs []string
	if dc := os.Getenv("DOCKER_CONFIG"); dc != "" {
		dirs = append(dirs, filepath.Join(dc, "cli-plugins"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".docker", "cli-plugins"))
	}
	dirs = append(dirs,
		"/usr/local/lib/docker/cli-plugins",
		"/usr/local/libexec/docker/cli-plugins",
		"/usr/lib/docker/cli-plugins",
		"/usr/libexec/docker/cli-plugins",
		"/opt/homebrew/lib/docker/cli-plugins",
		"/Applications/Docker.app/Contents/Resources/cli-plugins",
	)
	for _, dir := range dirs {
		p := filepath.Join(dir, "docker-compose")
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// config 执行 `docker compose --env-file /dev/null -f <file> config --quiet`，返回错误输出（成功时为空）。
// 不用 -o / 不去掉 --quiet：渲染结果里有环境变量取值，不应进测试日志。
func (c *composeChecker) config(t *testing.T, composeFile string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.docker, "compose", "--env-file", "/dev/null", "-f", composeFile, "config", "--quiet")
	cmd.Dir = filepath.Dir(composeFile)
	cmd.Env = c.env
	var stderr bytes.Buffer
	cmd.Stdout = &stderr // --quiet 成功时不输出；万一有输出也只用于失败诊断
	cmd.Stderr = &stderr
	err := cmd.Run()
	return redact(stderr.String()), err
}

// configJSON 执行 `docker compose --env-file /dev/null -f <file> config --format json`，在进程内解析渲染结果。
// 渲染结果里有环境变量取值（含合成密钥），成功和失败都不进测试日志：命令失败只输出脱敏后的错误输出，
// JSON 解析失败只报字节数，不回显内容。
func (c *composeChecker) configJSON(t *testing.T, composeFile string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.docker, "compose", "--env-file", "/dev/null", "-f", composeFile, "config", "--format", "json")
	cmd.Dir = filepath.Dir(composeFile)
	cmd.Env = c.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker compose config --format json 失败: %v\n%s", err, redact(stderr.String()))
	}
	var project map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &project); err != nil {
		t.Fatalf("docker compose config --format json 的输出不是合法 JSON（%d 字节，内容不回显）", stdout.Len())
	}
	return project
}

// parsedService 从解析结果里取某个服务的对象。
func parsedService(t *testing.T, project map[string]any, name string) map[string]any {
	t.Helper()
	services, ok := project["services"].(map[string]any)
	if !ok {
		t.Fatal("解析结果里没有 services 对象")
	}
	svc, ok := services[name].(map[string]any)
	if !ok {
		t.Fatalf("解析结果里没有 %s 服务", name)
	}
	return svc
}

// parsedServiceEnv 取服务解析后的 environment：compose 规范成「键 → 值」对象，值为 null 表示从宿主环境透传。
func parsedServiceEnv(t *testing.T, svc map[string]any, name string) map[string]any {
	t.Helper()
	env, ok := svc["environment"].(map[string]any)
	if !ok {
		t.Fatalf("%s 服务解析后的 environment 不是对象（实际 %T）", name, svc["environment"])
	}
	return env
}

// TestComposeConfig_RejectsConfigLibrarySyntax 反证：确认这条校验真的能抓到插值语法错误，
// 而不是对任何文件都返回成功。
func TestComposeConfig_RejectsConfigLibrarySyntax(t *testing.T) {
	checker := newComposeChecker(t)
	bad := filepath.Join(t.TempDir(), "docker-compose.yml")
	content := "services:\n  app:\n    image: busybox:1.36\n    environment:\n      FLAG: ${FLAG:false}\n"
	if err := os.WriteFile(bad, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := checker.config(t, bad)
	if err == nil {
		t.Fatal("${FLAG:false} 不是合法的 compose 插值，docker compose config 应拒绝")
	}
	if !strings.Contains(out, "invalid interpolation format") {
		t.Errorf("应因插值格式被拒，实际输出:\n%s", out)
	}
}

// TestComposeConfig_GeneratedCombinations 仅必选、典型（grpc + opentelemetry）、全选三种组合
// 生成的 docker-compose.yml 都能被 docker compose 解析。compose 只依赖模板渲染，不需要 tidy / wire。
// 未选 milvus 的组合不带任何外置 S3 变量也必须能解析（不残留 S3 必填项）。
func TestComposeConfig_GeneratedCombinations(t *testing.T) {
	checker := newComposeChecker(t)
	cases := []struct {
		name     string
		dirName  string
		project  string
		optional []string
	}{
		{"只选必选组件", "taurusx-compose-required", "compose-required", nil},
		{"必选 + grpc + opentelemetry", "taurusx-compose-partial", "compose-partial", []string{"grpc", "opentelemetry"}},
		{"全部组件", "taurusx-compose-all", "compose-all", components.OptionalNames()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := filepath.Join(tmpRoot, tc.dirName)
			if err := os.RemoveAll(outDir); err != nil {
				t.Fatalf("清理输出目录失败: %v", err)
			}
			var log bytes.Buffer
			if _, err := Generate(Options{
				ProjectName: tc.project,
				Module:      "example.com/" + tc.project,
				OutputDir:   outDir,
				Components:  tc.optional,
				Stdout:      &log,
			}); err != nil {
				t.Fatalf("生成失败: %v\n%s", err, log.String())
			}
			use := checker
			if !contains(tc.optional, "milvus") {
				use = checker.withoutS3()
			}
			if out, err := use.config(t, filepath.Join(outDir, "docker-compose.yml")); err != nil {
				t.Fatalf("docker compose config 拒绝了生成的 docker-compose.yml: %v\n%s", err, out)
			}
		})
	}
}

// TestComposeConfig_MilvusRequiresExternalS3 勾选 milvus 时外置 S3 的每个必填项缺失或为空，
// docker compose 都必须拒绝，并在错误里点名该变量；JWT_SECRET 等其它必填项齐全，排除别的原因。
func TestComposeConfig_MilvusRequiresExternalS3(t *testing.T) {
	checker := newComposeChecker(t)
	outDir := generateDocsOnly(t, "taurusx-compose-s3-required", "compose-s3-required", []string{"milvus"})
	file := filepath.Join(outDir, "docker-compose.yml")
	if out, err := checker.config(t, file); err != nil {
		t.Fatalf("外置 S3 必填项齐全时 docker compose config 应通过: %v\n%s", err, out)
	}
	assertParsedMilvusS3(t, checker.configJSON(t, file))
	for _, v := range composeSyntheticS3 {
		missing := checker.without(v.name)
		empty := checker.without(v.name)
		empty.env = append(empty.env, v.name+"=")
		for label, c := range map[string]*composeChecker{"缺失": missing, "为空": empty} {
			out, err := c.config(t, file)
			if err == nil {
				t.Errorf("%s %s 时 docker compose config 应拒绝", v.name, label)
				continue
			}
			if !strings.Contains(out, v.name) {
				t.Errorf("%s %s 时应因该变量被拒，实际输出:\n%s", v.name, label, out)
			}
		}
	}
}

// composeS3Target 外置 S3 变量插值到 milvus 服务的哪个 MINIO_* 键。
var composeS3Target = map[string]string{
	"MILVUS_S3_ADDRESS":           "MINIO_ADDRESS",
	"MILVUS_S3_ACCESS_KEY_ID":     "MINIO_ACCESS_KEY_ID",
	"MILVUS_S3_SECRET_ACCESS_KEY": "MINIO_SECRET_ACCESS_KEY",
	"MILVUS_S3_BUCKET":            "MINIO_BUCKET_NAME",
	"MILVUS_S3_ROOT_PATH":         "MINIO_ROOT_PATH",
	"MILVUS_S3_REGION":            "MINIO_REGION",
}

// assertParsedMilvusS3 校验 docker compose 插值、规范化之后的真实取值，而不是模板源文本：
//   - milvus 的固定安全键存在、是 JSON 字符串且取值正确。尤其 MINIO_SSL_TLSCACERT 必须是存在的空字符串：
//     缺失时镜像自带配置里的非空 CA 占位生效，null 表示从宿主环境透传，两者都会绕开「CA 显式置空」；
//   - 外置 S3 必填项确实插值成了合成值；
//   - app 拿不到任何外置 S3 的键，也拿不到任何合成值。
//
// 不匹配时只报键名和类型；固定安全键不是密钥，另报实际值；外置 S3 的取值一律不回显。
func assertParsedMilvusS3(t *testing.T, project map[string]any) {
	t.Helper()
	milvusEnv := parsedServiceEnv(t, parsedService(t, project, "milvus"), "milvus")
	for _, want := range []struct{ key, value string }{
		{"MINIO_SSL_TLSCACERT", ""}, {"MINIO_USE_SSL", "true"}, {"MINIO_PORT", "443"},
		{"MINIO_CLOUD_PROVIDER", "aws"}, {"MINIO_USE_IAM", "false"}, {"COMMON_STORAGETYPE", "remote"},
		{"MINIO_USE_VIRTUAL_HOST", "false"}, {"MINIO_REQUEST_TIMEOUT_MS", "10000"},
	} {
		raw, ok := milvusEnv[want.key]
		if !ok {
			t.Errorf("解析后 milvus.environment 缺少 %s", want.key)
			continue
		}
		got, isString := raw.(string)
		if !isString {
			t.Errorf("解析后 milvus.environment.%s 应为 JSON 字符串，实际 %T（null 会从宿主环境透传）", want.key, raw)
			continue
		}
		if got != want.value {
			t.Errorf("解析后 milvus.environment.%s 应为 %q，实际 %q", want.key, want.value, got)
		}
	}
	for _, v := range composeSyntheticS3 {
		key := composeS3Target[v.name]
		raw, ok := milvusEnv[key]
		if !ok {
			t.Errorf("解析后 milvus.environment 缺少 %s（来自 %s）", key, v.name)
			continue
		}
		if got, isString := raw.(string); !isString || got != v.value {
			t.Errorf("解析后 milvus.environment.%s 不是 %s 的合成值（类型 %T，取值不回显）", key, v.name, raw)
		}
	}

	app := parsedService(t, project, "app")
	for key := range parsedServiceEnv(t, app, "app") {
		if strings.HasPrefix(key, "MILVUS_S3_") || strings.HasPrefix(key, "MINIO_") {
			t.Errorf("外置 S3 变量只给 milvus，解析后 app.environment 却有 %s", key)
		}
	}
	appJSON, err := json.Marshal(app)
	if err != nil {
		t.Fatal("重新编码解析后的 app 服务失败（内容不回显）")
	}
	for _, v := range composeSyntheticS3 {
		if bytes.Contains(appJSON, []byte(v.value)) {
			t.Errorf("解析后 app 服务里出现了 %s 的合成值（取值不回显）", v.name)
		}
	}
}
