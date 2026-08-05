#!/bin/bash

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; NC='\033[0m'

REPO="https://github.com/xkatld/zjmf-lxd-server"
VERSION=""
NAME="lxdapi"
DIR="/opt/$NAME"
CFG="$DIR/config.yaml"
SERVICE="/etc/systemd/system/$NAME.service"
DB_FILE="lxdapi.db"
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
    -v|--version) VERSION="$2"; [[ $VERSION != v* ]] && VERSION="v$VERSION"; shift 2;;
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

arch=$(uname -m)
case $arch in
  x86_64) BIN="lxdapi-amd64" ;;
  aarch64|arm64) BIN="lxdapi-arm64" ;;
  *) err "不支持的架构: $arch，仅支持 amd64 和 arm64" ;;
esac

DOWNLOAD_URL="$REPO/releases/download/$VERSION/$BIN.zip"

UPGRADE=false
if [[ -d "$DIR" ]] && [[ -f "$DIR/version" ]]; then
  CUR=$(cat "$DIR/version")
  if [[ $CUR != "$VERSION" || $FORCE == true ]]; then
    UPGRADE=true
  else
    ok "已是最新版本 $VERSION"
    exit 0
  fi
fi

echo
echo "========================================"
echo "      步骤 1/4: 安装软件包"
echo "========================================"
echo

info "检测操作系统..."
if [[ ! -f /etc/os-release ]]; then
  err "无法检测操作系统"
fi

OS_ID=$(grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"')
if [[ "$OS_ID" != "ubuntu" && "$OS_ID" != "debian" ]]; then
  err "不支持的操作系统: $OS_ID"
fi

ok "操作系统检查通过: $OS_ID"

info "更新软件包列表..."
apt update -y || err "更新失败"

info "安装依赖包..."
DEBIAN_FRONTEND=noninteractive apt install -y curl wget unzip zip openssl xxd systemd iptables-persistent || err "安装失败"

ok "软件包安装完成"

echo
echo "========================================"
echo "      步骤 2/4: 准备环境"
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
echo "      步骤 3/4: 下载和安装程序"
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

get_default_interface() {
  ip route | grep default | head -1 | awk '{print $5}' || echo "eth0"
}

get_interface_ipv4() {
  local interface="$1"
  ip -4 addr show "$interface" 2>/dev/null | grep inet | grep -v 127.0.0.1 | head -1 | awk '{print $2}' | cut -d/ -f1 || echo ""
}

get_interface_ipv6() {
  local interface="$1"
  ip -6 addr show "$interface" 2>/dev/null | grep inet6 | grep -v "::1" | grep -v "fe80" | head -1 | awk '{print $2}' | cut -d/ -f1 || echo ""
}

DEFAULT_INTERFACE=$(get_default_interface)
DEFAULT_IPV4=$(get_interface_ipv4 "$DEFAULT_INTERFACE")
DEFAULT_IPV6=$(get_interface_ipv6 "$DEFAULT_INTERFACE")
DEFAULT_IP=$(curl -s 4.ipw.cn || echo "$DEFAULT_IPV4")
DEFAULT_HASH=$(openssl rand -hex 8 | tr 'a-f' 'A-F')
DEFAULT_PORT="8080"

echo
echo "========================================"
echo "      步骤 4/4: 配置向导"
echo "========================================"
echo
echo "    LXD API 服务配置向导 - $VERSION"
echo

echo "==== 步骤 1/6: 基础信息配置 ===="
echo

read -p "服务器外网 IP [$DEFAULT_IP]: " EXTERNAL_IP
EXTERNAL_IP=${EXTERNAL_IP:-$DEFAULT_IP}

read -p "API 访问密钥 [$DEFAULT_HASH]: " API_HASH
API_HASH=${API_HASH:-$DEFAULT_HASH}

read -p "API 服务端口 [$DEFAULT_PORT]: " SERVER_PORT
SERVER_PORT=${SERVER_PORT:-$DEFAULT_PORT}

ok "基础信息配置完成"
echo

echo "==== 步骤 2/6: 存储池配置 ===="
echo

DETECTED_POOLS_LIST=$(lxc storage list --format csv 2>/dev/null | cut -d, -f1 | grep -v "^NAME$" | head -10)
if [[ -n "$DETECTED_POOLS_LIST" ]]; then
  echo "检测到的存储池："
  echo "$DETECTED_POOLS_LIST" | sed 's/^/  - /'
else
  warn "未检测到存储池"
fi
echo
echo "存储池配置方式："
echo "1. 自动使用所有检测到的存储池"
echo "2. 手动指定存储池列表"
echo
read -p "请选择 [1-2]: " STORAGE_MODE

while [[ ! $STORAGE_MODE =~ ^[1-2]$ ]]; do
  warn "无效选择，请输入 1 或 2"
  read -p "请选择 [1-2]: " STORAGE_MODE
done

case $STORAGE_MODE in
  1)
    DETECTED_POOLS=$(lxc storage list --format csv 2>/dev/null | cut -d, -f1 | grep -v "^NAME$" | head -10 | tr '\n' ' ')
    if [[ -n "$DETECTED_POOLS" ]]; then
      STORAGE_POOLS=""
      for pool in $DETECTED_POOLS; do
        if [[ -n "$STORAGE_POOLS" ]]; then
          STORAGE_POOLS="$STORAGE_POOLS, \"$pool\""
        else
          STORAGE_POOLS="\"$pool\""
        fi
      done
      ok "已自动配置存储池: $DETECTED_POOLS"
    else
      STORAGE_POOLS="\"default\""
      warn "未检测到存储池，使用默认配置: default"
    fi
    ;;
  2)
    echo "请输入存储池名称，多个存储池用空格分隔（按优先级顺序）"
    echo "示例: default zfs-pool btrfs-pool"
    read -p "存储池列表: " MANUAL_POOLS
    if [[ -n "$MANUAL_POOLS" ]]; then
      STORAGE_POOLS=""
      for pool in $MANUAL_POOLS; do
        if [[ -n "$STORAGE_POOLS" ]]; then
          STORAGE_POOLS="$STORAGE_POOLS, \"$pool\""
        else
          STORAGE_POOLS="\"$pool\""
        fi
      done
      ok "已手动配置存储池: $MANUAL_POOLS"
    else
      STORAGE_POOLS="\"default\""
      warn "输入为空，使用默认配置: default"
    fi
    ;;
