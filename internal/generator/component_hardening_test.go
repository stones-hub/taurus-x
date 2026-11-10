package generator

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// composeService 返回 docker-compose.yml 里某个服务的整段（从 "  name:" 行到下一个同级服务或顶层键之前）。
func composeService(compose, name string) string {
	marker := "\n  " + name + ":\n"
	i := strings.Index(compose, marker)
	if i < 0 {
		return ""
	}
	rest := compose[i+len(marker):]
	for _, line := range strings.SplitAfter(rest, "\n") {
		// 服务内的行至少缩进 4 个空格；注释和空行不算边界
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(line, "    ") {
			continue
		}
		return rest[:strings.Index(rest, line)]
	}
	return rest
}

// assertCredentialNormalization 凭据占位只把纯空白当没配：Redis 密码不再 TrimSpace，
// 统一走 emptyIfBlank；Redis 日志默认 warn；对应的运行级单测随项目生成。
func assertCredentialNormalization(t *testing.T, outDir string, selected []string) {
	t.Helper()
	taurusDir := filepath.Join(outDir, "backend", "internal", "taurus")
	componentsSrc := readFile(t, filepath.Join(taurusDir, "components.go"))
	conv := readFile(t, filepath.Join(taurusDir, "conv.go"))

	if body := funcBody(conv, "func emptyIfBlank(s string) string {"); !strings.Contains(body, `if strings.TrimSpace(s) == "" {`) ||
		!strings.Contains(body, "return s") {
		t.Errorf("emptyIfBlank 应只把纯空白归空、其余原样返回:\n%s", body)
	}
	if !strings.Contains(componentsSrc, `redisx.WithPassword(emptyIfBlank(cfg.GetString("redis.password")))`) {
		t.Error("Redis 密码应经 emptyIfBlank 处理（真实密码的首尾空格必须保留）")
	}
	if strings.Contains(componentsSrc, "strings.TrimSpace(cfg.GetString(") {
		t.Error("components.go 不得再对配置值整体 TrimSpace（会改掉真实凭据）")
	}
	if body := funcBody(componentsSrc, "func redisLogLevel(name string) redisx.LogLevel {"); !strings.Contains(body, "default:\n\t\treturn redisx.LogLevelWarn") {
		t.Errorf("redisLogLevel 没配或写错时应回落到 warn:\n%s", body)
	}
	redisToml := readFile(t, filepath.Join(outDir, "backend", "config", "autoload", "redis", "redis.toml"))
	if !strings.Contains(redisToml, "\nlogger_level = \"warn\"\n") {
		t.Error("redis.toml 的 logger_level 默认应为 warn")
	}
	if strings.Contains(redisToml, "TrimSpace") {
		t.Error("redis.toml 注释仍写着 TrimSpace，与实现不符")
	}
	tests := readFile(t, filepath.Join(taurusDir, "placeholder_test.go"))
	for _, name := range []string{"func TestEmptyIfBlank(", "func TestRedisLogLevel_DefaultsToWarn(", "func TestRedisConfig_RealFile("} {
		if !strings.Contains(tests, name) {
			t.Errorf("placeholder_test.go 缺少 %s", name)
		}
	}

	consulTest := readFile(t, filepath.Join(taurusDir, "consul_auth_test.go"))
	if contains(selected, "consul") {
		auth := funcBody(componentsSrc, "func consulAuthOptions(token, username, password string) ([]consul.Option, error) {")
		for _, want := range []string{"token = emptyIfBlank(token)", "username, password = emptyIfBlank(username), emptyIfBlank(password)", "consul.WithBasicAuth(username, password)", "return nil, fmt.Errorf("} {
			if !strings.Contains(auth, want) {
				t.Errorf("consulAuthOptions 缺少 %q:\n%s", want, auth)
			}
		}
		if !strings.Contains(funcBody(componentsSrc, "func ProvideConsulComponent("), "consulAuthOptions(") {
			t.Error("ProvideConsulComponent 应通过 consulAuthOptions 构造认证选项")
		}
		for _, name := range []string{"BlankPlaceholdersSendNothing", "RealValuesSentVerbatim", "HalfBasicAuthRejected"} {
			if !strings.Contains(consulTest, "func TestConsulAuthOptions_"+name+"(") {
				t.Errorf("consul_auth_test.go 缺少 TestConsulAuthOptions_%s", name)
			}
		}
	} else if strings.TrimSpace(consulTest) != "package taurus" {
		t.Error("未选 consul 时 consul_auth_test.go 应只有包声明")
	}
}

