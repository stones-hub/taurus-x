package generator

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stones-hub/taurus-x/internal/components"
)

// 文档回归：生成项目里给用户看的处置步骤与限制说明必须完整、准确，不能做无条件承诺。
// 生成项目只有一个日常入口 README.md（原 docs/部署文档.md 已并入并删除）；docs/运维手册.md 只放进阶内容。

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

// markdownSection 取 heading（含前后换行，例如 "\n## 八、备份与恢复\n"）之后、下一个二级标题之前的正文。
func markdownSection(t *testing.T, doc, heading string) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, heading)
	if !found {
		t.Fatalf("文档缺少章节 %q", strings.TrimSpace(heading))
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	return section
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

// TestDocs_CredentialWordingHasLimits 根 README 与各组合生成的 README / 运维手册 / 配置注释：凭据「原样」说法都带限制。
// 凭据首尾空格的说明面向生成项目的使用者，权威位置是生成项目 README（根 README 只讲脚手架本身，不再重复）。
func TestDocs_CredentialWordingHasLimits(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertCredentialWording(t, "README.md（taurus-x）", string(root))

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
				filepath.Join("docs", "运维手册.md"),
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
			// 每个组合的生成 README 都必须说明凭据首尾空格的处理及限制
			if !strings.Contains(readFile(t, filepath.Join(outDir, "README.md")), "首尾空格") {
				t.Error("生成项目 README 应说明凭据首尾空格的处理及限制")
			}
			// 每个组合都有 Redis，配置注释必须写明限制
			redis := readFile(t, filepath.Join(outDir, "backend", "config", "autoload", "redis", "redis.toml"))
			if !strings.Contains(redis, "首尾空格") {
				t.Error("redis.toml 应说明密码首尾空格的处理及限制")
			}
		})
	}
}

// milvusDisableSteps 生成 README「6.5 Milvus 外置 S3」里「Docker 方式暂时不用 Milvus」必须给出的完整三步。
var milvusDisableSteps = []string{
	"把 app 的 `MILVUS_ENABLE` 改成 `\"false\"`",
	"删掉 app 的 `depends_on` 里的 `milvus` 一项",
	"删除（或整段注释掉）`milvus`、`etcd` 两个服务",
	"只做前两步不够",
}

