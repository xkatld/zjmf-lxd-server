#!/bin/bash

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; NC='\033[0m'

REPO="https://github.com/xkatld/zjmf-lxd-server"
VERSION=""
NAME="lxdweb"
DIR="/opt/$NAME"
CFG="$DIR/config.yaml"
SERVICE="/etc/systemd/system/$NAME.service"
DB_FILE="lxdweb.db"
FORCE=false
DELETE=false

log() { echo -e "$1"; }
ok() { log "${GREEN}[OK]${NC} $1"; }
info() { log "${BLUE}[INFO]${NC} $1"; }
warn() { log "${YELLOW}[WARN]${NC} $1"; }
err() { log "${RED}[ERR]${NC} $1"; exit 1; }

[[ $EUID -ne 0 ]] && err "请使用 root 运行"

while [[ $# -gt 0 ]]; do
  case $1 in
    -v|--version) VERSION="$2"; shift 2;;
    -f|--force) FORCE=true; shift;;
    -d|--delete) DELETE=true; shift;;
    -h|--help) echo "$0 -v 版本 [-f] [-d]"; exit 0;;
    *) err "未知参数 $1";;
  esac
done

if [[ $DELETE == true ]]; then
  echo "警告: 此操作将删除所有数据，包括数据库文件！"
  
  read -p "确定要继续吗? (y/N): " CONFIRM
  if [[ $CONFIRM != "y" && $CONFIRM != "Y" ]]; then
    ok "取消删除操作"
    exit 0
  fi
  
  systemctl stop $NAME 2>/dev/null || true
  systemctl disable $NAME 2>/dev/null || true
  rm -f "$SERVICE"
  systemctl daemon-reload
  
  WRAPPER="/usr/local/bin/lxdweb"
  if [[ -f "$WRAPPER" ]]; then
    rm -f "$WRAPPER"
    ok "已删除全局命令: lxdweb"
  fi
  
  if [[ -d "$DIR" ]]; then
    rm -rf "$DIR"
    ok "已删除 $NAME 服务和目录"
  else
    ok "目录 $DIR 不存在，无需删除"
  fi
  exit 0
fi

if [[ -z "$VERSION" ]]; then
  err "必须提供版本号参数，使用 -v 或 --version 指定版本"
fi

echo
echo "========================================"
echo "      步骤 1/6: 检测系统环境"
echo "========================================"
echo

info "检测操作系统..."
if [[ -f /etc/os-release ]]; then
  OS_NAME=$(grep '^NAME=' /etc/os-release | cut -d'"' -f2)
  OS_VERSION=$(grep '^VERSION_ID=' /etc/os-release | cut -d'"' -f2)
  echo "  系统: $OS_NAME ${OS_VERSION:-}"
else
  echo "  系统: 未知 Linux 发行版"
fi

info "检测系统架构..."
arch=$(uname -m)
case $arch in
  x86_64) 
    BIN="lxdweb-amd64"
    echo "  架构: amd64"
    ;;
  aarch64|arm64) 
    BIN="lxdweb-arm64"
    echo "  架构: arm64"
    ;;
  *) 
    err "不支持的架构: $arch，仅支持 amd64 和 arm64"
    ;;
esac

ok "系统环境检测完成"

echo
echo "========================================"
echo "      步骤 2/6: 检查版本状态"
echo "========================================"
echo

DOWNLOAD_URL="$REPO/releases/download/$VERSION/$BIN.zip"

UPGRADE=false
if [[ -d "$DIR" ]] && [[ -f "$DIR/version" ]]; then
  CUR=$(cat "$DIR/version")
  if [[ $CUR != "$VERSION" || $FORCE == true ]]; then
    UPGRADE=true
    info "检测到已安装版本: $CUR"
    info "执行升级操作: $CUR -> $VERSION"
  else
    ok "已是最新版本 $VERSION"
    exit 0
  fi