// assertMilvusFailFast Milvus 启用后严格校验、连不上即失败并关闭已建的池；disable_conn 不能绕过握手；
// 配置注释与已发布的 v0.3.0 / SDK v2.6.5 的实际行为一致；对应离线测试随项目生成。
func assertMilvusFailFast(t *testing.T, outDir string, selected []string) {
	t.Helper()
	taurusDir := filepath.Join(outDir, "backend", "internal", "taurus")
	milvusTest := readFile(t, filepath.Join(taurusDir, "milvus_provider_test.go"))
	rollbackTest := readFile(t, filepath.Join(taurusDir, "wire_rollback_test.go"))
	readLogTest := readFile(t, filepath.Join(taurusDir, "milvus_read_log_test.go"))
	componentsSrc := readFile(t, filepath.Join(taurusDir, "components.go"))
	if !contains(selected, "milvus") {
		if strings.TrimSpace(milvusTest) != "package taurus" || strings.TrimSpace(rollbackTest) != "package taurus" ||
			strings.TrimSpace(readLogTest) != "package taurus" {
			t.Error("未选 milvus 时 milvus_provider_test.go / wire_rollback_test.go / milvus_read_log_test.go 应只有包声明")
		}
		// 未选 milvus：失败日志回调和 Milvus 依赖都不生成
		for _, bad := range []string{"milvusReadFailureObserver", "WithReadRecoveryObserver", "milvus_read_failed", "/util/merr"} {
			if strings.Contains(componentsSrc, bad) {
				t.Errorf("未选 milvus，components.go 却出现 %q", bad)
			}
		}
		if goMod := readFile(t, filepath.Join(outDir, "backend", "go.mod")); strings.Contains(goMod, "github.com/milvus-io/") {
			t.Error("未选 milvus，go.mod 却带了 milvus-io 依赖")
		}
		return
	}

	provide := funcBody(componentsSrc, "func ProvideMilvusComponent(cfg *config.Config, logger *logx.Manager) (milvus.Pool, func(), error) {")
	if provide == "" {
		t.Fatal("components.go 找不到 ProvideMilvusComponent")
	}
	for _, bad := range []string{"skipped", "continue", "err.Error()"} {
		if strings.Contains(provide, bad) {
			t.Errorf("ProvideMilvusComponent 不应再跳过条目或拼接原始错误（命中 %q）", bad)
		}
	}
	addFail := blockAfter(provide, "if err := pool.Add(entry.name, entry.opts...); err != nil {")
	if !strings.Contains(addFail, "closePool()") || !strings.Contains(addFail, "milvusConnectFailure(err)") ||
		strings.Index(addFail, "closePool()") > strings.Index(addFail, "return nil") {
		t.Errorf("建连失败时应先关闭已建的池、再返回只含分类的错误:\n%s", addFail)
	}
	if strings.Index(provide, "milvusEntries(") > strings.Index(provide, "newMilvusPool()") {
		t.Error("应先校验完整个 milvus.list 再建池连接")
	}
	if !strings.Contains(provide, `boundedCleanup("milvus", milvusCleanupTimeout, func(context.Context) error {`) {
		t.Error("Milvus 池的关闭应保持限时")
	}
	assertMilvusReadFailureLog(t, outDir, componentsSrc, provide, milvusTest+readLogTest)
	entries := funcBody(componentsSrc, "func milvusEntries(raw interface{}) ([]milvusEntry, error) {")
	for _, want := range []string{"缺失或不是列表", "为空", "格式不对", "name 为空或不是字符串", "带首尾空白", "重复"} {
		if !strings.Contains(entries, want) {
			t.Errorf("milvusEntries 缺少拒绝情形 %q", want)
		}
	}
	opts := funcBody(componentsSrc, "func milvusClientOptions(item map[string]interface{}) ([]mclient.Option, error) {")
	if strings.Contains(opts, "WithDisableConn") || !strings.Contains(opts, "disable_conn=true") {
		t.Error("启用 Milvus 时 disable_conn=true 必须被拒绝，不能传给组件跳过握手")
	}
	if !strings.Contains(opts, "creds[i] = emptyIfBlank(v)") || strings.Contains(opts, "TrimSpace(v)") {
		t.Error("Milvus 凭据应经 emptyIfBlank 处理，不能 TrimSpace")
	}

	yaml := readFile(t, filepath.Join(outDir, "backend", "config", "autoload", "milvus", "milvus.yaml"))
	for _, want := range []string{
		"enable_tls_auth: ${MILVUS_TLS:false}", "disable_conn: false", `min_connect_timeout: "10s"`,
		"不应用这两项", "10 秒", "deadline", "Close 是终态", "不支持 mTLS 和自定义 CA", "启动失败",
		// 依赖已发布的 v0.3.0：只给 Search / Query 兜底；业务错误按 merr 白名单 errors.Is 分类
		"已发布的 taurus-pro-milvus " + milvusRelease + " / SDK v2.6.5", milvusRelease + " 只给 Search / Query",
		"从调用开始计的 60 秒兜底", "其它业务调用仍要自己设 deadline", "errors.Is 对照固定的 merr 哨兵错误白名单分类",
		// Close 取消在途读；裸 SDK 句柄不能与 Search / Query 的恢复并发
		milvusRelease + " 默认启用 Search / Query 的只读恢复", "Close 会直接取消在途的 Search / Query 和恢复等待",
		"final_class=other", "不能一律当成 Close 引起的", "所以它也不能与 Search / Query 并发",
		"mclient.WithReadRecovery(false)", "组件不承诺裸 SDK 调用的透明恢复",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("milvus.yaml 缺少 %q", want)
		}
	}
	for _, stale := range []string{"错误用 status.Code(err) 判断 gRPC 状态", "（客户端不替你兜底）", "关闭后不能再 Add / Get，Close 会等在途请求结束",
		// 已发布 v0.3.0 后不再区分旧版 / 候选
		"v0.2.0", "尚未发布", "候选", "Close 会等在途请求结束"} {
		if strings.Contains(yaml, stale) {
			t.Errorf("milvus.yaml 仍残留过期说法 %q", stale)
		}
	}

	for _, name := range []string{
		"TestMilvusClientOptions_PublicOptionSemantics", "TestMilvusEntries_Rejects",
		"TestProvideMilvusComponent_Disabled", "TestProvideMilvusComponent_RealConfigCredentialsOnTheWire",
		"TestProvideMilvusComponent_PartialFailureClosesPool", "TestProvideMilvusComponent_ConfigErrorsFailBeforeConnecting",
		"TestBuildComponents_MilvusFailureClosesPool",
	} {
		if !strings.Contains(milvusTest, "func "+name+"(") {
			t.Errorf("milvus_provider_test.go 缺少 %s", name)
		}
	}
	hasRollback := strings.Contains(rollbackTest, "func TestBuildComponents_MilvusFailureRollsBackConsul(")
	if contains(selected, "consul") != hasRollback {
		t.Errorf("wire_rollback_test.go 应当且仅当同时选了 consul 和 milvus 时包含回滚测试（实际 %t）", hasRollback)
	}

	assertMilvusDocs(t, outDir)
}

