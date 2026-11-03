//go:build unix

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 备份 / 恢复过程中按 Ctrl+C：终端把 SIGINT 发给整个前台进程组（脚本和它的 mysqldump / mysql 子进程）。
// 这里用独立进程组模拟同样的投递方式，验证中断后临时文件清理干净、给出正确提示。

// interruptWhenStarted 启动命令，等假命令写出 started 标记后向整个进程组发 SIGINT，返回退出码和输出。
func (e *opsEnv) interruptWhenStarted(t *testing.T, name string, args ...string) (int, string) {
	t.Helper()
	cmd := e.command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	started := filepath.Join(e.fake, "started")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Wait()
			t.Fatalf("假命令没有启动:\n%s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		t.Fatalf("SIGINT 后脚本没有退出:\n%s", out.String())
	}
	// 进程组里万一还有残留（如假命令的后台 sleep），一并清掉，不影响后续用例。
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return cmd.ProcessState.ExitCode(), out.String()
}

func TestDBScripts_BackupInterruptCleansUp(t *testing.T) {
	e := newOpsEnv(t)
	e.reset(t)
	e.extra = []string{"FAKE_DUMP=sleep"}
	code, out := e.interruptWhenStarted(t, "scripts/db/backup.sh", "--database", "ops_demo")
	if code != 130 || !strings.Contains(out, "已中断：未生成备份文件") {
		t.Fatalf("中断备份应以 130 退出并提示未生成文件，实际 %d:\n%s", code, out)
	}
	if files := listDir(t, filepath.Join(e.project, "backups")); len(files) != 0 {
		t.Fatalf("中断后不能留下任何备份或临时文件: %v", files)
	}
	e.assertClean(t, out)
}

func TestDBScripts_RestoreInterruptWarnsPartialWrite(t *testing.T) {
	e := newOpsEnv(t)
	b := e.makeBackup(t)
	e.reset(t)
	e.extra = []string{"FAKE_IMPORT_SLEEP=1"}
	code, out := e.interruptWhenStarted(t, "scripts/db/restore.sh", "--file", b.dump, "--database", "ops_drill", "--confirm", "ops_drill@127.0.0.1:3306")
	if code != 130 || !strings.Contains(out, "可能已写入部分数据") {
		t.Fatalf("导入中被中断应以 130 退出并提示可能部分写入，实际 %d:\n%s", code, out)
	}
	if queries, _ := e.fakeFile("mysql.queries"); strings.Contains(strings.ToUpper(queries), "DROP") {
		t.Fatalf("中断后也不应执行任何 DROP:\n%s", queries)
	}
	e.assertClean(t, out)
}

// 在「落正式文件名」阶段（三次 ln：.meta → .sha256 → 数据文件）被中断：已经落下的正式文件必须一并删掉，
// 不留下半套看着像备份的文件，提示「未生成备份文件」。
func TestDBScripts_BackupInterruptDuringPublish(t *testing.T) {
	e := newOpsEnv(t)
	for _, at := range []string{"1", "2", "3"} {
		t.Run("第"+at+"次ln", func(t *testing.T) {
			os.RemoveAll(filepath.Join(e.project, "backups"))
			e.reset(t)
			e.extra = []string{"FAKE_LN_SLEEP_AT=" + at}
			code, out := e.interruptWhenStarted(t, "scripts/db/backup.sh", "--database", "ops_demo")
			if code != 130 || !strings.Contains(out, "未生成备份文件") {
				t.Fatalf("应以 130 退出并提示未生成备份文件，实际 %d:\n%s", code, out)
			}
			if files := listDir(t, filepath.Join(e.project, "backups")); len(files) != 0 {
				t.Fatalf("中断后不能留下任何正式或临时文件: %v", files)
			}
			e.assertClean(t, out)
		})
	}
}

// 真实 ln 已经成功、backup_done 还没置上时被中断（三个落盘点各测一次）：本次落下的正式文件按 inode
// 识别并删除，不留孤立的 .meta / .sha256 / .sql.gz，提示与「未生成备份文件」一致。
func TestDBScripts_BackupInterruptAfterPublishLink(t *testing.T) {
	e := newOpsEnv(t)
	for _, after := range []string{"1", "2", "3"} {
		t.Run("第"+after+"次ln之后", func(t *testing.T) {
			os.RemoveAll(filepath.Join(e.project, "backups"))
			e.reset(t)
			e.extra = []string{"FAKE_LN_SLEEP_AFTER=" + after}
			code, out := e.interruptWhenStarted(t, "scripts/db/backup.sh", "--database", "ops_demo")
			if code != 130 || !strings.Contains(out, "未生成备份文件") {
				t.Fatalf("应以 130 退出并提示未生成备份文件，实际 %d:\n%s", code, out)
			}
			if files := listDir(t, filepath.Join(e.project, "backups")); len(files) != 0 {
				t.Fatalf("真实 ln 成功后被中断，也不能留下任何正式或临时文件: %v", files)
			}
			e.assertClean(t, out)
		})
	}
}
