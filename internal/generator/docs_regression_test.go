package generator

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 文档回归：生成项目里给用户看的处置步骤与限制说明必须完整、准确，不能做无条件承诺。

// credentialLimitNeedles 凭据「首尾空格不被去掉」说法必须带上的限制说明（配置库先文本替换再解析）。
var credentialLimitNeedles = []string{"不保证任意字符都能原样传递", "只验证过由进程环境变量直接提供的普通值", "未验证"}

// unconditionalCredentialClaims 不带限制的「原样」承诺，以前的写法，不应再出现。
var unconditionalCredentialClaims = []string{
	"真实值原样使用", "真实密码原样使用", "首尾空格也保留", "首尾空格不会被去掉", "首尾空格保留）",
}

// quoteAdvice 不能建议靠加引号保留空格：.env 类文件的引号规则本轮未验证，现有文档还明确要求值不加引号
// （前面紧跟「不」的「不要加引号」不算建议）。
var quoteAdvice = []string{"需加引号", "要加引号", "加上引号", "加引号即可", "加引号保留"}

// generateDocsOnly 只渲染模板（不 tidy / wire），文档与配置不依赖 Go 依赖解析。
func generateDocsOnly(t *testing.T, dirName, project string, optional []string) string {
	t.Helper()
	outDir := filepath.Join(tmpRoot, dirName)
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatalf("清理输出目录失败: %v", err)
	}
	var log bytes.Buffer
	if _, err := Generate(Options{
		ProjectName: project,
		Module:      "example.com/" + project,
		OutputDir:   outDir,
		Components:  optional,
		Stdout:      &log,
	}); err != nil {
		t.Fatalf("生成失败: %v\n%s", err, log.String())
	}
	return outDir
}

// assertCredentialWording 文件里一旦提到凭据首尾空格，就必须同时写明限制；不得出现无条件承诺或加引号建议。
func assertCredentialWording(t *testing.T, name, content string) {
	t.Helper()
	for _, claim := range unconditionalCredentialClaims {
		if strings.Contains(content, claim) {
			t.Errorf("%s 出现无条件的原样承诺 %q", name, claim)
		}
	}
	for _, advice := range quoteAdvice {
		// 现有文档的「不要加引号」是正确的反向要求，不算建议
		if strings.Count(content, advice) > strings.Count(content, "不"+advice) {
			t.Errorf("%s 建议加引号（%q）：.env 类文件的引号规则未验证，不能这样承诺", name, advice)
		}
	}
	if !strings.Contains(content, "首尾空格") {
		return
	}
	for _, needle := range credentialLimitNeedles {
		if !strings.Contains(content, needle) {
			t.Errorf("%s 提到凭据首尾空格，但缺少限制说明 %q", name, needle)
		}
	}
}

// TestDocs_CredentialWordingHasLimits 根 README 与各组合生成的 README / 配置注释：凭据「原样」说法都带限制。
func TestDocs_CredentialWordingHasLimits(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertCredentialWording(t, "README.md（taurus-x）", string(root))
	if !strings.Contains(string(root), "首尾空格") {
		t.Error("taurus-x README 应说明凭据首尾空格的处理及限制")
	}

	cases := []struct {
		dirName, project string
		optional         []string
	}{
		{"taurusx-docs-required", "docs-required", nil},
		{"taurusx-docs-partial", "docs-partial", []string{"grpc", "opentelemetry"}},
		{"taurusx-docs-all", "docs-all", components.OptionalNames()},
	}
	for _, tc := range cases {
		t.Run(tc.dirName, func(t *testing.T) {
			outDir := generateDocsOnly(t, tc.dirName, tc.project, tc.optional)
			files := []string{
				"README.md",
				filepath.Join("docs", "部署文档.md"),
				filepath.Join("backend", "config", "autoload", "redis", "redis.toml"),
			}
			if contains(tc.optional, "consul") {
				files = append(files, filepath.Join("backend", "config", "autoload", "consul", "consul.yaml"))
			}
			if contains(tc.optional, "milvus") {
				files = append(files, filepath.Join("backend", "config", "autoload", "milvus", "milvus.yaml"))
			}
			for _, rel := range files {
				assertCredentialWording(t, rel, readFile(t, filepath.Join(outDir, rel)))
			}
			// 每个组合都有 Redis，配置注释必须写明限制
			redis := readFile(t, filepath.Join(outDir, "backend", "config", "autoload", "redis", "redis.toml"))
			if !strings.Contains(redis, "首尾空格") {
				t.Error("redis.toml 应说明密码首尾空格的处理及限制")
			}
		})
	}
}