esac
echo

echo "==== 步骤 3/6: 任务队列配置 ===="
echo
info "使用 Database 任务队列（基于 SQLite）"
ok "任务队列配置完成"
echo

echo "==== 步骤 4/6: 流量监控性能配置 ===="
echo
echo "请选择流量监控性能方案："
echo "1. 高性能模式 (适用独立服务器 8核+)         - CPU占用 10-15%, 封禁响应 ~10秒"
echo "2. 标准模式 (适用独立服务器 4-8核)          - CPU占用 5-10%, 封禁响应 ~15秒"
echo "3. 轻量模式 (适用独立服务器 2-4核)          - CPU占用 2-5%, 封禁响应 ~30秒"
echo "4. 最小模式 (适用无独享内核或共享VPS)       - CPU占用 0.5-2%, 封禁响应 ~60秒"
echo "5. 自定义模式 (手动配置所有参数)"
echo
read -p "请选择 [1-5, 默认2]: " TRAFFIC_MODE
TRAFFIC_MODE=${TRAFFIC_MODE:-2}

while [[ ! $TRAFFIC_MODE =~ ^[1-5]$ ]]; do
  warn "无效选择，请输入 1-5"
  read -p "请选择 [1-5, 默认2]: " TRAFFIC_MODE
  TRAFFIC_MODE=${TRAFFIC_MODE:-2}
done