// TestDocs_MilvusDisableStepsComplete 全选时 README 给出完整的三步处置；未选 milvus 的组合（README 与运维手册）不残留这段说明。
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
		manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
		readme := readFile(t, filepath.Join(outDir, "README.md"))
		for _, residue := range append(append([]string{}, milvusDisableSteps...), "MILVUS_ENABLE", "minio", "MinIO", "MINIO_", "MILVUS_S3_", "外置 S3", "etcd") {
			if strings.Contains(manual, residue) || strings.Contains(readme, residue) {
				t.Errorf("%s 未选 milvus，文档却残留 %q", tc.dirName, residue)
			}
		}
	}

	outDir := generateDocsOnly(t, "taurusx-docs-all", "docs-all", components.OptionalNames())
	s3 := markdownSection(t, readFile(t, filepath.Join(outDir, "README.md")), readmeMilvusS3Heading)
	for _, step := range milvusDisableSteps {
		if !strings.Contains(s3, step) {
			t.Errorf("全选 README「6.5 Milvus 外置 S3」缺少「暂时不用 Milvus」的步骤 %q", step)
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

// applyMilvusDisableSteps 按 README 的三步改 compose 文本：MILVUS_ENABLE 改 false、去掉 app 对 milvus 的依赖、
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

// readmeMilvusS3Heading 生成 README 里 Milvus 外置 S3 的唯一说明（只在勾选 milvus 时生成，原部署文档第二节与运维手册第五节合并于此）。
const readmeMilvusS3Heading = "\n### 6.5 Milvus 外置 S3\n"

// milvusS3PrecedenceNeedles README「6.5 Milvus 外置 S3」必须写明的 Milvus 配置来源说明：
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

// TestDocs_MilvusS3ConfigSources 勾选 milvus 时 README「6.5 Milvus 外置 S3」是外置 S3 配置来源、日常运维与验收的唯一说明，
// 优先级、生效时机、非空 CA 处置和边界说法齐全，旧的错误说法不复发；运维手册不再重复这段（不会出现两份口径）；
// 未选 milvus 的 README 与运维手册不残留任何 Milvus / 外置 S3 术语。
func TestDocs_MilvusS3ConfigSources(t *testing.T) {
	for _, tc := range []struct {
		dirName, project string
		optional         []string
	}{
		{"taurusx-docs-s3-required", "docs-s3-required", nil},
		{"taurusx-docs-s3-partial", "docs-s3-partial", []string{"grpc", "opentelemetry"}},
	} {
		outDir := generateDocsOnly(t, tc.dirName, tc.project, tc.optional)
		for _, rel := range []string{"README.md", filepath.Join("docs", "运维手册.md")} {
			doc := readFile(t, filepath.Join(outDir, rel))
			for _, residue := range []string{"五、Milvus", "第五节", "Milvus", "外置 S3", "MILVUS_S3_", "MINIO_", "etcd", "StorageConfig"} {
				if strings.Contains(doc, residue) {
					t.Errorf("%s 未选 milvus，%s 却残留 %q", tc.dirName, rel, residue)
				}
			}
		}
	}

	outDir := generateDocsOnly(t, "taurusx-docs-s3-all", "docs-s3-all", components.OptionalNames())
	readme := readFile(t, filepath.Join(outDir, "README.md"))
	if !strings.Contains(readme, "[6.5 Milvus 外置 S3](#65-milvus-外置-s3)") {
		t.Error("README 开头的组件说明应指向「6.5 Milvus 外置 S3」")
	}
	s3 := markdownSection(t, readme, readmeMilvusS3Heading)
	if !strings.Contains(s3, "状态 NOT_RUN") {
		t.Error("README 的外置 S3 上线前验收应标明状态 NOT_RUN")
	}
	for _, want := range milvusS3PrecedenceNeedles {
		if !strings.Contains(s3, want) {
			t.Errorf("README「6.5 Milvus 外置 S3」缺少 Milvus 配置来源说明 %q", want)
		}
	}
	// 原运维手册第五节的轮换、迁移、备份与未验收边界并入 README 后保持
	for _, want := range []string{"**轮换访问密钥**", "再吊销旧密钥", "**不要动这几项**", "`MINIO_SSL_TLSCACERT: \"\"`",
		"**不支持私有 CA / 自签证书**", "**不要改桶和前缀**", "换端点、桶或前缀属于数据迁移",
		"**备份**：Milvus 的数据 = etcd 元数据 + 对象存储里该前缀下的对象", "没有对真实外置端点做过备份恢复验证"} {
		if !strings.Contains(s3, want) {
			t.Errorf("README「6.5 Milvus 外置 S3」缺少 %q", want)
		}
	}

	manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
	// 运维手册只保留查询失败日志判读，外置 S3 的配置来源 / 运维不再重复出一份
	for _, dup := range []string{"五、Milvus 外置 S3", "MILVUS_S3_", "StorageConfig", "etcd > 环境变量 > 配置文件", "轮换访问密钥"} {
		if strings.Contains(manual, dup) {
			t.Errorf("运维手册不应再重复 Milvus 外置 S3 的说明（命中 %q），唯一说明在 README", dup)
		}
	}
	for name, doc := range map[string]string{"README": readme, "运维手册": manual} {
		for _, stale := range staleMilvusS3Wording {
			if strings.Contains(doc, stale) {
				t.Errorf("%s 仍有旧的错误说法 %q", name, stale)
			}
		}
	}
}

// docsCombos 文档轻量回归的三种组合：只选必选、典型（grpc + opentelemetry）、全选。只渲染不 tidy / Wire。
var docsCombos = []struct {
	dirName, project string
	optional         []string
}{
	{"taurusx-docs-ops-required", "docs-ops-required", nil},
	{"taurusx-docs-ops-partial", "docs-ops-partial", []string{"grpc", "opentelemetry"}},
	{"taurusx-docs-ops-all", "docs-ops-all", components.OptionalNames()},
}

// generatedReadmeHeadings 生成 README 作为唯一日常入口必须具备的操作章节（本地开发一条直接流程、Docker 推荐部署、原生部署要点）。
var generatedReadmeHeadings = []string{
	"\n## 二、准备环境\n",
	"\n## 三、配置\n", "\n### 3.3 写法限制与文件权限\n", "\n### 3.4 HTTPS、反向代理与可信来源\n",
	"\n## 四、首次初始化\n", "\n### 4.1 数据库只初始化一次\n", "\n### 4.2 初始管理员\n", "\n### 4.3 忘记初始密码 / 重置管理员密码\n",
	"\n## 五、本机开发\n", "\n### 5.2 配置与启动\n",
	"\n## 六、部署\n", "\n### 6.1 Docker Compose 部署（推荐）\n", "\n### 6.2 原生部署（要点）\n",
	"\n### 6.3 反向代理、HTTPS 与按 IP 限制后台访问\n", "\n### 6.4 部署后验证\n",
	"\n## 七、日常运维：状态、日志、停止\n",
	"\n## 八、备份与恢复\n", "\n### 8.4 Docker 部署：连到 compose 里的 MySQL\n", "\n### 8.5 备份\n", "\n### 8.7 恢复\n",
	"\n### 8.8 恢复演练（恢复到新的空库，不删任何库）\n", "\n### 8.9 正式恢复（生产数据损坏或误删时）\n",
	"\n## 九、更新与失败处理\n", "\n### 9.2 每次更新都走的门禁\n", "\n### 9.4 执行增量 SQL\n", "\n### 9.5 失败了怎么办\n",
	"\n## 十、常见问题\n",
	"\n## 十一、已知风险与待办\n", "\n### 安全\n", "\n### 优雅退出\n", "\n### 行为约定（",
}

// generatedReadmeContract 生成 README 必须保留的操作契约（按实际脚本 / Makefile / compose 写的命令与边界）。
func generatedReadmeContract(project string) []string {
	return []string{
		// 初始化只对新库 / 首次空卷，更新不重新导入 init.sql；代码与数据库回退分开
		"**它只用于全新的空库**", "**更新不是重新导入 `init.sql`**", "**没有自动迁移，也没有一键回滚**",
		"**代码回退与数据库回退是两件事**", "换回旧程序（**不会**回滚数据库，也不会自动还原数据）",
		// 重置管理员密码：交互式 mysql、关历史、事务 + auth_version，散列不放 shell 参数
		"(cd backend && go run ./bin/hashpw)", "MYSQL_HISTFILE=/dev/null", "START TRANSACTION;",
		"UPDATE admin_users SET auth_version = auth_version + 1", "COMMIT;", "ROLLBACK;",
		"shell 仍会把 `$2`、`$10` 这类片段当成变量展开",
		// Docker：compose .env 用编辑器写、MySQL 初始密码只在首次初始化生效、只替换 app；
		// 回滚标签取自正在运行的容器的镜像 ID，不假定 latest 就是线上
		"chmod 600 .env", "只在 MySQL 数据卷**第一次初始化**时生效", "docker compose up -d --no-deps app",
		"docker inspect " + project + "-app | grep '\"Image\"'",
		"docker tag <上一条输出里 sha256: 开头的镜像 ID> " + project + "-app:prev",
		"不要假定 `" + project + "-app:latest` 就是线上镜像",
		"docker tag " + project + "-app:prev " + project + "-app:latest",
		"**`docker compose down -v` 会删除数据卷，等于删库**", "不加 `-v` 也不等于数据永远安全",
		// 更新 / 回退：代码、前端、config、运行配置秘密配套留副本，私有目录保管，不覆盖秘密；停止后再替换
		"回滚副本含运行配置里的秘密，统一放在**只有你可读**的目录", "**不覆盖已有的密码和密钥**",
		"cp -p /data/app/.env ~/" + project + "-rollback/<旧版本>/env",
		"cp -a /data/app/backend/build/" + project + " /data/app/backend/config /data/app/frontend/dist ~/" + project + "-rollback/<旧版本>/",
		"**独立的构建目录**", "停止后再替换，不写运行中的文件", "`config/` 不要整目录覆盖",
		"先确认旧程序和旧配置能在**当前**表结构上运行", "代码、前端、配置要配套回退",
		// 备份恢复：Make 与脚本参数的真实差异；密码在备份 / 恢复全部结束后才 unset
		"`make db-backup` 只接受 `DB`", "scripts/db/backup.sh --database", "--output-dir",
		"`--skip-checksum` 只能直接调脚本时使用", "只有恰好写 `CREATE=1` 才会传 `--create`",
		"一份备份是**三个文件**", "拷一份到**主机外**的可信存储",
		"**等这些命令全部结束之后**——无论成功、失败还是被 Ctrl+C 中断——再清掉密码",
		// Docker 下连 compose 的 MySQL：不发布端口、原生 Linux 查 bridge IP 并双向核对 server_uuid，Desktop 不照搬
		"**没有映射端口**", "`127.0.0.1:3306` **不是**这个容器", "docker inspect " + project + "-mysql | grep '\"IPAddress\"'",
		"两次的 `server_uuid` 必须一致", "**Docker Desktop（macOS / Windows）不能照搬**",
		// 演练 / 正式恢复：目标由库级对象决定（无：异名新空库；有：独立实例同名空库），后续主机端口库名随目标；
		// 先身份、权限、备份再切换，源数据不删
		"不需要删除任何库", "先恢复到**新库**、核对后再切换",
		"**异名新空库**", "只能是**另一个独立 MySQL 实例**上与源库**同名**的空库",
		"后面所有步骤的主机、端口、库名都用这个目标", "`DB_DSN` 的主机、端口、库名都指向目标",
		"把运行配置里 `DB_DSN` 的**主机、端口、库名**都改成实际目标", "用具备 8.5 备份权限的账号对恢复出的目标库做一次备份",
		"旧库与源数据保留取证，不在原库上覆盖、不删除",
		// 非 root、工作目录、权限、停止预算（原生部署按用户语义写要求，不再给逐平台守护配置）；按平台选运行用户
		"**不以 root 运行**", "**不要改成 root**", "**必须以 `backend/` 为工作目录**", "chmod 640 /data/app/backend/.env.local",
		"没有 `useradd`，不要照抄上一条", "`id -gn <运行用户>`", "sudo -u <运行用户> ./build/" + project,
		"**停止等待不少于 45 秒**", "`AUTH_TRUSTED_ORIGINS=https://实际后台域名`", "`JWT_COOKIE_SECURE=true`",
		"**不要填 `0.0.0.0/0` 之类的大网段**",
		// 不要求全局 pnpm
		"npm exec --yes pnpm@10.34.7 --", "**不要求全局安装 pnpm**",
	}
}

// staleOperationClaims 原部署文档 / README 里不真实或不安全的说法，不能复发。
var staleOperationClaims = []string{
	"只要不用 `-v`，数据永远安全", "数据永远安全。", "统一执行这一条即可", "`up -d app` 只重启 app",
	"shell 不会替换它", "-e \"UPDATE admin_user_logins", "cat > .env", "<<'EOF'",
	`ports: ["127.0.0.1:3306:3306"]`, "-p<密码>", "人工删测试库**，然后",
	// 正式恢复不能无条件只改库名（含库级对象时目标在另一实例）；原生回退不再靠运行中 cp -f 覆盖；
	// 运行用户不再写死 Linux 的 app（macOS 没有 useradd）
	"只把 `DB_DSN` 里的库名改成新库", "cp -f /data/app/backend/build/", "sudo -u app ", "chown \"$USER\":app ",
	// 回退不能先启动未回退的新程序、也不能用旧运行配置整体覆盖刚恢复的 DSN；FAQ 不泛建议换名；
	// Docker 容器内监听不能改回环
	"不能时先按 8.9 处理数据", "<旧版本>/env /data/app/.env", "<旧版本>/env.local /data/app/backend/.env.local",
	"恢复只进空库，换一个新库名", "让后端只监听回环（`SERVER_ADDRESS=127.0.0.1`，Docker",
}

// assertRecoveryAndStartupOrder 独立审计指出的操作顺序回归（只核关键语义串与先后，不逐字锁整章）：
//   - 重置密码：含两条 UPDATE 的 SQL 块不含 COMMIT，先看两条 1 row affected，异常 ROLLBACK，COMMIT 单独成块在最后；
//   - 监听地址：原生用 127.0.0.1，Docker 容器内保持 0.0.0.0、只把宿主端口绑回环；
//   - 本机开发两个长驻命令用子 shell，不留在子目录里；
//   - 8.9 区分只恢复数据与联合回退；联合回退保持停机、先回退程序且不启动、DSN 保留恢复目标、最后启动；
//     9.5 表格与 Docker / 原生步骤同一顺序；
//   - FAQ 按库级对象给恢复目标。
//
// 这些是文档契约，不替代真实的数据库恢复或回退演练。
func assertRecoveryAndStartupOrder(t *testing.T, readme, project string) {
	t.Helper()
	initSection := markdownSection(t, readme, "\n## 四、首次初始化\n")
	var sqlBlocks []string
	var cur *strings.Builder
	fencedBlocks(initSection, func(_ int, line, lang string, inFence bool) {
		switch {
		case inFence && lang == "sql":
			if cur == nil {
				cur = &strings.Builder{}
			}
			cur.WriteString(line + "\n")
		case !inFence && cur != nil:
			sqlBlocks = append(sqlBlocks, cur.String())
			cur = nil
		}
	})
	sawUpdates, sawCommitAlone := false, false
	for _, b := range sqlBlocks {
		if strings.Contains(b, "START TRANSACTION;") {
			sawUpdates = true
			if strings.Contains(b, "COMMIT;") {
				t.Error("重置密码：含 UPDATE 的 SQL 块不能带 COMMIT，整块粘贴就会在检查 1 row affected 之前提交")
			}
		}
		if strings.TrimSpace(b) == "COMMIT;" {
			sawCommitAlone = true
		}
	}
	if !sawUpdates || !sawCommitAlone {
		t.Errorf("重置密码：应有含 START TRANSACTION 的 SQL 块和单独的 COMMIT 块（实际 %d 个 SQL 块）", len(sqlBlocks))
	}
	assertOrder(t, "重置密码先检查再提交", initSection,
		"START TRANSACTION;", "UPDATE admin_users SET auth_version = auth_version + 1", "1 row affected",
		"在同一个 `mysql>` 会话里输入 `ROLLBACK;`", "\nCOMMIT;\n")

	for _, want := range []string{"原生部署设 `SERVER_ADDRESS=127.0.0.1`", "Docker 容器内保持 `SERVER_ADDRESS: 0.0.0.0`",
		"只把宿主端口映射改成 `127.0.0.1:9080:9080`",
		"(cd backend && make run)", "(cd frontend && npm exec --yes pnpm@10.34.7 -- dev --host 127.0.0.1)",
		"「库级对象：无」换一个异名新空库，「有」只能用另一个独立实例上的同名空库"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README 缺少 %q", want)
		}
	}
	fencedBlocks(readme, func(lineNo int, line, lang string, inFence bool) {
		code := strings.TrimSpace(line)
		if inFence && (lang == "bash" || lang == "") && (strings.HasPrefix(code, "cd backend &&") || strings.HasPrefix(code, "cd frontend &&")) {
			t.Errorf("README 第 %d 行进入子目录后不会回到项目根，后续相对路径会错，应改用子 shell: %s", lineNo, line)
		}
	})

	restoreSec := markdownSection(t, readme, "\n### 8.9 正式恢复（生产数据损坏或误删时）\n")
	assertOrder(t, "8.9 联合回退", restoreSec,
		"app 一直保持停止，配置全部改好之后才启动", "**只恢复数据**", "**数据恢复 + 版本回退**",
		"回退程序、前端与兼容配置，**不启动**", "保留目标的主机、端口、库名和有效凭据", "最后才启动",
		"不要用旧版本的 `.env` / `.env.local` 回滚副本整体覆盖")

	failure := markdownSection(t, readme, "\n### 9.5 失败了怎么办\n")
	assertOrder(t, "9.5 表格的联合回退", failure,
		"**联合回退**", "按 8.9 第 1–6 步", "回退程序和兼容配置（不启动）", "按 8.9 第 7 步「数据恢复 + 版本回退」改 `DB_DSN`", "最后启动")
	assertOrder(t, "9.5 Docker 回退保持停机、DSN 核对后最后启动", failure,
		"docker compose stop app", "git checkout <旧版本的提交>",
		"docker tag "+project+"-app:prev "+project+"-app:latest",
		"# 用编辑器核对 /data/app/.env：DB_DSN 指向当前要用的库", "docker compose up -d --no-deps app")
	assertOrder(t, "9.5 原生回退保持停机、DSN 保留后最后启动", failure,
		"**先停止** app", "只按项改回（同样保留当前 `DB_DSN`）", "最后用守护工具启动")
	if !strings.Contains(failure, "**不整体覆盖**") {
		t.Error("9.5 应说明运行配置只按项改回、不整体覆盖")
	}
}

// assertSecretLifecycle 同一个代码块里 `read -rs X` 之后紧接 `unset X`（中间没有真正使用它的命令）时，
// 照抄会在执行前就清掉密码：密码必须在使用它的命令全部结束后才 unset。
func assertSecretLifecycle(t *testing.T, name, doc string) {
	t.Helper()
	var block []string
	check := func() {
		for i, line := range block {
			m := regexp.MustCompile(`read -rs (\w+)`).FindStringSubmatch(line)
			if m == nil {
				continue
			}
			used := false
			for _, next := range block[i+1:] {
				next = strings.TrimSpace(next)
				if next == "unset "+m[1] {
					if !used {
						t.Errorf("%s 在同一代码块里读入 %s 后、没有任何使用它的命令就 unset：照抄会提前清掉密码", name, m[1])
					}
					break
				}
				if next != "" && !strings.HasPrefix(next, "#") && !strings.HasPrefix(next, "export ") && !strings.HasPrefix(next, "read ") {
					used = true
				}
			}
		}
		block = nil
	}
	fencedBlocks(doc, func(_ int, line, _ string, inFence bool) {
		if inFence {
			block = append(block, line)
		} else if block != nil {
			check()
		}
	})
	check()
}

// assertOrder 要求 needles 在 doc 里都出现且按给定顺序出现（用于回滚 / 更新的操作顺序）。
func assertOrder(t *testing.T, what, doc string, needles ...string) {
	t.Helper()
	last := -1
	for _, n := range needles {
		i := strings.Index(doc[last+1:], n)
		if i < 0 {
			t.Errorf("%s：在前一步之后找不到 %q（顺序不对或缺失）", what, n)
			return
		}
		last += 1 + i
	}
}

// assertUpdateRollbackOrder 更新先从运行中的容器镜像 ID 留回滚标签、备份运行配置，再切新版本；
// Docker 回退先切回旧源码、再打回旧镜像、最后只替换 app；原生先留副本、再替换，回退从副本恢复。
func assertUpdateRollbackOrder(t *testing.T, readme, project string) {
	t.Helper()
	update := markdownSection(t, readme, "\n## 九、更新与失败处理\n")
	assertOrder(t, "Docker 更新", update,
		"docker inspect "+project+"-app | grep '\"Image\"'",
		"docker tag <上一条输出里 sha256: 开头的镜像 ID> "+project+"-app:prev",
		"cp -p /data/app/.env ~/"+project+"-rollback/<旧版本>/env",
		"git checkout <要发布的 tag 或提交号>",
		"docker compose build app",
		"docker compose up -d --no-deps app")
	assertOrder(t, "原生更新", update,
		"cp -a /data/app/backend/build/"+project,
		"cp /data/app/backend/.env.local ~/"+project+"-rollback/<旧版本>/env.local",
		"cp ~/"+project+"-build/backend/build/"+project+" /data/app/backend/build/"+project)
	assertOrder(t, "Docker 回退", update,
		"git checkout <旧版本的提交>",
		"docker tag "+project+"-app:prev "+project+"-app:latest",
		"docker compose up -d --no-deps app")
	assertOrder(t, "原生回退", update,
		"**先停止** app，从回滚副本恢复",
		"cp ~/"+project+"-rollback/<旧版本>/"+project+" /data/app/backend/build/"+project)
	for _, bad := range []string{"docker tag " + project + "-app:latest " + project + "-app:prev"} {
		if strings.Contains(readme, bad) {
			t.Errorf("README 不应假定 latest 就是线上镜像：%q", bad)
		}
	}
}

// readmeLineBudget 生成 README 行数的回归护栏：防止再次把逐平台安装教程、实现机制与历史拼回日常入口（目标 600–900 行）。
const readmeLineBudget = 1000

// fencedBlocks 逐行遍历 Markdown，回调给出代码块的语言（不在代码块里时 inFence=false）。
func fencedBlocks(doc string, visit func(lineNo int, line, lang string, inFence bool)) (unclosed bool) {
	inFence, lang := false, ""
	for i, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence, lang = true, strings.TrimPrefix(trimmed, "```")
			} else {
				inFence, lang = false, ""
			}
			continue
		}
		visit(i+1, line, lang, inFence)
	}
	return inFence
}