// milvusDisableSteps 部署文档里「Docker 方式暂时不用 Milvus」必须给出的完整三步。
var milvusDisableSteps = []string{
	"把 app 的 `MILVUS_ENABLE` 改成 `\"false\"`",
	"删掉 app 的 `depends_on` 里的 `milvus` 一项",
	"删除（或整段注释掉）`milvus`、`etcd` 两个服务",
	"只做前两步不够",
}

// TestDocs_MilvusDisableStepsComplete 全选时部署文档给出完整的三步处置；未选 milvus 的组合不残留这段说明。
// 并按文档步骤真实改一份全选 compose，用 docker compose 确认改完不带任何外置 S3 变量也能解析、且不再包含这两个服务。
func TestDocs_MilvusDisableStepsComplete(t *testing.T) {
	for _, tc := range []struct {
		dirName, project string
		optional         []string
	}{
		{"taurusx-docs-required", "docs-required", nil},
		{"taurusx-docs-partial", "docs-partial", []string{"grpc", "opentelemetry"}},
	} {
		outDir := generateDocsOnly(t, tc.dirName, tc.project, tc.optional)
		doc := readFile(t, filepath.Join(outDir, "docs", "部署文档.md"))
		readme := readFile(t, filepath.Join(outDir, "README.md"))
		for _, residue := range append(append([]string{}, milvusDisableSteps...), "MILVUS_ENABLE", "minio", "MinIO", "MINIO_", "MILVUS_S3_", "外置 S3", "etcd") {
			if strings.Contains(doc, residue) || strings.Contains(readme, residue) {
				t.Errorf("%s 未选 milvus，文档却残留 %q", tc.dirName, residue)
			}
		}
	}

	outDir := generateDocsOnly(t, "taurusx-docs-all", "docs-all", components.OptionalNames())
	doc := readFile(t, filepath.Join(outDir, "docs", "部署文档.md"))
	for _, step := range milvusDisableSteps {
		if !strings.Contains(doc, step) {
			t.Errorf("全选部署文档缺少「暂时不用 Milvus」的步骤 %q", step)
		}
	}

	// 按文档三步改 compose，真实解析；去掉 Milvus 后不应再需要任何外置 S3 变量
	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	edited := applyMilvusDisableSteps(t, compose)
	for _, gone := range []string{"MILVUS_S3_", "MINIO_", "COMMON_STORAGETYPE"} {
		if strings.Contains(edited, gone) {
			t.Errorf("按文档处置后 compose 仍残留 %q", gone)
		}
	}
	checker := newComposeChecker(t).withoutS3()
	file := filepath.Join(outDir, "docker-compose.milvus-disabled.yml")
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := checker.config(t, file); err != nil {
		t.Fatalf("按文档去掉 Milvus 后 docker compose config 失败: %v\n%s", err, out)
	}
	services := checker.services(t, file)
	for _, gone := range []string{"milvus", "etcd"} {
		if contains(services, gone) {
			t.Errorf("按文档处置后 compose 仍包含服务 %s（实际 %v）", gone, services)
		}
	}
	for _, kept := range []string{"mysql", "redis", "app"} {
		if !contains(services, kept) {
			t.Errorf("按文档处置后 compose 缺少服务 %s（实际 %v）", kept, services)
		}
	}
}