// assertMilvusDocs 勾选 milvus 时，生成 README「6.5 Milvus 外置 S3」（原部署文档第二节并入）是外置 S3 契约与
// 启用即 fail-fast 的唯一说明；README / 运维手册不残留旧说法与内置 MinIO 结论。
func assertMilvusDocs(t *testing.T, outDir string) {
	t.Helper()
	readme := readFile(t, filepath.Join(outDir, "README.md"))
	manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
	for _, stale := range []string{"连不上时也只打日志", "app 自身不依赖 Milvus 启动"} {
		if strings.Contains(readme+manual, stale) {
			t.Errorf("文档仍残留过期说法 %q", stale)
		}
	}
	s3 := markdownSection(t, readme, readmeMilvusS3Heading)
	if !strings.Contains(s3, "Milvus 启用后连不上会让启动失败") {
		t.Error("README「6.5 Milvus 外置 S3」应说明 Milvus 启用即 fail-fast")
	}
	// 外置 S3 契约：必填变量、固定安全设置、私有 CA 不支持、不得被更高优先级来源覆盖、兼容验收未做
	for _, want := range []string{"Milvus 的对象存储必须接外置 S3", "`MILVUS_S3_ADDRESS`", "`MILVUS_S3_ACCESS_KEY_ID` / `MILVUS_S3_SECRET_ACCESS_KEY`",
		"`MILVUS_S3_BUCKET`", "`MILVUS_S3_ROOT_PATH`", "`MILVUS_S3_REGION`", "`MILVUS_S3_USE_VIRTUAL_HOST`", "`MILVUS_S3_REQUEST_TIMEOUT_MS`",
		"`MINIO_SSL_TLSCACERT: \"\"`", "`MINIO_USE_SSL: \"true\"`", "**只支持公共 CA 签发证书的 HTTPS 端点**",
		"**不支持私有 CA / 自签证书**", "**不要在 etcd 里写 Milvus 的", "`MILVUS_CONF_MINIO_*`", "状态 NOT_RUN"} {
		if !strings.Contains(s3, want) {
			t.Errorf("README「6.5 Milvus 外置 S3」缺少外置 S3 契约说明 %q", want)
		}
	}
	if !strings.Contains(readme, "**外置 S3**") || !strings.Contains(readme, "不支持私有 CA / 自签证书 / HTTP") {
		t.Error("README 应说明 Milvus 必须接外置 S3、且只支持公共 CA 的 HTTPS")
	}
	// 旧的内置 MinIO 结论必须全部撤掉
	for _, stale := range []string{"对象存储镜像分发", "minio/minio:", "minio-data", "确认旧标签可拉取", "MINIO_ROOT_USER", "minioadmin"} {
		if strings.Contains(readme+manual, stale) {
			t.Errorf("文档仍残留内置 MinIO 的过期说法 %q", stale)
		}
	}
}

