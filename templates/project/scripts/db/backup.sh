#!/usr/bin/env bash
# MySQL 逻辑备份：mysqldump → gzip，校验通过后才以原子方式落成最终文件，并写 .meta 与 .sha256。
#
# 用法（在项目根目录）：
#   MYSQL_OPS_USER=<账号> MYSQL_OPS_PASSWORD=<密码> scripts/db/backup.sh --database <库名> [--output-dir backups]
# 或 make db-backup DB=<库名>（连接参数同样走环境变量）。
#
# 一致性：--single-transaction 对 InnoDB 表给出一致性快照，不锁表；本项目的表都是 InnoDB。
# 限制：备份期间不要做 DDL（ALTER / DROP / RENAME / TRUNCATE），否则快照可能不一致或导出报错；
# MyISAM 等非事务表不在快照保护内。详见项目 README.md「备份与恢复」。
set -euo pipefail

# 在调用任何外部命令之前拿走密码：脚本目录只用 shell 内建计算（不调 dirname），source 之后
# 第一件事就是把 MYSQL_OPS_PASSWORD 移出环境，之后的 sed / gzip / awk / mysql 等子进程都不会继承它。
case $0 in
*/*) SCRIPT_DIR=${0%/*} ;;
*) SCRIPT_DIR=. ;;
esac
# shellcheck source=common.sh
. "$SCRIPT_DIR/common.sh"
dbops_take_password

usage() {
	sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
}

database=""
output_dir="backups"
while [ $# -gt 0 ]; do
	case $1 in
	--database)
		[ $# -ge 2 ] || dbops_die "--database 需要一个值"
		database=$2
		shift 2
		;;
	--output-dir)
		[ $# -ge 2 ] || dbops_die "--output-dir 需要一个值"
		output_dir=$2
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) dbops_die "未知参数: $1（--help 查看用法）" ;;
	esac
done

dbops_validate_db_name "$database"
[ -n "$output_dir" ] || dbops_die "--output-dir 不能为空"
dbops_load_connection
dbops_require_tools mysql mysqldump gzip awk
dbops_require_mysql_client mysqldump

DBOPS_PRIVATE_DIR=""
partial=""
partial_meta=""
partial_sum=""
final=""
# backup_done=1 表示三个正式文件都已落好，备份完成。
backup_done=0

# remove_if_ours <正式名> <本次的临时文件>：正式名与本次临时文件是同一个文件（同一 inode，
# 即本次 ln 落下的硬链接）时才删。不按名字无条件删：同名但不是本次写的文件不会被误删；
# 也不依赖「ln 成功后再登记」—— ln 刚成功、还没来得及登记时收到信号，同样能识别出来。
remove_if_ours() {
	if [ -n "$2" ] && [ -e "$2" ] && [ "$1" -ef "$2" ]; then
		rm -f -- "$1"
	fi
}
cleanup() {
	# 没完成就退出（失败或中断）：先删本次已落的正式文件，再删临时文件（删了临时文件就没法按 inode 识别了）。
	if [ "$backup_done" -ne 1 ] && [ -n "$final" ]; then
		remove_if_ours "$final.meta" "$partial_meta"
		remove_if_ours "$final.sha256" "$partial_sum"
		remove_if_ours "$final" "$partial"
	fi
	dbops_cleanup_config
	local f
	for f in "$partial" "$partial_meta" "$partial_sum"; do
		if [ -n "$f" ]; then
			rm -f -- "$f"
		fi
	done
}
on_signal() {
	if [ "$backup_done" -eq 1 ]; then
		dbops_info "已中断：备份在中断前已完成: $final"
	else
		dbops_info "已中断：未生成备份文件（临时文件和已写出的部分文件已删除）"
	fi
	exit "$1"
}
trap cleanup EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

umask 077
mkdir -p -- "$output_dir"
stamp=$(date +%Y%m%d-%H%M%S)
final="$output_dir/$database-$stamp.sql.gz"
[ ! -e "$final" ] || dbops_die "目标文件已存在，拒绝覆盖: $final"

dbops_write_client_config
dbops_server_identity
dbops_print_identity "源实例"

# 临时文件与最终文件在同一目录（同一文件系统），
# 任何一步失败，最终文件名都不会出现（临时文件由 trap 删除）。
partial=$(mktemp "$output_dir/.$database-$stamp.partial.XXXXXX")
partial_meta=$(mktemp "$output_dir/.$database-$stamp.meta.XXXXXX")
partial_sum=$(mktemp "$output_dir/.$database-$stamp.sha256.XXXXXX")

started=$(date +%s)
dbops_info "开始备份 $database@$DBOPS_HOST:$DBOPS_PORT ..."
# 连接参数（--defaults-file 第一、其后 --protocol/--host/--port/--user）由 dbops_mysqldump 统一加上。只传库名（不用多库模式）：导出里不带 CREATE DATABASE / USE，
# 恢复时由 restore.sh 显式指定目标库，才能恢复到独立的测试库。
if ! dbops_mysqldump \
	--single-transaction --quick \
	--routines --triggers --events \
	--hex-blob --no-tablespaces --set-gtid-purged=OFF \
	--default-character-set=utf8mb4 \
	"$database" | gzip -c >"$partial"; then
	dbops_die "mysqldump 或 gzip 失败，未生成备份文件"
fi

dbops_verify_dump "$partial"

# .meta 记录这份备份的来源，restore.sh 据此决定恢复约束（见 restore.sh 的库级对象说明）。
{
	printf 'format=taurus-dbops-mysqldump-1\n'
	printf 'source_database=%s\n' "$database"
	printf 'source_hostname=%s\n' "$DBOPS_SERVER_HOSTNAME"
	printf 'source_port=%s\n' "$DBOPS_SERVER_PORT"
	printf 'source_server_uuid=%s\n' "$DBOPS_SERVER_UUID"
	printf 'stored_object_markers=%s\n' "$DBOPS_DUMP_STORED_MARKERS"
	printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$partial_meta"

# .sha256 覆盖数据文件和 .meta，只用来发现损坏，不是来源认证（见 README.md「备份与恢复」）。
sum=$(dbops_sha256 "$partial")
meta_sum=$(dbops_sha256 "$partial_meta")
{
	printf '%s  %s\n' "$sum" "$(basename -- "$final")"
	printf '%s  %s\n' "$meta_sum" "$(basename -- "$final").meta"
} >"$partial_sum"

# 用硬链接落最终文件名：ln 在目标已存在时失败（不覆盖），且是原子的。数据文件最后落。
# 任何一步失败或被中断（包括 ln 已成功、backup_done 还没置上的瞬间），cleanup 按 inode 删掉本次已落的正式文件。
[ ! -e "$final.meta" ] && [ ! -e "$final.sha256" ] || dbops_die "目标文件已存在，拒绝覆盖: $final.meta / .sha256"
ln -- "$partial_meta" "$final.meta" || dbops_die "无法写入 $final.meta"
ln -- "$partial_sum" "$final.sha256" || dbops_die "无法写入 $final.sha256"
ln -- "$partial" "$final" || dbops_die "无法写入 $final"
backup_done=1

size=$(wc -c <"$final" | tr -d ' ')
printf '备份完成\n'
printf '  库:          %s@%s:%s（server_uuid=%s）\n' "$database" "$DBOPS_HOST" "$DBOPS_PORT" "$DBOPS_SERVER_UUID"
printf '  文件:        %s（%s 字节）\n' "$final" "$size"
printf '  sha256:      %s\n' "$sum"
printf '  库级对象:    %s\n' "$([ "$DBOPS_DUMP_STORED_MARKERS" -eq 0 ] && echo '无（只有表和数据）' || echo "有（视图/存储过程/触发器/事件，恢复只能用同名库，见 README.md「备份与恢复」）")"
printf '  耗时:        %s 秒\n' "$(($(date +%s) - started))"