// applyMilvusDisableSteps 按部署文档的三步改 compose 文本：MILVUS_ENABLE 改 false、去掉 app 对 milvus 的依赖、
// 删除 milvus / etcd 两个服务及它们的卷声明。任何一步找不到要改的内容都直接失败（说明模板变了）。
func applyMilvusDisableSteps(t *testing.T, compose string) string {
	t.Helper()
	replace := func(old, new string) {
		t.Helper()
		if strings.Count(compose, old) != 1 {
			t.Fatalf("compose 里应恰好有一处 %q", old)
		}
		compose = strings.Replace(compose, old, new, 1)
	}
	replace(`MILVUS_ENABLE: "true"`, `MILVUS_ENABLE: "false"`)
	replace("      milvus:\n        condition: service_healthy\n", "")

	start := strings.Index(compose, "\n  # ---- Milvus standalone")
	end := strings.Index(compose, "\n  app:\n")
	if start < 0 || end < start {
		t.Fatal("compose 里找不到 milvus / etcd 服务段")
	}
	compose = compose[:start] + "\n" + compose[end:]
	for _, v := range []string{"  etcd-data:\n", "  milvus-data:\n"} {
		replace(v, "")
	}
	return compose
}

// services 执行 `docker compose --env-file /dev/null -f <file> config --services`，只输出服务名，不含环境取值。
func (c *composeChecker) services(t *testing.T, composeFile string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.docker, "compose", "--env-file", "/dev/null", "-f", composeFile, "config", "--services")
	cmd.Dir = filepath.Dir(composeFile)
	cmd.Env = c.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker compose config --services 失败: %v\n%s", err, redact(stderr.String()))
	}
	return strings.Fields(stdout.String())
}

// milvusS3PrecedenceNeedles 部署文档第二节和运维手册第五节都必须写明、且口径一致的 Milvus 配置来源说明：
// 三层优先级、MILVUS_CONF_* 同层别名、静态环境变量与 etcd 运行时覆盖的区别、非空 CA 后的处置、防范边界与 NOT_RUN。
var milvusS3PrecedenceNeedles = []string{
	// 三层优先级：etcd 更高；user.yaml / MILVUSCONF 低于环境变量，禁止它们只是防御性约束
	"**etcd > 环境变量 > 配置文件**",
	"etcd 的优先级高于环境变量，能直接盖掉 compose 里写死的值",
	"`user.yaml` 和 `MILVUSCONF`（改配置目录）属于配置文件层，优先级低于环境变量",
	"防御性约束", "不是因为它们优先级更高",
	// MILVUS_CONF_* 与普通环境变量同层，冲突时谁赢不确定
	"`MILVUS_CONF_MINIO_*` 这类 `MILVUS_CONF_*` 前缀变量与普通环境变量同属环境变量层，是同一配置项的别名",
	"谁生效取决于进程环境变量的遍历顺序，不能保证谁赢",
	// 静态环境变量只在启动时读；etcd 覆盖运行中生效，影响之后的任务
	"compose 里的环境变量（含 `.env` 插值后的值）只在 Milvus 进程启动时读取",
	"etcd 里的覆盖在运行中就会生效，不需要重启",
	"索引构建、统计、分析、导入、压缩（compaction）任务拿到的存储配置（StorageConfig）",
	// 非空 CA：可能关闭校验并污染进程状态，只清 etcd 不够，必须清覆盖后重启整个进程
	"C++ 存储客户端可能对之后的任务关闭证书和主机名校验",
	"存储 V2 还会把 CA 写进进程级状态，只删掉 etcd 里的覆盖不能保证恢复校验",
	"先删掉 etcd 里的这条覆盖，再重启整个 Milvus 进程",
	// 防范边界：固定环境变量防不了 etcd 写者 / 同网失陷，本项目没改网络和认证
	"防不了能写 etcd 的人", "也防不了同一容器网络里已被攻破的容器", "本项目没有改这部分网络和认证配置",
	"只按 v2.6.5 源码分析，没有做动态验证（NOT_RUN）",
}