else
  info "未检测到已安装版本"
  info "执行全新安装: $VERSION"
fi

echo
echo "========================================"
echo "      步骤 3/6: 安装系统依赖"
echo "========================================"
echo

info "检测包管理器..."
pkg_manager=""
if command -v apt &> /dev/null; then
  pkg_manager="apt"
  echo "  使用: APT (Debian/Ubuntu)"
elif command -v dnf &> /dev/null; then
  pkg_manager="dnf"
  echo "  使用: DNF (Fedora/RHEL 8+)"
elif command -v yum &> /dev/null; then
  pkg_manager="yum"
  echo "  使用: YUM (RHEL/CentOS)"
elif command -v zypper &> /dev/null; then
  pkg_manager="zypper"
  echo "  使用: Zypper (openSUSE)"
elif command -v pacman &> /dev/null; then
  pkg_manager="pacman"
  echo "  使用: Pacman (Arch Linux)"
else
  err "未检测到支持的包管理器"
fi

info "更新软件包列表..."
case $pkg_manager in
  apt)
    apt update -y
    ;;
  dnf)
    dnf check-update -y || true
    ;;
  yum)
    yum check-update -y || true
    ;;
  zypper)
    zypper refresh
    ;;
  pacman)
    pacman -Sy --noconfirm
    ;;
esac

info "安装依赖包..."
case $pkg_manager in
  apt)
    apt install -y curl wget unzip zip openssl systemd || err "依赖安装失败"
    ;;
  dnf)
    dnf install -y curl wget unzip zip openssl systemd || err "依赖安装失败"
    ;;
  yum)
    yum install -y curl wget unzip zip openssl systemd || err "依赖安装失败"
    ;;
  zypper)
    zypper install -y curl wget unzip zip openssl systemd || err "依赖安装失败"
    ;;
  pacman)
    pacman -S --noconfirm curl wget unzip zip openssl systemd || err "依赖安装失败"
    ;;
esac

ok "系统依赖安装完成"

echo
echo "========================================"
echo "      步骤 4/6: 准备环境"
echo "========================================"
echo

if systemctl is-active --quiet $NAME 2>/dev/null; then
  info "停止当前服务..."
  systemctl stop $NAME 2>/dev/null || true
  ok "服务已停止"
else
  info "服务未运行，跳过停止操作"
fi

