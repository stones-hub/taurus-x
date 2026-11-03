# shellcheck shell=bash
# backup.sh / restore.sh 共用的函数，只被它们 source，不单独执行。
#
# 连接参数只从环境变量读：
#   MYSQL_OPS_HOST      默认 127.0.0.1
#   MYSQL_OPS_PORT      默认 3306
#   MYSQL_OPS_USER      必填，不提供默认账号
#   MYSQL_OPS_PASSWORD  必须显式设置（无密码账号写成 MYSQL_OPS_PASSWORD= ）
#
# 怎么交给 mysql / mysqldump（见 dbops_conn_args）：
#   - 主机、端口、账号、协议不敏感，放在命令行上：--defaults-file=<临时文件> --protocol=TCP
#     --host --port --user。命令行参数优先级高于任何选项文件，包括 mysql_config_editor 写的
#     ~/.mylogin.cnf（它会覆盖 --defaults-file 里的同名项）。
#   - 密码只写进私有临时目录里一个权限 600 的选项文件，经 --defaults-file 交给客户端，
#     不出现在命令行参数（ps 可见）里，也不打印。退出时（含失败、Ctrl+C）删除整个临时目录。
#   - 每次调用都带 MYSQL_TEST_LOGIN_FILE=<临时目录里一个不存在的路径>：客户端因此不读用户的
#     ~/.mylogin.cnf（连同里面可能存的密码），只用这里给定的参数。
#   - MYSQL_OPS_PASSWORD 在脚本开头（dbops_take_password，任何外部命令之前）就被复制到不导出的
#     shell 变量并 unset，之后启动的任何子进程（sha256 / gzip / awk / mysql / mysqldump）环境里都没有密码。
#
# 只支持本机已安装的 MySQL 8.x 客户端（mysql / mysqldump），不支持 docker exec。

# 即使调用方的 shell 开了 allexport（set -a，或经 SHELLOPTS 继承），这里新建的变量也不自动导出：
# 否则下面复制出来的密码变量会被传给所有子进程。
set +a

dbops_die() {
	printf '错误: %s\n' "$*" >&2
	exit 1
}

dbops_info() {
	printf '%s\n' "$*" >&2
}

# 库名只允许字母、数字、下划线：它会被拼进 SQL 和文件名，白名单比转义可靠。
dbops_validate_db_name() {
	case $1 in
	'' | *[!A-Za-z0-9_]*) dbops_die "库名只能由字母、数字、下划线组成: '$1'" ;;
	esac
	[ "${#1}" -le 64 ] || dbops_die "库名超过 64 个字符"
}

# 把密码从环境移到不导出的私有变量。backup.sh / restore.sh 在 source 本文件后、调用任何外部命令之前执行。
dbops_take_password() {
	if [ -n "${MYSQL_OPS_PASSWORD+set}" ]; then
		DBOPS_PASSWORD=$MYSQL_OPS_PASSWORD
		DBOPS_PASSWORD_SET=1
	else
		DBOPS_PASSWORD=""
		DBOPS_PASSWORD_SET=0
	fi
	export -n DBOPS_PASSWORD
	unset MYSQL_OPS_PASSWORD
}

dbops_load_connection() {
	DBOPS_HOST=${MYSQL_OPS_HOST:-127.0.0.1}
	DBOPS_PORT=${MYSQL_OPS_PORT:-3306}
	DBOPS_USER=${MYSQL_OPS_USER:-}
	[ -n "$DBOPS_USER" ] || dbops_die "必须设置 MYSQL_OPS_USER（脚本不提供默认账号）"
	if [ "${DBOPS_PASSWORD_SET:-0}" -ne 1 ]; then
		dbops_die "必须设置 MYSQL_OPS_PASSWORD（无密码账号请显式写 MYSQL_OPS_PASSWORD= ）"
	fi
	case $DBOPS_HOST in
	'' | *[!A-Za-z0-9._:-]*) dbops_die "MYSQL_OPS_HOST 含非法字符" ;;
	esac
	case $DBOPS_PORT in
	'' | *[!0-9]*) dbops_die "MYSQL_OPS_PORT 必须是数字" ;;
	esac
	case $DBOPS_USER in
	*[!A-Za-z0-9._-]*) dbops_die "MYSQL_OPS_USER 只能由字母、数字、点、下划线、短横线组成" ;;
	esac
	case $DBOPS_PASSWORD in
	*$'\n'* | *$'\r'*) dbops_die "MYSQL_OPS_PASSWORD 不能包含换行" ;;
	esac
}