case $TRAFFIC_MODE in
  1)
    TRAFFIC_INTERVAL=5
    TRAFFIC_BATCH_SIZE=20
    TRAFFIC_LIMIT_CHECK_INTERVAL=10
    TRAFFIC_LIMIT_CHECK_BATCH_SIZE=20
    TRAFFIC_AUTO_RESET_INTERVAL=600
    TRAFFIC_AUTO_RESET_BATCH_SIZE=20
    ok "已选择: 高性能模式"
    ;;
  2)
    TRAFFIC_INTERVAL=10
    TRAFFIC_BATCH_SIZE=10
    TRAFFIC_LIMIT_CHECK_INTERVAL=15
    TRAFFIC_LIMIT_CHECK_BATCH_SIZE=10
    TRAFFIC_AUTO_RESET_INTERVAL=900
    TRAFFIC_AUTO_RESET_BATCH_SIZE=10
    ok "已选择: 标准模式"
    ;;
  3)
    TRAFFIC_INTERVAL=15
    TRAFFIC_BATCH_SIZE=5
    TRAFFIC_LIMIT_CHECK_INTERVAL=30
    TRAFFIC_LIMIT_CHECK_BATCH_SIZE=5
    TRAFFIC_AUTO_RESET_INTERVAL=1800
    TRAFFIC_AUTO_RESET_BATCH_SIZE=5
    ok "已选择: 轻量模式"
    ;;
  4)
    TRAFFIC_INTERVAL=30
    TRAFFIC_BATCH_SIZE=3
    TRAFFIC_LIMIT_CHECK_INTERVAL=60
    TRAFFIC_LIMIT_CHECK_BATCH_SIZE=3
    TRAFFIC_AUTO_RESET_INTERVAL=3600
    TRAFFIC_AUTO_RESET_BATCH_SIZE=3
    ok "已选择: 最小模式"
    ;;
  5)
    ok "已选择: 自定义模式"
    echo
    echo "==== 流量统计配置 ===="
    read -p "流量统计间隔（秒，建议5-30）[10]: " TRAFFIC_INTERVAL
    TRAFFIC_INTERVAL=${TRAFFIC_INTERVAL:-10}
    
    read -p "流量统计批量（建议3-20）[10]: " TRAFFIC_BATCH_SIZE
    TRAFFIC_BATCH_SIZE=${TRAFFIC_BATCH_SIZE:-10}
    
    echo
    echo "==== 流量限制检测配置 ===="
    read -p "限制检测间隔（秒，建议10-60）[15]: " TRAFFIC_LIMIT_CHECK_INTERVAL
    TRAFFIC_LIMIT_CHECK_INTERVAL=${TRAFFIC_LIMIT_CHECK_INTERVAL:-15}
    
    read -p "限制检测批量（建议3-20）[10]: " TRAFFIC_LIMIT_CHECK_BATCH_SIZE
    TRAFFIC_LIMIT_CHECK_BATCH_SIZE=${TRAFFIC_LIMIT_CHECK_BATCH_SIZE:-10}
    
    echo
    echo "==== 自动重置检查配置 ===="
    read -p "自动重置检查间隔（秒，建议600-3600）[900]: " TRAFFIC_AUTO_RESET_INTERVAL
    TRAFFIC_AUTO_RESET_INTERVAL=${TRAFFIC_AUTO_RESET_INTERVAL:-900}
    
    read -p "自动重置检查批量（建议3-20）[10]: " TRAFFIC_AUTO_RESET_BATCH_SIZE
    TRAFFIC_AUTO_RESET_BATCH_SIZE=${TRAFFIC_AUTO_RESET_BATCH_SIZE:-10}
    
    ok "自定义配置完成"
    ;;
esac

ok "流量监控性能配置完成"
echo

echo "==== 步骤 5/6: 网络管理方案 ===="
echo
echo "请选择网络模式："
echo "1. IPv4 NAT共享"
echo "2. IPv4 NAT + IPv6独立"
echo
read -p "请选择网络模式 [1-2]: " NETWORK_MODE

while [[ ! $NETWORK_MODE =~ ^[1-2]$ ]]; do
  warn "无效选择，请输入 1-2"
  read -p "请选择网络模式 [1-2]: " NETWORK_MODE
done

case $NETWORK_MODE in
  1)
    NAT_SUPPORT="true"
    IPV6_BINDING_ENABLED="false"
    ok "已选择: 模式1 - IPv4 NAT共享"
    ;;
  2)
    NAT_SUPPORT="true"
    IPV6_BINDING_ENABLED="true"
    ok "已选择: 模式2 - IPv4 NAT + IPv6独立"
    ;;
esac

echo
echo "==== 网络接口配置 ===="
read -p "外网网卡接口 [$DEFAULT_INTERFACE]: " NETWORK_INTERFACE
NETWORK_INTERFACE=${NETWORK_INTERFACE:-$DEFAULT_INTERFACE}

if [[ $NAT_SUPPORT == "true" ]]; then
  read -p "外网 IPv4 地址 [$DEFAULT_IPV4]: " NETWORK_IPV4
  NETWORK_IPV4=${NETWORK_IPV4:-$DEFAULT_IPV4}
else
  NETWORK_IPV4=""
fi

NETWORK_IPV6=""