// staleMilvusS3Wording 两条旧的错误说法：把「只在启动时读取」套到所有配置来源（etcd 覆盖并非如此）；
// 把 user.yaml / MILVUSCONF / MILVUS_CONF_* 归为「更高优先级」的来源。
var staleMilvusS3Wording = []string{
	"这些配置只在启动时读取", "Milvus 只在启动时读这些配置", "改配置要重建容器",
	"更高优先级的来源", "更高优先级的配置",
}

// TestDocs_MilvusS3ConfigSources 勾选 milvus 时运维手册有第五节，部署文档第二节与运维手册第五节对 Milvus 配置来源的
// 优先级、生效时机、非空 CA 处置和边界说法一致，旧的错误说法不复发；未选 milvus 的运维手册不残留第五节。
func TestDocs_MilvusS3ConfigSources(t *testing.T) {
	for _, tc := range []struct {
		dirName, project string
		optional         []string
	}{
		{"taurusx-docs-s3-required", "docs-s3-required", nil},
		{"taurusx-docs-s3-partial", "docs-s3-partial", []string{"grpc", "opentelemetry"}},
	} {
		outDir := generateDocsOnly(t, tc.dirName, tc.project, tc.optional)
		manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
		for _, residue := range []string{"五、Milvus", "第五节", "Milvus", "外置 S3", "MILVUS_S3_", "MINIO_", "etcd", "StorageConfig"} {
			if strings.Contains(manual, residue) {
				t.Errorf("%s 未选 milvus，运维手册却残留 %q", tc.dirName, residue)
			}
		}
	}

	outDir := generateDocsOnly(t, "taurusx-docs-s3-all", "docs-s3-all", components.OptionalNames())
	manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
	if !strings.Contains(manual, "另附 Milvus 外置 S3 的运维要点（第五节）。") {
		t.Error("运维手册开头应指向 Milvus 外置 S3 的第五节")
	}
	_, opsS3, found := strings.Cut(manual, "\n## 五、Milvus 外置 S3\n")
	if !found {
		t.Fatal("勾选 milvus 时运维手册缺少「五、Milvus 外置 S3」一节")
	}
	// 第五节原有的轮换、迁移、备份与未验收边界保持
	for _, want := range []string{"**轮换访问密钥**", "再吊销旧密钥", "**不要动这几项**", "`MINIO_SSL_TLSCACERT: \"\"`",
		"**不支持私有 CA / 自签证书**", "**不要改桶和前缀**", "换端点、桶或前缀属于数据迁移",
		"**备份**：Milvus 的数据 = etcd 元数据 + 对象存储里该前缀下的对象", "没有对真实外置端点做过备份恢复验证"} {
		if !strings.Contains(opsS3, want) {
			t.Errorf("运维手册第五节缺少 %q", want)
		}
	}

	deploy := readFile(t, filepath.Join(outDir, "docs", "部署文档.md"))
	_, deployS3, found := strings.Cut(deploy, "Milvus 的对象存储必须接外置 S3（生产前置条件）")
	if !found {
		t.Fatal("部署文档缺少 Milvus 外置 S3 说明")
	}
	deployS3, _, _ = strings.Cut(deployS3, "\n## 三、")
	if !strings.Contains(deployS3, "状态 NOT_RUN") {
		t.Error("部署文档的外置 S3 上线前验收应标明状态 NOT_RUN")
	}

	for name, section := range map[string]string{"部署文档第二节": deployS3, "运维手册第五节": opsS3} {
		for _, want := range milvusS3PrecedenceNeedles {
			if !strings.Contains(section, want) {
				t.Errorf("%s 缺少 Milvus 配置来源说明 %q", name, want)
			}
		}
	}
	for name, doc := range map[string]string{"部署文档": deploy, "运维手册": manual} {
		for _, stale := range staleMilvusS3Wording {
			if strings.Contains(doc, stale) {
				t.Errorf("%s 仍有旧的错误说法 %q", name, stale)
			}
		}
	}
}
