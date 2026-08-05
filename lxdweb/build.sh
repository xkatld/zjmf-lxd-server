#!/bin/bash

set -e

echo "开始交叉编译 LXD Web..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "脚本目录: $SCRIPT_DIR"

cd "$SCRIPT_DIR"

if [ ! -f "main.go" ]; then
    echo "错误: 请在 lxdweb 项目根目录执行此脚本"
    exit 1
fi

GO_BIN=$(command -v go 2>/dev/null)
if [ -z "$GO_BIN" ]; then
    if [ -x "/usr/local/go/bin/go" ]; then
        export PATH="/usr/local/go/bin:$PATH"
        GO_BIN="/usr/local/go/bin/go"
    elif [ -x "$HOME/go/bin/go" ]; then
        export PATH="$HOME/go/bin:$PATH"
        GO_BIN="$HOME/go/bin/go"
    fi
fi

if [ -z "$GO_BIN" ]; then
    echo "错误: 未找到 Go 编译器"
    exit 1
fi

echo "Go 版本: $(go version)"

TARGET_ARCHITECTURES=("amd64" "arm64")
CURRENT_ARCH=$(uname -m)
echo "当前系统架构: $CURRENT_ARCH"
echo "目标架构: ${TARGET_ARCHITECTURES[*]}"

echo "清理旧文件..."
rm -f lxdweb-*

echo "更新 Swagger 文档..."
SWAG_BIN=""
if command -v swag &> /dev/null; then
    SWAG_BIN="$(command -v swag)"
    echo "[SWAG] swag 工具已安装: $SWAG_BIN"
else
    GOPATH_BIN=$(go env GOPATH)/bin
    if [ -x "$GOPATH_BIN/swag" ]; then
        SWAG_BIN="$GOPATH_BIN/swag"
        echo "[SWAG] 在 GOPATH 中找到 swag: $SWAG_BIN"
    else
        echo "[SWAG] swag 工具未安装，正在自动安装..."
        if go install github.com/swaggo/swag/cmd/swag@latest; then
            if [ -x "$GOPATH_BIN/swag" ]; then
                SWAG_BIN="$GOPATH_BIN/swag"
                echo "[SWAG] swag 工具安装成功: $SWAG_BIN"
            else
                SWAG_BIN="$(command -v swag 2>/dev/null)"
                if [ -z "$SWAG_BIN" ]; then
                    echo "[WARN] 找不到 swag 可执行文件，跳过文档生成"
                else
                    echo "[SWAG] swag 工具安装成功: $SWAG_BIN"
                fi
            fi
        else
            echo "[WARN] swag 安装失败，跳过文档生成"
        fi
    fi
fi

if [ -n "$SWAG_BIN" ]; then
    echo "[SWAG] 生成 Swagger 文档..."
    SWAG_PATH="${SWAG_BIN}"
    GO_BIN_DIR=$(dirname "$(command -v go 2>/dev/null)")
    if [ -z "$GO_BIN_DIR" ]; then
        GO_BIN_DIR="/usr/local/go/bin"
    fi
    SWAG_CMD="PATH=${GO_BIN_DIR}:$PATH ${SWAG_PATH} init --parseInternal -g main.go --output docs --dir ./handlers,./models,./services 2>/dev/null || ${SWAG_PATH} init --parseInternal -g main.go --output docs"
    if eval "$SWAG_CMD"; then
        echo "[SWAG] Swagger 文档生成成功"
    else
        echo "[WARN] Swagger 文档生成失败，继续编译..."
    fi
else
    echo "[WARN] swag 工具不可用，跳过文档生成"
fi

echo "下载依赖..."
go mod download
go mod tidy

echo ""
echo "开始交叉编译..."
for arch in "${TARGET_ARCHITECTURES[@]}"; do
    echo "正在编译 linux/$arch 架构..."
    
    GOOS=linux \
    GOARCH=$arch \
    CGO_ENABLED=0 \
    go build \
        -trimpath \
        -ldflags="-s -w" \
        -tags="modernc" \
        -o "lxdweb-$arch" \
        .
    
    if [ -f "lxdweb-$arch" ]; then
        chmod +x "lxdweb-$arch"
        echo "[BUILD] lxdweb-$arch 编译成功"
    else
        echo "[ERROR] lxdweb-$arch 编译失败"
        exit 1
    fi
done

echo ""
echo "检查模板文件..."
if [ -d "templates" ]; then
    echo "[CHECK] 模板文件目录存在"
else
    echo "[WARN] templates目录不存在"
fi

echo ""
echo "====== 编译完成 ======"
echo "生成的二进制文件:"
for arch in "${TARGET_ARCHITECTURES[@]}"; do
    if [ -f "lxdweb-$arch" ]; then
        file_info=$(ls -lh "lxdweb-$arch")
        file_size=$(echo "$file_info" | awk '{print $5}')
        echo "  lxdweb-$arch: $file_size"
        
        if command -v file &> /dev/null; then
            file_type=$(file "lxdweb-$arch")
            echo "    架构验证: $file_type"
        fi
    fi
done

echo ""
echo "二进制文件:"
echo "  AMD64 系统: ./lxdweb-amd64"
echo "  ARM64 系统: ./lxdweb-arm64"
echo ""
echo "交叉编译成功完成！"