// assertMilvusReadFailureLog Milvus Search / Query 最终失败写脱敏日志：provider 接收 wire 传入的日志组件、启用时缺日志组件即报错，
// 每个客户端挂观察回调（只追加观察回调，不显式开关只读恢复）；日志格式固定、分类不读错误文本；对应离线测试与文档随项目生成。
func assertMilvusReadFailureLog(t *testing.T, outDir, componentsSrc, provide, milvusTests string) {
	t.Helper()
	nilCheck := strings.Index(provide, "if logger == nil {")
	if nilCheck < 0 || nilCheck > strings.Index(provide, "newMilvusPool()") || nilCheck < strings.Index(provide, `if !cfg.GetBool("milvus.enable") {`) {
		t.Error("ProvideMilvusComponent 应在未启用时直接返回、启用时先拒绝 nil 日志组件再建池")
	}
	appendObserver := strings.Index(provide, "entry.opts = append(entry.opts, mclient.WithReadRecoveryObserver(observer))")
	if !strings.Contains(provide, "observer := milvusReadFailureObserver(logger)") || appendObserver < 0 ||
		appendObserver > strings.Index(provide, "if err := pool.Add(entry.name, entry.opts...); err != nil {") {
		t.Error("每个 Milvus 客户端都应在 pool.Add 之前追加失败日志观察回调")
	}
	if strings.Contains(provide, "Container") {
		t.Error("ProvideMilvusComponent 不应从全局 Container 取日志组件")
	}
	// 只读恢复用组件默认值：生成代码与测试都不显式调用 WithReadRecovery（WithReadRecoveryObserver 不受影响）
	if strings.Contains(componentsSrc, "WithReadRecovery(") || strings.Contains(milvusTests, "WithReadRecovery(") {
		t.Error("生成项目不应显式开关只读恢复（WithReadRecovery），应使用组件默认启用")
	}
	// 第二个参数必须就是 ProvideLoggerComponent 返回的日志组件变量，第一个参数是同一个配置变量；
	// 只按变量名捕获，不绑定模板基线的写法（wire 重新生成时变量名会变）
	wireGen := readFile(t, filepath.Join(outDir, "backend", "internal", "taurus", "wire_gen.go"))
	loggerCalls := regexp.MustCompile(`(\w+), \w+, err :?= ProvideLoggerComponent\((\w+)\)`).FindAllStringSubmatch(wireGen, -1)
	milvusCalls := regexp.MustCompile(`ProvideMilvusComponent\((\w+), (\w+)\)`).FindAllStringSubmatch(wireGen, -1)
	if len(loggerCalls) != 1 || len(milvusCalls) != 1 {
		t.Errorf("wire_gen.go 应各有一处 ProvideLoggerComponent / ProvideMilvusComponent 调用，实际 %d / %d", len(loggerCalls), len(milvusCalls))
	} else if logVar, cfgVar := loggerCalls[0][1], loggerCalls[0][2]; logVar == cfgVar ||
		milvusCalls[0][1] != cfgVar || milvusCalls[0][2] != logVar {
		t.Errorf("wire_gen.go 应把 ProvideLoggerComponent 返回的日志组件 %s 传给 ProvideMilvusComponent(%s, %s)，实际 %s",
			logVar, cfgVar, logVar, milvusCalls[0][0])
	}

	if !strings.Contains(componentsSrc, "const milvusReadFailedFormat = \"milvus_read_failed op=%s first_class=%s final_class=%s attempts=%d elapsed_ms=%d\"\n") {
		t.Error("Milvus 失败日志应使用固定格式常量")
	}
	// 注释口径：执行次数不是服务端 RPC 次数；回调只挂在 milvus.list 建的客户端上；日志配置与关闭的边界
	for _, want := range []string{"不是服务端收到的 RPC 次数（可能在本地构建请求阶段就失败），也不含 SDK 内部的重试",
		"业务运行时自己 pool.Add 的客户端\n//     不会自动挂", "日志组件可能回退到控制台", "default 的级别配得比 ERROR 高时这条会被过滤",
		"连接池的限时 cleanup 到期后不保证迟到的回调都已结束"} {
		if !strings.Contains(componentsSrc, want) {
			t.Errorf("components.go 的 Milvus 注释缺少 %q", want)
		}
	}
	for _, stale := range []string{"实际查询次数", "Close 是终态，会等在途请求结束"} {
		if strings.Contains(componentsSrc, stale) {
			t.Errorf("components.go 的 Milvus 注释仍残留过期说法 %q", stale)
		}
	}
	// provider 段（文档注释 + 函数体）只写已发布版本的建连上限与 Close 语义，不残留旧版 / 候选的说法
	providerSec := ""
	if i := strings.Index(componentsSrc, "// ProvideMilvusComponent 注入"); i >= 0 {
		providerSec, _, _ = strings.Cut(componentsSrc[i:], "\n}")
	}
	for _, want := range []string{"milvus.DefaultCreateTimeout，" + milvusRelease + " 为 10 秒，含握手",
		"Close 是终态；" + milvusRelease + " 会直接取消在途的 Search / Query 和恢复等待，其它调用仍按原有语义等它结束"} {
		if !strings.Contains(providerSec, want) {
			t.Errorf("ProvideMilvusComponent 的注释缺少 %q", want)
		}
	}
	for _, stale := range []string{"v0.2.0", "候选", "尚未发布", "会等在途请求结束"} {
		if strings.Contains(providerSec, stale) {
			t.Errorf("ProvideMilvusComponent 的注释仍残留过期说法 %q", stale)
		}
	}
	observer := funcBody(componentsSrc, "func milvusReadFailureObserver(logger *logx.Manager) func(mclient.ReadRecoveryEvent) {")
	for _, want := range []string{"if ev.Err == nil {\n\t\t\treturn", `logger.LError("default", milvusReadFailedFormat,`,
		"milvusReadOp(ev.Op), milvusReadErrorClass(ev.FirstErr), milvusReadErrorClass(ev.Err)", "ev.Attempts, ev.Elapsed.Milliseconds())"} {
		if !strings.Contains(observer, want) {
			t.Errorf("milvusReadFailureObserver 缺少 %q:\n%s", want, observer)
		}
	}
	for _, bad := range []string{"ev.Collection", "Error()", "%v", "%q", "Sprintf", "entry", "go func"} {
		if strings.Contains(observer, bad) {
			t.Errorf("milvusReadFailureObserver 不应出现 %q（不记集合名 / 错误原文，不另起 goroutine）", bad)
		}
	}
	class := funcBody(componentsSrc, "func milvusReadErrorClass(err error) string {")
	for _, want := range []string{"errors.As(err, &recoveryErr)", "err = recoveryErr.Last",
		"errors.Is(err, mclient.ErrReadRecoveryExhausted)", "errors.Is(err, mclient.ErrReadRecoveryDatabaseChanged)",
		"errors.Is(err, context.Canceled)", "errors.Is(err, context.DeadlineExceeded)",
		"errors.Is(err, merr.ErrServiceNotReady)", "errors.Is(err, merr.ErrServiceUnavailable)",
		"errors.Is(err, merr.ErrChannelNotAvailable)", "errors.Is(err, merr.ErrChannelTSafeStalled)",
		"errors.As(err, &withStatus)", "case codes.PermissionDenied:", "case codes.Unauthenticated:", "case codes.InvalidArgument:",
		"case codes.Unavailable:", "default:\n\t\treturn \"other\""} {
		if !strings.Contains(class, want) {
			t.Errorf("milvusReadErrorClass 缺少 %q", want)
		}
	}
	for _, bad := range []string{"Error()", "status.Code(", "Message()", "strings.", "Sprintf", "First"} {
		if strings.Contains(class, bad) {
			t.Errorf("milvusReadErrorClass 不应出现 %q（不读错误文本、不按首次错误分类）", bad)
		}
	}

	for _, name := range []string{
		"TestMilvusReadErrorClass", "TestMilvusReadOp", "TestMilvusReadFailureObserver_FinalFailureOnly",
		"TestMilvusReadFailureObserver_Concurrent", "TestProvideMilvusComponent_EnabledRequiresLogger",
		"TestMilvusReadFailureLog_FirstAttempt", "TestMilvusReadFailureLog_RecoveryCutShortByCaller",
		"TestMilvusReadFailureLog_ConcurrentRequests", "TestBuildComponents_MilvusNextRequestAfterFailure",
	} {
		if !strings.Contains(milvusTests, "func "+name+"(") {
			t.Errorf("milvus_read_log_test.go 缺少 %s", name)
		}
	}

	assertMilvusReadFailureDocs(t, outDir)
}