if [[ $UPGRADE == true ]]; then
  info "清理旧程序文件..."
  find "$DIR" -maxdepth 1 -type f ! -name "$DB_FILE" ! -name "$DB_FILE-shm" ! -name "$DB_FILE-wal" -delete 2>/dev/null || true
  for subdir in "$DIR"/*; do
    if [[ -d "$subdir" ]]; then
      rm -rf "$subdir" 2>/dev/null || true
    fi
  done
  ok "旧文件已清理（保留数据库）"
fi

info "创建安装目录..."
mkdir -p "$DIR"
ok "环境准备完成"

echo
echo "========================================"
echo "      步骤 5/6: 下载和安装程序"
echo "========================================"
echo

info "下载 $NAME $VERSION..."
TMP=$(mktemp -d)
wget -qO "$TMP/app.zip" "$DOWNLOAD_URL" || err "下载失败"

info "解压安装文件..."
unzip -qo "$TMP/app.zip" -d "$DIR"
chmod +x "$DIR/$BIN"
echo "$VERSION" > "$DIR/version"
rm -rf "$TMP"

ok "程序文件安装完成"

echo
echo "========================================"
echo "      步骤 6/6: 生成配置和启动服务"
echo "========================================"
echo

DEFAULT_PORT="3000"
DEFAULT_SESSION_SECRET=$(openssl rand -hex 32)
DEFAULT_ADMIN_USER="admin"
DEFAULT_ADMIN_PASS="admin123"

if [[ ! -f "$CFG" ]]; then
  info "配置文件不存在，开始配置向导..."
  echo
  
  read -p "Web 界面端口 [$DEFAULT_PORT]: " WEB_PORT
  WEB_PORT=${WEB_PORT:-$DEFAULT_PORT}
  
  read -p "Session 密钥 [自动生成]: " SESSION_SECRET
  SESSION_SECRET=${SESSION_SECRET:-$DEFAULT_SESSION_SECRET}
  
  echo
  info "配置管理员账户..."
  read -p "管理员用户名 [$DEFAULT_ADMIN_USER]: " ADMIN_USER
  ADMIN_USER=${ADMIN_USER:-$DEFAULT_ADMIN_USER}
  
  read -sp "管理员密码 [$DEFAULT_ADMIN_PASS]: " ADMIN_PASS
  echo
  ADMIN_PASS=${ADMIN_PASS:-$DEFAULT_ADMIN_PASS}
  
  info "生成配置文件..."
  cat > "$CFG" <<EOF
server:
  # 服务器监听地址
  address: "0.0.0.0:$WEB_PORT"
  # 运行模式: debug | release
  mode: "release"
  # 会话密钥
  session_secret: "$SESSION_SECRET"
  # 启用 HTTPS
  enable_https: true
  # 证书文件路径
  cert_file: "cert.pem"
  # 密钥文件路径
  key_file: "key.pem"

database:
  # 数据库文件路径
  path: "$DB_FILE"

admin:
  # 默认管理员账户
  username: "$ADMIN_USER"
  password: "$ADMIN_PASS"

logging:
  # 日志级别: debug | info | warn | error
  level: "error"
  # 日志文件保存路径
  file: "lxdweb.log"
  # 单个日志文件最大大小（MB）
  max_size: 10
  # 保留的旧日志文件数量
  max_backups: 2
  # 保留的旧日志文件天数
  max_age: 30
  # 是否压缩旧日志文件
  compress: true
  # 开发模式（控制台输出格式）
  dev_mode: false
EOF
  ok "配置文件已创建 (HTTPS已启用)"
else
  info "配置文件已存在，跳过配置向导"
fi

info "生成 systemd 服务文件..."
cat > "$SERVICE" <<EOF
[Unit]
Description=lxdweb-xkatld
After=network.target

[Service]
WorkingDirectory=$DIR
ExecStart=$DIR/$BIN
Restart=always
RestartSec=5
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF
ok "服务文件已生成"

info "创建全局命令..."
WRAPPER="/usr/local/bin/lxdweb"
cat > "$WRAPPER" <<'WRAPPER_EOF'
#!/bin/bash
# lxdweb 命令行工具包装脚本
DIR="/opt/lxdweb"
BIN=$(ls "$DIR"/lxdweb-* 2>/dev/null | head -1)

if [[ -z "$BIN" || ! -x "$BIN" ]]; then
  echo "错误: 未找到 lxdweb 可执行文件" >&2
  exit 1
fi

cd "$DIR" && exec "$BIN" "$@"
WRAPPER_EOF

chmod +x "$WRAPPER"
ok "全局命令已创建: lxdweb"

info "启动服务..."
systemctl daemon-reload
systemctl enable --now $NAME
ok "服务已启动"

info "等待服务稳定..."
sleep 3

echo
echo "========================================"
echo "          安装完成"
echo "========================================"
echo
echo "数据目录: $DIR"
echo "Web 端口: $(grep 'address:' $CFG | awk -F: '{print $NF}' | tr -d ' "')"
echo "配置文件: $CFG"
echo "数据库: SQLite ($DB_FILE)"
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "管理员账户:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ADMIN_USER_DISPLAY=$(grep 'username:' "$CFG" | grep -A1 'admin:' | tail -1 | awk -F'"' '{print $2}')
echo "  用户名: ${ADMIN_USER_DISPLAY:-admin}"
echo "  密码:   (已在配置向导中设置)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
echo "========================================"
echo "服务状态:"
echo "========================================"
systemctl status $NAME --no-pager