if [[ $IPV6_BINDING_ENABLED == "true" ]]; then
  echo
  echo "==== IPv6 独立绑定配置 ===="
  read -p "IPv6 绑定网卡接口 [$DEFAULT_INTERFACE]: " IPV6_BINDING_INTERFACE
  IPV6_BINDING_INTERFACE=${IPV6_BINDING_INTERFACE:-$DEFAULT_INTERFACE}
  
  echo
  echo "配置 IPv6 地址池："
  read -p "IPv6 起始地址 (例如: 2001:db8::1): " IPV6_POOL_START
  IPV6_POOL_START=${IPV6_POOL_START:-"2001:db8::1"}
  
  read -p "IPv6 前缀长度 [64]: " IPV6_POOL_PREFIX
  IPV6_POOL_PREFIX=${IPV6_POOL_PREFIX:-64}
  
  read -p "IPv6 地址池大小 [1000]: " IPV6_POOL_SIZE
  IPV6_POOL_SIZE=${IPV6_POOL_SIZE:-1000}
else
  IPV6_BINDING_INTERFACE=""
  IPV6_POOL_START="2001:db8::1"
  IPV6_POOL_PREFIX=64
  IPV6_POOL_SIZE=1000
fi

ok "网络配置完成"
echo

echo "==== 步骤 6/6: Nginx 反向代理配置 ===="
echo
echo "是否启用 Nginx 反向代理功能？"
echo "此功能允许为容器配置域名反向代理（需要已安装 Nginx）"
echo
read -p "是否启用 Nginx 反向代理? (y/N): " ENABLE_NGINX_PROXY

if [[ $ENABLE_NGINX_PROXY == "y" || $ENABLE_NGINX_PROXY == "Y" ]]; then
  NGINX_PROXY_ENABLED="true"
  
  if ! command -v nginx &> /dev/null; then
    info "正在安装 Nginx..."
    apt update -y && apt install -y nginx || err "Nginx 安装失败"
    systemctl enable nginx
    systemctl start nginx
    ok "Nginx 安装完成"
  else
    ok "检测到 Nginx 已安装"
  fi
  
  if [[ -d "/etc/logrotate.d" ]]; then
    cat > /etc/logrotate.d/nginx-lxdapi <<'EOF'