dbops_require_tools() {
	local tool
	for tool in "$@"; do
		command -v "$tool" >/dev/null 2>&1 || dbops_die "找不到 $tool，请先安装 MySQL 8.x 客户端 / 对应工具"
	done
}

# MariaDB 的 mysqldump 不认 --set-gtid-purged 等参数，导出的语法也与 MySQL 8 有差异，直接拒绝。
dbops_require_mysql_client() {
	local version
	version=$("$1" --version 2>&1) || dbops_die "$1 --version 执行失败"
	case $version in
	*MariaDB*) dbops_die "$1 是 MariaDB 客户端，本脚本只支持 MySQL 8.x 客户端" ;;
	esac
}

# 写临时选项文件：只放密码（总是写，空密码写成 password=""）。值用双引号包起来，并转义反斜杠和双引号（MySQL 选项文件的转义规则），
# 这样密码里出现 # ; 空格等字符时不会被截断成注释。文件放在 mktemp -d 建的私有目录（700）里。
dbops_write_client_config() {
	local escaped
	umask 077
	DBOPS_PRIVATE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/dbops.XXXXXX") || dbops_die "无法创建临时目录"
	chmod 700 "$DBOPS_PRIVATE_DIR"
	DBOPS_CNF="$DBOPS_PRIVATE_DIR/client.cnf"
	# 这个路径永远不创建：MYSQL_TEST_LOGIN_FILE 指向它，客户端就找不到可读的 login 文件。
	DBOPS_NO_LOGIN_FILE="$DBOPS_PRIVATE_DIR/no-login-file"
	escaped=${DBOPS_PASSWORD//\\/\\\\}
	escaped=${escaped//\"/\\\"}
	# 无密码账号也写 password=""（显式空密码）：不写这一行时客户端没有任何密码选项，连接时可能退回去读
	# 调用方环境里为别的实例设置的客户端密码环境变量；只写 password（不带 =）则会交互式提示输入。
	{
		printf '[client]\n'
		printf 'password="%s"\n' "$escaped"
	} >"$DBOPS_CNF"
	chmod 600 "$DBOPS_CNF"
	unset DBOPS_PASSWORD escaped
	dbops_conn_args
}

# 每条 mysql / mysqldump 命令共用的连接参数。顺序固定：--defaults-file 必须是第一个参数
# （客户端只在第一个位置识别它），其后的命令行参数覆盖任何选项文件里的同名项。
dbops_conn_args() {
	DBOPS_CONN=(
		--defaults-file="$DBOPS_CNF"
		--protocol=TCP
		--host="$DBOPS_HOST"
		--port="$DBOPS_PORT"
		--user="$DBOPS_USER"
	)
}

# mysql：再加 --local-infile=0，SQL 里的 LOAD DATA LOCAL INFILE 读不到运维机上的文件。
dbops_mysql() {
	MYSQL_TEST_LOGIN_FILE="$DBOPS_NO_LOGIN_FILE" mysql "${DBOPS_CONN[@]}" --local-infile=0 "$@"
}

dbops_mysqldump() {
	MYSQL_TEST_LOGIN_FILE="$DBOPS_NO_LOGIN_FILE" mysqldump "${DBOPS_CONN[@]}" "$@"
}

dbops_cleanup_config() {
	if [ -n "${DBOPS_PRIVATE_DIR:-}" ]; then
		rm -rf -- "$DBOPS_PRIVATE_DIR"
	fi
}

# 执行一条查询。第一个参数是查询标签（只出现在 SQL 注释里，便于日志和测试辨认）。
dbops_query() {
	dbops_mysql --batch --skip-column-names -e "/* dbops:$1 */ $2"
}

# 读取实际连上的服务端身份：@@hostname / @@port / @@server_uuid。
# --confirm 只核对「参数拼出来的目标」；这三个值来自服务端本身，打印出来供人核对
# 确实连到了想连的那个实例（例如测试实例而不是生产实例）。都不是敏感信息。
dbops_server_identity() {
	local out
	out=$(dbops_query identity "SELECT @@hostname, @@port, @@server_uuid") || dbops_die "无法读取服务端身份"
	IFS=$'\t' read -r DBOPS_SERVER_HOSTNAME DBOPS_SERVER_PORT DBOPS_SERVER_UUID <<EOF
$out
EOF
	case $DBOPS_SERVER_UUID in
	'' | *[!0-9a-fA-F-]*) dbops_die "服务端返回的 server_uuid 不合法: '$DBOPS_SERVER_UUID'" ;;
	esac
	case $DBOPS_SERVER_PORT in
	'' | *[!0-9]*) dbops_die "服务端返回的端口不合法" ;;
	esac
	case $DBOPS_SERVER_HOSTNAME in
	'' | *[!A-Za-z0-9._-]*) DBOPS_SERVER_HOSTNAME='(含非常规字符，未显示)' ;;
	esac
}