// assertMilvusReadFailureDocs 勾选 milvus 时，运维手册「四、Milvus 查询失败日志」（原第六节，运维手册只保留进阶判读后重新编号）
// 是恢复与失败日志口径的完整说明；README「6.5 Milvus 外置 S3」只保留用户必需的摘要并指向它，不残留过期说法。
func assertMilvusReadFailureDocs(t *testing.T, outDir string) {
	t.Helper()
	manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
	_, opsLog, found := strings.Cut(manual, "\n## 四、Milvus 查询失败日志\n")
	if !found {
		t.Fatal("勾选 milvus 时运维手册缺少「四、Milvus 查询失败日志」一节")
	}
	for _, want := range []string{"只在**本次请求内**每隔 5 秒重建连接后重试", "单次调用最多 60 秒，调用方 ctx 的期限更短时以调用方为准",
		"没有后台重连、健康轮询或自动重放", "前一次失败不会把客户端关掉", "写操作（插入、删除、建删集合等）不在恢复范围内",
		"模板默认 `logs/app.log`", "milvus_read_failed op=Search first_class=", "首次直接成功、经恢复后成功都不写",
		"客户端名、地址、集合名、过滤表达式、向量、凭据和服务端 / SDK 的错误原文", "沿用现有的同步文件日志组件，不排队、不重试",
		"不是失败请求的补偿或重放记录", "`go.mod` 依赖的已发布 taurus-pro-milvus " + milvusRelease, "模板不带 replace",
		"离线单元测试用进程内 fake Milvus", "不代表已在你们的真实 Milvus 环境里验证过",
		// 后台约束只指组件包装层；Close 是显式调用的终态；写操作只承诺包装层不重试；attempts 的口径
		"这里只指组件包装层的恢复", "SDK / gRPC 对已建连接自身的既有维护", "连接池 Close 是显式调用后的终态",
		"不论是进程退出时的清理还是业务代码主动调用", "组件包装层不做恢复或重试", "这不代表 SDK 内部不重试",
		"越过恢复门禁、执行查询函数的次数", "包含在本地构建请求阶段就失败的执行，不保证每次都发到了服务端，也不含 SDK 内部的重试",
		// 5 秒是等待间隔；Close 取消在途读与 other；裸 SDK 并发约束；日志覆盖范围、配置与关闭边界
		"5 秒是每次失败后的等待间隔，不保证每 5 秒都有一次拨号", "拨号和 SDK 重试的耗时都算在同一个 60 秒预算里",
		"Close 会直接取消在途的 Search / Query 和恢复等待，其它调用仍按原有语义等它结束",
		"`final_class=other`", "`other` 也包含其它原因，不能一律当成 Close 引起的",
		"不能与\n  Search / Query 的恢复、Close、UseDatabase 并发使用", "也保护不了已经拿着旧句柄的在途调用",
		"显式 `mclient.WithReadRecovery(false)` 的客户端专用，或保证它与上述调用串行", "组件不承诺裸 SDK 调用的透明恢复",
		"不等于服务端收到的 RPC 次数", "只有按 `milvus.list` 创建的客户端会写这条日志", "自己 `pool.Add` 的客户端不会自动挂上",
		"删掉这项时日志组件可能回退到控制台", "级别配得比 ERROR 高时这条会被过滤",
		"限时清理到期后，不保证迟到的日志回调都已结束", "关闭连接池也不等于失败日志已经刷盘"} {
		if !strings.Contains(opsLog, want) {
			t.Errorf("运维手册第四节缺少 %q", want)
		}
	}
	for _, stale := range []string{"只有进程退出时的连接池 Close 是终态", "实际发出的查询次数", "失败直接返回，是否重试由调用方决定",
		// 已发布 v0.3.0 后撤掉候选 / 未发布 / 编译不过的说法
		"v0.2.0", "尚未发布", "候选", "编译不过", "未交付边界"} {
		if strings.Contains(opsLog, stale) {
			t.Errorf("运维手册第四节仍残留过期说法 %q", stale)
		}
	}
	// README 只留摘要：失败后等待 5 秒（不承诺固定拨号节奏）、拨号与 SDK 重试共用 60 秒预算且调用方更短期限优先、
	// 无后台重连、包装层写不重试（不代表 SDK 不重试）、最终失败一条脱敏日志、已发布版本无 replace，并指向运维手册第四节
	readme := readFile(t, filepath.Join(outDir, "README.md"))
	milvusNote := markdownSection(t, readme, readmeMilvusS3Heading)
	for _, want := range []string{"只在本次请求内失败后等待 5 秒再重建重试", "5 秒是等待间隔，不保证固定的拨号节奏",
		"拨号和 SDK 内部重试共用单次 60 秒预算", "调用方 ctx 期限更短时以调用方为准", "组件包装层没有后台重连",
		"写操作组件包装层不重试（不代表 SDK 内部不重试）", "`milvus_read_failed`", "[`docs/运维手册.md`](docs/运维手册.md) 第四节",
		"`go.mod` 依赖的已发布 taurus-pro-milvus " + milvusRelease, "模板不带 replace"} {
		if !strings.Contains(milvusNote, want) {
			t.Errorf("README 的 Milvus 失败日志摘要缺少 %q", want)
		}
	}
	// 「每 5 秒重建重试」会被读成固定拨号节奏
	for _, stale := range []string{"v0.2.0", "尚未发布", "候选", "编译不过", "每 5 秒重建重试"} {
		if strings.Contains(milvusNote, stale) {
			t.Errorf("README 的 Milvus 说明仍残留过期说法 %q", stale)
		}
	}
}