/var/log/nginx/*-access.log /var/log/nginx/*-error.log {
    daily
    rotate 3
    missingok
    notifempty
    compress
    delaycompress
    sharedscripts
    postrotate
        [ -f /var/run/nginx.pid ] && kill -USR1 `cat /var/run/nginx.pid`
    endscript
}
EOF
    ok "Nginx 日志轮转配置已创建"
  fi
  
  ok "Nginx 反向代理功能已启用"
else
  NGINX_PROXY_ENABLED="false"
  ok "已禁用 Nginx 反向代理功能"
fi

echo
echo "========================================"
echo "      生成配置文件"
echo "========================================"
echo

info "正在生成配置文件..."

replace_config_var() {
  local placeholder="$1"
  local value="$2"
  escaped_value=$(printf '%s\n' "$value" | sed -e 's/[\/&]/\\&/g')
  sed -i "s/\${$placeholder}/$escaped_value/g" "$CFG"
}

CPU_CORES=$(nproc 2>/dev/null || echo "4")
if [[ $CPU_CORES -lt 2 ]]; then
  WORKER_COUNT=2
elif [[ $CPU_CORES -gt 16 ]]; then
  WORKER_COUNT=16
else
  WORKER_COUNT=$CPU_CORES
fi
info "检测到 CPU 核心数: $CPU_CORES，设置 Worker 数量为: $WORKER_COUNT"

replace_config_var "SERVER_PORT" "$SERVER_PORT"
replace_config_var "PUBLIC_NETWORK_IP_ADDRESS" "$EXTERNAL_IP"
replace_config_var "API_ACCESS_HASH" "$API_HASH"
replace_config_var "STORAGE_POOLS" "$STORAGE_POOLS"
replace_config_var "WORKER_COUNT" "$WORKER_COUNT"

replace_config_var "DB_TYPE" "$DB_TYPE"

replace_config_var "NAT_SUPPORT" "$NAT_SUPPORT"
replace_config_var "NETWORK_EXTERNAL_INTERFACE" "$NETWORK_INTERFACE"
replace_config_var "NETWORK_EXTERNAL_IPV4" "$NETWORK_IPV4"

replace_config_var "IPV6_BINDING_ENABLED" "$IPV6_BINDING_ENABLED"
replace_config_var "IPV6_BINDING_INTERFACE" "$IPV6_BINDING_INTERFACE"
replace_config_var "IPV6_POOL_START" "$IPV6_POOL_START"
replace_config_var "IPV6_POOL_PREFIX" "$IPV6_POOL_PREFIX"
replace_config_var "IPV6_POOL_SIZE" "$IPV6_POOL_SIZE"

replace_config_var "TRAFFIC_INTERVAL" "$TRAFFIC_INTERVAL"
replace_config_var "TRAFFIC_BATCH_SIZE" "$TRAFFIC_BATCH_SIZE"
replace_config_var "TRAFFIC_LIMIT_CHECK_INTERVAL" "$TRAFFIC_LIMIT_CHECK_INTERVAL"
replace_config_var "TRAFFIC_LIMIT_CHECK_BATCH_SIZE" "$TRAFFIC_LIMIT_CHECK_BATCH_SIZE"
replace_config_var "TRAFFIC_AUTO_RESET_INTERVAL" "$TRAFFIC_AUTO_RESET_INTERVAL"
replace_config_var "TRAFFIC_AUTO_RESET_BATCH_SIZE" "$TRAFFIC_AUTO_RESET_BATCH_SIZE"

replace_config_var "NGINX_PROXY_ENABLED" "$NGINX_PROXY_ENABLED"

ok "配置文件已生成"

echo
echo "========================================"
echo "      创建服务并启动"
echo "========================================"
echo

info "创建系统服务..."

cat > "$SERVICE" <<EOF
[Unit]
Description=lxdapi-xkatld
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

info "启动服务..."
systemctl daemon-reload
systemctl enable --now $NAME

ok "系统服务已创建并启动"

echo

echo "========================================"
echo "          安装/升级完成"
echo "========================================"
echo
echo "服务信息:"
echo "  数据目录: $DIR"
echo "  外网 IP: $EXTERNAL_IP"
echo "  API 端口: $SERVER_PORT"
echo "  API Hash: $API_HASH"
echo
echo "数据库配置:"
echo "  数据库: SQLite (lxdapi.db)"
echo "  任务队列: database"
echo
echo "存储池配置: [$STORAGE_POOLS]"
echo
echo "网络模式:"
case $NETWORK_MODE in
  1) echo "  IPv4 NAT";;
  2) echo "  IPv4 + IPv6 NAT";;
  3) echo "  全功能模式 (IPv4 NAT + IPv6 NAT + IPv6 独立绑定)";;
  4) echo "  混合模式 (IPv4 NAT + IPv6 独立绑定)";;
  5) echo "  纯 IPv6 公网 模式";;
esac
echo
echo "流量监控性能:"
case $TRAFFIC_MODE in
  1) echo "  模式: 高性能模式 (统计间隔: ${TRAFFIC_INTERVAL}秒, 检测间隔: ${TRAFFIC_LIMIT_CHECK_INTERVAL}秒)";;
  2) echo "  模式: 标准模式 (统计间隔: ${TRAFFIC_INTERVAL}秒, 检测间隔: ${TRAFFIC_LIMIT_CHECK_INTERVAL}秒)";;
  3) echo "  模式: 轻量模式 (统计间隔: ${TRAFFIC_INTERVAL}秒, 检测间隔: ${TRAFFIC_LIMIT_CHECK_INTERVAL}秒)";;
  4) echo "  模式: 最小模式 (统计间隔: ${TRAFFIC_INTERVAL}秒, 检测间隔: ${TRAFFIC_LIMIT_CHECK_INTERVAL}秒)";;
  5) echo "  模式: 自定义模式 (统计间隔: ${TRAFFIC_INTERVAL}秒, 检测间隔: ${TRAFFIC_LIMIT_CHECK_INTERVAL}秒)";;
esac
echo
echo "反向代理:"
if [[ $NGINX_PROXY_ENABLED == "true" ]]; then
  echo "  状态: 已启用 (Nginx 已安装并启动)"
else
  echo "  状态: 未启用"
fi
echo

info "等待服务稳定..."
sleep 3

echo
echo "========================================"
echo "服务状态:"
echo "========================================"
systemctl status $NAME --no-pager
