#!/usr/bin/env bash
# 把 backup.sh 生成的 .sql.gz 恢复到一个「不存在」或「空」的库。
#
# 用法（在项目根目录）：
#   MYSQL_OPS_USER=<账号> MYSQL_OPS_PASSWORD=<密码> scripts/db/restore.sh \
#     --file backups/<库名>-<时间>.sql.gz --database <目标库> --confirm <目标库>@<主机>:<端口> [--create]
# 或 make db-restore BACKUP_FILE=... DB=<目标库> CONFIRM=<目标库>@<主机>:<端口> [CREATE=1]
#
# 前提：只恢复**本项目 backup.sh 生成、并且一直存放在可信位置**的备份。导入就是把文件里的 SQL
# 原样交给 mysql 执行，脚本不能、也不尝试把一份不可信的 SQL 变安全；.sha256 只用来发现
# 传输 / 存储损坏，能改备份文件的人同样能改 .sha256，它不是来源认证。
#
# 安全约定：
#   - 目标库非空（有表、视图、存储过程或事件）一律拒绝，不提供覆盖开关；脚本从不 DROP 任何库。
#   - --confirm 必须逐字等于由参数拼出的「库@主机:端口」，防止填错目标；它只核对参数，
#     不能证明服务端身份 —— 连上以后会打印服务端的 hostname / port / server_uuid 供人核对。
#   - 只有加 --create 才会在库不存在时创建它。
#   - 导入用 mysql --binary-mode：非交互输入下禁用 \! system、source、tee 等客户端命令
#     （只保留 mysqldump 输出需要的 delimiter / charset）。--one-database 只是按 USE 语句过滤，
#     **不是**隔离手段：写成「其它库.表」的语句照样执行。
#   - 备份含库级对象（视图、存储过程 / 函数、触发器、事件）时，只允许恢复到与源库**同名**的库：
#     这些定义里可能写死了源库名，恢复到别的库名会读写到源库。要在源库还在的情况下演练，
#     请用另一个 MySQL 实例上的同名库。
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
	sed -n '2,27p' "$0" | sed 's/^# \{0,1\}//'
}

file=""
database=""
confirm=""
create=0
skip_checksum=0
while [ $# -gt 0 ]; do
	case $1 in
	--file | --database | --confirm)
		[ $# -ge 2 ] || dbops_die "$1 需要一个值"
		case $1 in
		--file) file=$2 ;;
		--database) database=$2 ;;
		--confirm) confirm=$2 ;;
		esac
		shift 2
		;;
	--create)
		create=1
		shift
		;;
	--skip-checksum)
		skip_checksum=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) dbops_die "未知参数: $1（--help 查看用法）" ;;
	esac
done

[ -n "$file" ] || dbops_die "必须指定 --file"
[ -f "$file" ] && [ -r "$file" ] || dbops_die "备份文件不存在或不可读: $file"
case $file in
*.sql.gz) ;;
*) dbops_die "只接受 backup.sh 生成的 .sql.gz 文件: $file" ;;
esac
dbops_validate_db_name "$database"
dbops_load_connection
dbops_require_tools mysql gzip awk
dbops_require_mysql_client mysql

target="$database@$DBOPS_HOST:$DBOPS_PORT"
if [ "$confirm" != "$target" ]; then
	dbops_die "目标核验失败：--confirm 必须逐字等于 '$target'（当前为 '${confirm}'）"
fi

DBOPS_PRIVATE_DIR=""
importing=0
on_signal() {
	if [ "$importing" -eq 1 ]; then
		dbops_info "已中断：$target 可能已写入部分数据。脚本不会删除任何库，请人工检查后自行处理"
	else
		dbops_info "已中断：尚未开始导入"
	fi
	exit "$1"
}
trap dbops_cleanup_config EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

# ---- 先校验文件，再碰数据库 ----

# .meta 由 backup.sh 生成；没有它说明不是本脚本支持的备份。
[ -f "$file.meta" ] || dbops_die "找不到 $file.meta：只支持恢复 backup.sh 生成的备份"
meta_format=""
meta_source_db=""
meta_source_uuid=""
meta_markers=""
while IFS='=' read -r key value; do
	case $key in
	format) meta_format=$value ;;
	source_database) meta_source_db=$value ;;
	source_server_uuid) meta_source_uuid=$value ;;
	stored_object_markers) meta_markers=$value ;;
	esac