// assertComposeOptionalServices Consul / Milvus 编排：固定兼容镜像、健康检查、service_healthy 依赖、
// 宿主端口只绑本机；Milvus 保持独立 etcd + 外置 S3（不内置 MinIO，不改成内嵌单容器 / 本地存储）。
func assertComposeOptionalServices(t *testing.T, outDir string, selected []string) {
	t.Helper()
	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	app := composeService(compose, "app")
	if app == "" {
		t.Fatal("docker-compose.yml 找不到 app 服务")
	}

	if contains(selected, "consul") {
		consulSvc := composeService(compose, "consul")
		if !strings.Contains(consulSvc, `- "127.0.0.1:8500:8500"`) || strings.Contains(consulSvc, `- "8500:8500"`) {
			t.Errorf("consul 的 8500 只能绑 127.0.0.1:\n%s", consulSvc)
		}
		if !strings.Contains(consulSvc, `test: ["CMD", "consul", "members"]`) {
			t.Error("consul 服务缺少健康检查")
		}
		if !strings.Contains(app, "consul:\n        condition: service_healthy") {
			t.Error("app 应等 consul 健康后再启动")
		}
	}

	// 任何组合都不再内置 MinIO
	if svc := composeService(compose, "minio"); svc != "" {
		t.Errorf("compose 不应再有 minio 服务:\n%s", svc)
	}
	for _, bad := range []string{"minio-data", "image: minio/", "image: quay.io/minio/", "MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "minioadmin"} {
		if strings.Contains(compose, bad) {
			t.Errorf("compose 不应再出现内置 MinIO 的 %q", bad)
		}
	}

	if !contains(selected, "milvus") {
		// 未选 milvus：不残留 etcd / Milvus 服务，也不出现任何外置 S3 必填项
		for _, bad := range []string{"MILVUS_S3_", "MINIO_", "COMMON_STORAGETYPE", "etcd"} {
			if strings.Contains(compose, bad) {
				t.Errorf("未选 milvus，compose 却出现 %q", bad)
			}
		}
		return
	}
	milvusSvc, etcdSvc := composeService(compose, "milvus"), composeService(compose, "etcd")
	if milvusSvc == "" || etcdSvc == "" {
		t.Fatal("Milvus 必须保留独立的 etcd 服务")
	}
	for _, want := range []string{"image: milvusdb/milvus:v2.6.5\n", "ETCD_ENDPOINTS: etcd:2379",
		`test: ["CMD", "curl", "-f", "http://localhost:9091/healthz"]`,
		`- "127.0.0.1:19530:19530"`, `- "127.0.0.1:9091:9091"`,
		"etcd:\n        condition: service_healthy",
		"- milvus-data:/var/lib/milvus",
		// 外置 S3 必填项：缺失即由 compose 拒绝（${VAR:?...}），不给默认值
		"MINIO_ADDRESS: ${MILVUS_S3_ADDRESS:?", "MINIO_ACCESS_KEY_ID: ${MILVUS_S3_ACCESS_KEY_ID:?",
		"MINIO_SECRET_ACCESS_KEY: ${MILVUS_S3_SECRET_ACCESS_KEY:?", "MINIO_BUCKET_NAME: ${MILVUS_S3_BUCKET:?",
		"MINIO_ROOT_PATH: ${MILVUS_S3_ROOT_PATH:?", "MINIO_REGION: ${MILVUS_S3_REGION:?",
		// 可选项按 v2.6.5 默认值
		"MINIO_USE_VIRTUAL_HOST: ${MILVUS_S3_USE_VIRTUAL_HOST:-false}\n", "MINIO_REQUEST_TIMEOUT_MS: ${MILVUS_S3_REQUEST_TIMEOUT_MS:-10000}\n",
		// 固定的安全设置：只走 HTTPS、CA 显式置空（v2.6.5 C++ 在 CA 非空时关闭证书校验）、aws + 静态密钥、remote
		"MINIO_PORT: \"443\"\n", "MINIO_USE_SSL: \"true\"\n", "MINIO_SSL_TLSCACERT: \"\"\n", "MINIO_CLOUD_PROVIDER: aws\n",
		"MINIO_USE_IAM: \"false\"\n", "COMMON_STORAGETYPE: remote\n"} {
		if !strings.Contains(milvusSvc, want) {
			t.Errorf("milvus 服务缺少 %q", want)
		}
	}
	// 安全设置只能出现一次且不能被 .env 覆盖：每个固定键在整个 compose 里恰好一处
	for _, key := range []string{"MINIO_USE_SSL:", "MINIO_SSL_TLSCACERT:", "MINIO_CLOUD_PROVIDER:", "MINIO_USE_IAM:", "COMMON_STORAGETYPE:", "MINIO_PORT:"} {
		if n := strings.Count(compose, key); n != 1 {
			t.Errorf("compose 里 %s 应恰好出现一次（实际 %d）", key, n)
		}
	}
	for _, bad := range []string{`- "19530:19530"`, `- "9091:9091"`, "ETCD_USE_EMBED", "standalone_embed",
		"minio:\n        condition", "MINIO_ADDRESS: minio:9000", "http://minio",
		"MINIO_USE_SSL: \"false\"", "MINIO_USE_SSL: ${", "MINIO_SSL_TLSCACERT: ${", "MINIO_SSL_TLSCACERT: /",
		"MINIO_USE_IAM: \"true\"", "MINIO_USE_IAM: ${", "MINIO_CLOUD_PROVIDER: ${", "COMMON_STORAGETYPE: ${",
		"COMMON_STORAGETYPE: local", "COMMON_STORAGETYPE: minio", "COMMON_STORAGETYPE: opendal",
		"MILVUS_CONF_", "MILVUSCONF", "user.yaml", "MILVUS_S3_ADDRESS:-", "MILVUS_S3_ACCESS_KEY_ID:-",
		"MILVUS_S3_SECRET_ACCESS_KEY:-", "MILVUS_S3_BUCKET:-", "MILVUS_S3_ROOT_PATH:-", "MILVUS_S3_REGION:-"} {
		if strings.Contains(milvusSvc, bad) {
			t.Errorf("milvus 服务不应出现 %q（端口须绑本机；不得内嵌 etcd / 本地存储；外置 S3 只走 HTTPS、CA 置空、必填项无默认值）", bad)
		}
	}
	if !strings.Contains(etcdSvc, "image: quay.io/coreos/etcd:v3.5.18\n") || !strings.Contains(etcdSvc, `test: ["CMD", "etcdctl", "endpoint", "health"]`) ||
		!strings.Contains(etcdSvc, "- etcd-data:/etcd") {
		t.Errorf("etcd 服务应为 v3.5.18、带健康检查并保留数据卷:\n%s", etcdSvc)
	}
	if strings.Contains(compose, "milvusdb/milvus:v2.4") {
		t.Error("compose 不得使用旧版 Milvus 镜像")
	}
	if strings.Contains(compose, "对象存储镜像分发受阻") || !strings.Contains(compose, "对象存储不再内置 MinIO") {
		t.Error("compose 应说明对象存储不再内置 MinIO，并去掉旧的镜像分发缺口注释")
	}
	if !strings.Contains(app, "milvus:\n        condition: service_healthy") {
		t.Error("app 应等 milvus 健康后再启动")
	}
	if strings.Contains(app, "MILVUS_S3_") || strings.Contains(app, "MINIO_") {
		t.Error("外置 S3 的凭据只给 milvus 服务，app 不应拿到")
	}
	volumes := compose[strings.LastIndex(compose, "\nvolumes:\n"):]
	for _, v := range []string{"  etcd-data:", "  milvus-data:"} {
		if !strings.Contains(volumes, v) {
			t.Errorf("顶层 volumes 缺少 %s", v)
		}
	}
}

// TestComposeService composeService 按服务切段，不会吃进下一个服务。
func TestComposeService(t *testing.T) {
	src := "services:\n  a:\n    image: x\n    # note\n\n    ports:\n      - \"1:1\"\n  b:\n    image: y\n\nvolumes:\n  v:\n"
	if got := composeService(src, "a"); !strings.Contains(got, "image: x") || !strings.Contains(got, "1:1") || strings.Contains(got, "image: y") {
		t.Errorf("服务 a 切段不对: %q", got)
	}
	if got := composeService(src, "b"); !strings.Contains(got, "image: y") || strings.Contains(got, "volumes") {
		t.Errorf("服务 b 切段不对: %q", got)
	}
	if composeService(src, "c") != "" {
		t.Error("不存在的服务应返回空串")
	}
}