// globalPnpmCall 直接调用全局 pnpm 的命令（npm exec --yes pnpm@版本 不算）。
var globalPnpmCall = regexp.MustCompile(`(^|[;&|(]\s*)pnpm\s`)

// assertCommandsSingleLine 文档里每条终端命令单行：任何代码块都不能用反斜杠续行；shell 代码块不能有 heredoc。
// checkPnpm 时 shell 代码块也不能直接调用全局 pnpm。
func assertCommandsSingleLine(t *testing.T, name, doc string, checkPnpm bool) {
	t.Helper()
	unclosed := fencedBlocks(doc, func(lineNo int, line, lang string, inFence bool) {
		if !inFence {
			return
		}
		if strings.HasSuffix(strings.TrimRight(line, " \t"), `\`) {
			t.Errorf("%s 第 %d 行用反斜杠续行: %s", name, lineNo, line)
		}
		shell := lang == "bash" || lang == "sh" || lang == "shell" || lang == "console" || lang == ""
		if shell && strings.Contains(line, "<<") {
			t.Errorf("%s 第 %d 行用了 heredoc，配置内容应通过编辑器写入: %s", name, lineNo, line)
		}
		if shell && checkPnpm && globalPnpmCall.MatchString(strings.TrimSpace(line)) {
			t.Errorf("%s 第 %d 行要求全局 pnpm，应改用 npm exec --yes pnpm@10.34.7 --: %s", name, lineNo, line)
		}
	})
	if unclosed {
		t.Errorf("%s 有未闭合的代码块", name)
	}
}

// markdownLink 行内链接 [文字](目标)。
var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// headingSlug 按 GitHub 的规则把标题转成锚点：小写，去掉字母 / 数字 / 空格 / - / _ 以外的字符，空格换成 -。
func headingSlug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// assertDocLinks 代码块以外的相对链接都能找到文件；同文档锚点都能对上某个标题。
func assertDocLinks(t *testing.T, path string) {
	t.Helper()
	doc := readFile(t, path)
	slugs := map[string]bool{}
	var prose strings.Builder
	fencedBlocks(doc, func(_ int, line, _ string, inFence bool) {
		if inFence {
			return
		}
		prose.WriteString(line + "\n")
		if trimmed := strings.TrimLeft(line, "#"); trimmed != line && strings.HasPrefix(trimmed, " ") {
			slugs[headingSlug(trimmed)] = true
		}
	})
	for _, m := range markdownLink.FindAllStringSubmatch(prose.String(), -1) {
		target := m[1]
		switch {
		case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"):
			continue
		case strings.HasPrefix(target, "#"):
			if !slugs[strings.TrimPrefix(target, "#")] {
				t.Errorf("%s 的锚点链接 %s 找不到对应标题", path, target)
			}
			continue
		}
		file, _, _ := strings.Cut(target, "#")
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(file))); err != nil {
			t.Errorf("%s 的链接 %s 指向不存在的文件: %v", path, target, err)
		}
	}
}

// assertReadmeDeployContract 合并后 README 承担原部署文档的固定值：预拉的 golang 镜像与 Dockerfile 一致（恰好 3 处），
// Docker 与原生守护工具的停止等待都等于 lifecycle.DeployStopWait（原生部署不再给逐平台配置，按用户语义写出
// supervisor / systemd / launchd 各自的键与取值），旧的退出预算不复发。
func assertReadmeDeployContract(t *testing.T, outDir string) {
	t.Helper()
	readme := readFile(t, filepath.Join(outDir, "README.md"))
	image := "golang:" + goVersion + "-alpine"
	if !strings.Contains(readFile(t, filepath.Join(outDir, "Dockerfile")), "FROM --platform=$BUILDPLATFORM "+image+" AS backend-builder") {
		t.Errorf("Dockerfile 后端构建镜像应为 %s", image)
	}
	if n := strings.Count(readme, image); n != 3 {
		t.Errorf("README 预拉的 golang 镜像应与 Dockerfile 一致且恰好 3 处（pull 1 + tag 2）：%s 实际 %d 处", image, n)
	}
	for _, tag := range golangImageTag.FindAllString(readme, -1) {
		if tag != image {
			t.Errorf("README 出现了与 %s 不一致的 golang 镜像标签 %s", image, tag)
		}
	}
	lifecycleSrc := readFile(t, filepath.Join(outDir, "backend", "app", "helper", "lifecycle", "shutdown.go"))
	m := regexp.MustCompile(`DeployStopWait = (\d+) \* time\.Second`).FindStringSubmatch(lifecycleSrc)
	if m == nil {
		t.Fatal("lifecycle 缺少 DeployStopWait 常量")
	}
	wait := m[1]
	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	if !strings.Contains(compose, "stop_grace_period: "+wait+"s\n") {
		t.Errorf("docker-compose.yml 的 stop_grace_period 应为 %ss", wait)
	}
	for _, want := range []string{"`stop_grace_period: " + wait + "s`", "**停止等待不少于 " + wait + " 秒**",
		"`stopwaitsecs=" + wait + "`", "`TimeoutStopSec=" + wait + "`", "`ExitTimeOut` 为 `" + wait + "`"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README 的停止等待说明应包含 %q（= lifecycle.DeployStopWait）", want)
		}
	}
	for _, stale := range []string{"stop_grace_period: 30s", "stopwaitsecs=30", "15.5 秒"} {
		if strings.Contains(compose+readme, stale) {
			t.Errorf("部署说明仍有旧的退出预算 %q", stale)
		}
	}
}

// assertOptionalDocsTrimmed 未选的 Consul / Milvus 在 README 与运维手册里不留任何术语。
func assertOptionalDocsTrimmed(t *testing.T, outDir string, optional []string) {
	t.Helper()
	residues := map[string][]string{
		"consul": {"consul"},
		"milvus": {"milvus", "minio", "etcd", "外置 s3", "storageconfig"},
	}
	for _, rel := range []string{"README.md", filepath.Join("docs", "运维手册.md")} {
		doc := strings.ToLower(readFile(t, filepath.Join(outDir, rel)))
		for name, words := range residues {
			if contains(optional, name) {
				continue
			}
			for _, w := range words {
				if strings.Contains(doc, w) {
					t.Errorf("未选 %s，%s 却残留 %q", name, rel, w)
				}
			}
		}
	}
}

// TestDocs_GeneratedOperationsGuide 三种组合只渲染模板（不 tidy / Wire），检查生成项目文档：
// 部署文档已删除且无任何引用；README 操作章节齐全、操作契约与真实脚本 / Makefile / compose 一致；
// 命令单行、无 heredoc、不要求全局 pnpm；链接与锚点有效；可选组件裁剪干净；
// 并实际运行迁移到 README / 运维手册的原有断言（assertOpsTooling、Milvus 文档断言）。
func TestDocs_GeneratedOperationsGuide(t *testing.T) {
	for _, tc := range docsCombos {
		t.Run(tc.dirName, func(t *testing.T) {
			outDir := generateDocsOnly(t, tc.dirName, tc.project, tc.optional)

			if _, err := os.Stat(filepath.Join(outDir, "docs", "部署文档.md")); !os.IsNotExist(err) {
				t.Error("docs/部署文档.md 已并入 README，不应再生成")
			}
			assertNoMatch(t, outDir, "部署文档")

			readme := readFile(t, filepath.Join(outDir, "README.md"))
			if n := strings.Count(readme, "\n") + 1; n > readmeLineBudget {
				t.Errorf("README 共 %d 行，超过回归护栏 %d 行：不要把逐平台教程、实现机制或历史拼回日常入口", n, readmeLineBudget)
			}
			for _, moved := range []string{"### 已落实的加固", "sqlmock", "DryRun", "<key>ExitTimeOut</key>", "[program:"} {
				if strings.Contains(readme, moved) {
					t.Errorf("README 不应再包含实现机制 / 逐平台守护配置 %q（归入维护文档）", moved)
				}
			}
			for _, h := range generatedReadmeHeadings {
				if !strings.Contains(readme, h) {
					t.Errorf("README 缺少操作章节 %q", strings.TrimSpace(h))
				}
			}
			for _, want := range generatedReadmeContract(tc.project) {
				if !strings.Contains(readme, want) {
					t.Errorf("README 缺少操作契约 %q", want)
				}
			}
			docs, err := filepath.Glob(filepath.Join(outDir, "docs", "*.md"))
			if err != nil || len(docs) == 0 {
				t.Fatalf("找不到 docs/*.md: %v", err)
			}
			for _, path := range append([]string{filepath.Join(outDir, "README.md"), filepath.Join(outDir, "scripts", "loadtest", "README.md")}, docs...) {
				rel, _ := filepath.Rel(outDir, path)
				doc := readFile(t, path)
				for _, bad := range staleOperationClaims {
					if strings.Contains(doc, bad) {
						t.Errorf("%s 仍有不真实或不安全的旧说法 %q", rel, bad)
					}
				}
				assertCommandsSingleLine(t, rel, doc, rel == "README.md" || strings.HasPrefix(rel, "docs"))
				assertSecretLifecycle(t, rel, doc)
				assertDocLinks(t, path)
			}
			assertUpdateRollbackOrder(t, readme, tc.project)
			assertRecoveryAndStartupOrder(t, readme, tc.project)
			assertOptionalDocsTrimmed(t, outDir, tc.optional)
			assertReadmeDeployContract(t, outDir)
			assertOpsTooling(t, outDir)
			if contains(tc.optional, "milvus") {
				assertMilvusDocs(t, outDir)
				assertMilvusReadFailureDocs(t, outDir)
			}
		})
	}
}

// TestDocs_RootReadmeScope 根 README 只面向脚手架使用者：安装、latest / 指定版本更新、生成、组件选择、生成失败处置；
// 不复制生成项目的部署 / 备份 / 更新操作，也不再放实现细节、验收历史与发布流程。
func TestDocs_RootReadmeScope(t *testing.T) {
	path := filepath.Join("..", "..", "README.md")
	root := readFile(t, path)
	for _, want := range []string{
		"## 2. 组件怎么选", "## 4. 安装", "## 5. 更新脚手架", "## 6. 生成项目", "## 7. 生成失败怎么办",
		"go install github.com/stones-hub/taurus-x@latest", "go install github.com/stones-hub/taurus-x@v",
		"GOPROXY=direct go install github.com/stones-hub/taurus-x@latest", "taurus-x version", "taurus-x new my-admin",
		"升级脚手架只影响以后新生成的项目", "**不要往已有项目目录里重新生成**",
		"**已经写出的文件保留在原地，不会自动删除**", "**这次的初始管理员密码不会显示**",
		"全部按生成项目里的 `README.md` 操作",
	} {
		if !strings.Contains(root, want) {
			t.Errorf("根 README 缺少 %q", want)
		}
	}
	for _, bad := range []string{
		// 生成项目的日常操作在生成 README，根 README 不复制
		"部署文档", "docker compose up", "supervisorctl", "launchctl", "make db-backup", "MYSQL_OPS_", "source /tmp",
		// 实现细节、验收历史与发布流程移到维护文档
		"脚手架是怎么实现的", "维护脚手架本身", "git tag -a", "gh release", "候选", "已验证与未验证边界", "missingkey",
	} {
		if strings.Contains(root, bad) {
			t.Errorf("根 README 不应包含 %q（属于生成项目 README 或维护文档）", bad)
		}
	}
	assertCommandsSingleLine(t, "README.md（taurus-x）", root, true)
	assertDocLinks(t, path)
}