done <"$file.meta"
[ "$meta_format" = "taurus-dbops-mysqldump-1" ] || dbops_die "$file.meta 格式不受支持: '$meta_format'"
case $meta_source_db in
'' | *[!A-Za-z0-9_]*) dbops_die "$file.meta 里的 source_database 不合法" ;;
esac
case $meta_source_uuid in
'' | *[!0-9a-fA-F-]*) dbops_die "$file.meta 里的 source_server_uuid 不合法" ;;
esac
case $meta_markers in
'' | *[!0-9]*) dbops_die "$file.meta 里的 stored_object_markers 不合法" ;;
esac

if [ -f "$file.sha256" ]; then
	base=$(basename -- "$file")
	expected=$(awk -v f="$base" '$2 == f { print $1 }' "$file.sha256")
	expected_meta=$(awk -v f="$base.meta" '$2 == f { print $1 }' "$file.sha256")
	[ -n "$expected" ] && [ "$expected" = "$(dbops_sha256 "$file")" ] ||
		dbops_die "sha256 不匹配，备份文件可能损坏: $file"
	[ -n "$expected_meta" ] && [ "$expected_meta" = "$(dbops_sha256 "$file.meta")" ] ||
		dbops_die "sha256 不匹配，元数据文件可能损坏: $file.meta"
elif [ "$skip_checksum" -ne 1 ]; then
	dbops_die "找不到 $file.sha256；确认文件来自可信存储后可加 --skip-checksum"
else
	dbops_info "警告：跳过 sha256 校验（--skip-checksum）"
fi
dbops_verify_dump "$file"

if [ "$DBOPS_DUMP_STORED_MARKERS" != "$meta_markers" ]; then
	dbops_die "备份内容与 $file.meta 不一致（库级对象标记 $DBOPS_DUMP_STORED_MARKERS ≠ $meta_markers），拒绝恢复"
fi
if [ "$DBOPS_DUMP_STORED_MARKERS" -gt 0 ] && [ "$database" != "$meta_source_db" ]; then
	dbops_die "备份含视图 / 存储过程 / 触发器 / 事件，其定义可能写死源库名 '$meta_source_db'；只支持恢复到同名库（源库还在时请用另一个 MySQL 实例）"
fi

# ---- 连库 ----

dbops_write_client_config
dbops_server_identity
dbops_print_identity "目标实例（请核对）"
if [ "$DBOPS_SERVER_UUID" = "$meta_source_uuid" ]; then
	dbops_info "注意：目标实例与备份的源实例是同一个（server_uuid 相同）"
fi

# 库名已经过白名单校验（只含字母数字下划线），可以安全拼进 SQL。
exists=$(dbops_query exists "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '$database'")
case $exists in
0)
	[ "$create" -eq 1 ] || dbops_die "目标库 $database 不存在；确认要新建时加 --create"
	dbops_info "创建库 $database ..."
	dbops_query create "CREATE DATABASE \`$database\` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci"
	;;
1) ;;
*) dbops_die "无法确认目标库是否存在（查询返回 '$exists'）" ;;
esac

# 非空判定只看本账号可见的对象：恢复账号必须对目标库有完整权限（见 README.md「备份与恢复」）。
objects=$(dbops_query objects "SELECT
  (SELECT COUNT(*) FROM information_schema.TABLES   WHERE TABLE_SCHEMA   = '$database')
+ (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = '$database')
+ (SELECT COUNT(*) FROM information_schema.EVENTS   WHERE EVENT_SCHEMA   = '$database')")
case $objects in
0) ;;
'' | *[!0-9]*) dbops_die "无法确认目标库是否为空（查询返回 '$objects'）" ;;
*) dbops_die "目标库 $database 非空（$objects 个对象），拒绝恢复。本脚本不覆盖已有数据：请恢复到一个新库，核对后再切换" ;;
esac

started=$(date +%s)
dbops_info "开始恢复 $file → $target ..."
importing=1
# dbops_mysql 统一加上连接参数（--defaults-file 第一、其后 --protocol/--host/--port/--user）和 --local-infile=0。
# --binary-mode：禁用客户端命令（\! system / source / tee 等），见文件头说明。
# --one-database：按 USE 语句过滤的附加防护，不是隔离。
if ! gzip -dc -- "$file" | dbops_mysql --binary-mode --one-database --default-character-set=utf8mb4 "$database"; then
	dbops_die "恢复失败：$target 可能已写入部分数据。脚本不会删除任何库，请人工检查后自行处理"
fi
importing=0

tables=$(dbops_query tables "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$database'")
printf '恢复完成\n'
printf '  目标:    %s（server_uuid=%s）\n' "$target" "$DBOPS_SERVER_UUID"
printf '  文件:    %s\n' "$file"
printf '  源库:    %s（server_uuid=%s）\n' "$meta_source_db" "$meta_source_uuid"
printf '  表数量:  %s\n' "$tables"
printf '  耗时:    %s 秒\n' "$(($(date +%s) - started))"
