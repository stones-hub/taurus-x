package cli

import (
	"runtime/debug"
	"testing"
)

// buildInfo 造一个只带 Main.Version 和 vcs.* 设置的 BuildInfo，
// resolveVersion 只看这几项，其余字段留空即可。
func buildInfo(mainVersion string, settings ...debug.BuildSetting) *debug.BuildInfo {
	info := &debug.BuildInfo{Settings: settings}
	info.Main.Version = mainVersion
	return info
}

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		want     string
	}{
		{
			name:     "-ldflags 注入值优先于 BuildInfo",
			injected: "v9.9.9",
			info:     buildInfo("v0.1.0"),
			want:     "v9.9.9",
		},
		{
			name:     "注入值首尾空白被裁掉",
			injected: "  v9.9.9\n",
			info:     nil,
			want:     "v9.9.9",
		},
		{
			name: "go install @v0.1.0 用正式 tag",
			info: buildInfo("v0.1.0"),
			want: "v0.1.0",
		},
		{
			name: "go install @<commit> 用伪版本",
			info: buildInfo("v0.0.0-20261019120000-1e15e63db2fd"),
			want: "v0.0.0-20261019120000-1e15e63db2fd",
		},
		{
			name: "(devel) 带 revision",
			info: buildInfo("(devel)",
				debug.BuildSetting{Key: "vcs.revision", Value: "1e15e63db2fd9a0c7f1b2d3e4f5a6b7c8d9e0f12"},
				debug.BuildSetting{Key: "vcs.modified", Value: "false"},
			),
			want: "dev (1e15e63db2fd)",
		},
		{
			name: "(devel) 带 revision 且工作区有改动",
			info: buildInfo("(devel)",
				debug.BuildSetting{Key: "vcs.revision", Value: "1e15e63db2fd9a0c7f1b2d3e4f5a6b7c8d9e0f12"},
				debug.BuildSetting{Key: "vcs.modified", Value: "true"},
			),
			want: "dev (1e15e63db2fd-dirty)",
		},
		{
			name: "(devel) 无 vcs 信息",
			info: buildInfo("(devel)"),
			want: "dev",
		},
		{
			name: "Main.Version 为空且无 vcs 信息",
			info: buildInfo(""),
			want: "dev",
		},
		{
			name: "Main.Version 为空但有 vcs 信息",
			info: buildInfo("",
				debug.BuildSetting{Key: "vcs.revision", Value: "1e15e63db2fd9a0c7f1b2d3e4f5a6b7c8d9e0f12"},
			),
			want: "dev (1e15e63db2fd)",
		},
		{
			name: "revision 短于 12 位时原样输出",
			info: buildInfo("(devel)",
				debug.BuildSetting{Key: "vcs.revision", Value: "1e15e63"},
				debug.BuildSetting{Key: "vcs.modified", Value: "true"},
			),
			want: "dev (1e15e63-dirty)",
		},
		{
			name: "BuildInfo 为 nil",
			info: nil,
			want: "dev",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.injected, tc.info); got != tc.want {
				t.Errorf("resolveVersion(%q, %+v) = %q, 期望 %q", tc.injected, tc.info, got, tc.want)
			}
		})
	}
}

// 测试进程本身是 go test 构建的，version() 必须返回非空字符串，不能 panic。
func TestVersionNotEmpty(t *testing.T) {
	if got := version(); got == "" {
		t.Error("version() 返回空字符串")
	}
}