dbops_print_identity() {
	dbops_info "$1: hostname=$DBOPS_SERVER_HOSTNAME port=$DBOPS_SERVER_PORT server_uuid=$DBOPS_SERVER_UUID"
}

dbops_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -- "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 -- "$1" | awk '{print $1}'
	else
		dbops_die "找不到 sha256sum 或 shasum"
	fi
}

# 校验备份文件，并统计「库级对象」标记行，结果放在 DBOPS_DUMP_STORED_MARKERS。
#
#   - gzip -t 通过；
#   - 最后一行是 mysqldump 的 '-- Dump completed' 结尾（中途被杀的导出没有这一行）；
#   - 库级对象标记：视图、存储过程 / 函数、触发器、事件的定义，以及 mysqldump 在它们两侧
#     输出的 ALTER DATABASE。这些定义可能写死了**源库名**（视图里的 `源库`.`表`、
#     ALTER DATABASE `源库`、例程体里的 `源库`.`表`），恢复到别的库名时会读写到源库，见 restore.sh。
#
# 认两种已知写法（同一个对象命中多条规则时会多计，只影响计数大小，不影响「有 / 无」）：
#   - 带版本注释的：/*!50003 CREATE*/ ... PROCEDURE / TRIGGER，/*!50106 CREATE*/ ... EVENT，
#     /*!50001 CREATE ... VIEW，/*!50001 VIEW ...；
#   - 不带版本注释的：CREATE DEFINER=... PROCEDURE / FUNCTION / TRIGGER / EVENT / VIEW，
#     以及没有 DEFINER 子句的 CREATE PROCEDURE / FUNCTION / TRIGGER / EVENT / VIEW、CREATE ALGORITHM= /
#     CREATE SQL SECURITY（视图）。具体 MySQL 8 版本实际输出哪一种，要用真库导出确认。
#   普通表是 CREATE TABLE，数据是 INSERT INTO，都不会命中。
#
# 这是**格式检查**，用来判断这份备份属于哪种恢复约束，不是 SQL 安全过滤：
# 它只认 mysqldump 自己的输出格式（每条语句一行开头），防不了刻意构造的 SQL。
dbops_verify_dump() {
	local out last
	gzip -t -- "$1" || dbops_die "gzip 校验失败: $1"
	out=$(gzip -dc -- "$1" | awk '
		/^\/\*![0-9][0-9][0-9][0-9][0-9] (CREATE|VIEW)/ { n++ }
		/^(\/\*![0-9][0-9][0-9][0-9][0-9] )?ALTER DATABASE/ { n++ }
		/^CREATE DEFINER=/ { n++ }
		/^CREATE (PROCEDURE|FUNCTION|TRIGGER|EVENT|VIEW) / { n++ }
		/^CREATE (ALGORITHM=|SQL SECURITY )/ { n++ }
		{ last = $0 }
		END { print n + 0; print last }') || dbops_die "无法读取备份内容: $1"
	DBOPS_DUMP_STORED_MARKERS=${out%%$'\n'*}
	last=${out#*$'\n'}
	case $last in
	'-- Dump completed'*) ;;
	*) dbops_die "备份不完整（缺少 mysqldump 的 '-- Dump completed' 结尾）: $1" ;;
	esac
}
