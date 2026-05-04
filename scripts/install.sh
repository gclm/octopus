#!/bin/bash
set -euo pipefail

# =============================================================================
# 配置
# =============================================================================
readonly APP_NAME="octopus"
readonly REPO="gclm/octopus"
readonly INSTALL_DIR="/opt/octopus"
readonly SERVICE_USER="octopus"
readonly SERVICE_FILE="/etc/systemd/system/octopus.service"

# 可自定义参数（通过命令行选项覆盖）
PORT=8080

# 颜色输出（全部走 stderr，避免污染命令替换）
info()  { echo -e "\033[36m[INFO]\033[0m  $*" >&2; }
ok()    { echo -e "\033[32m[OK]\033[0m    $*" >&2; }
warn()  { echo -e "\033[33m[WARN]\033[0m  $*" >&2; }
die()   { echo -e "\033[31m[ERROR]\033[0m $*" >&2; exit 1; }

usage() {
    cat <<EOF
用法: sudo bash $0 [选项]

选项:
  -p, --port PORT       监听端口 (默认: 8080)
  -h, --help            显示帮助信息

示例:
  sudo bash $0
  sudo bash $0 --port 3000
EOF
    exit 0
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -p|--port)    PORT="$2";       shift 2 ;;
            -h|--help)    usage ;;
            *) die "未知选项: $1\n$(usage)" ;;
        esac
    done

    if ! [[ "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
        die "无效端口号: ${PORT}，范围应为 1-65535"
    fi
}

# =============================================================================
# 检查 root 权限
# =============================================================================
check_root() {
    if [[ $EUID -ne 0 ]]; then
        die "请使用 root 用户或 sudo 运行此脚本"
    fi
}

# =============================================================================
# 检测系统架构
# =============================================================================
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)  echo "x86_64" ;;
        aarch64) echo "arm64" ;;
        armv7l)  echo "armv7" ;;
        i386|i686) echo "x86" ;;
        *)       die "不支持的架构: $arch" ;;
    esac
}

# =============================================================================
# 获取最新 Release 版本号
# =============================================================================
get_latest_version() {
    info "获取最新版本信息..."
    local version
    version=$(curl -fsSL --connect-timeout 10 --max-time 30 \
        "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
    if [[ -z "$version" ]]; then
        die "无法获取最新版本号"
    fi
    echo "$version"
}

# =============================================================================
# 下载并安装二进制文件
# =============================================================================
download_binary() {
    local version="$1"
    local arch="$2"
    local filename="${APP_NAME}-linux-${arch}.zip"
    local mirror_url="https://ghfast.top/https://github.com/${REPO}/releases/download/${version}/${filename}"
    local github_url="https://github.com/${REPO}/releases/download/${version}/${filename}"
    local tmp_dir
    tmp_dir=$(mktemp -d)

    info "下载 ${filename}..."
    if curl -fSL --progress-bar --connect-timeout 15 --max-time 600 \
        -o "${tmp_dir}/${filename}" "$mirror_url"; then
        ok "镜像下载成功"
    elif curl -fSL --progress-bar --connect-timeout 15 --max-time 600 \
        -o "${tmp_dir}/${filename}" "$github_url"; then
        ok "直连下载成功"
    else
        rm -rf "$tmp_dir"
        die "下载失败 (镜像与直连均不可用)"
    fi

    info "解压 ${filename}..."
    if ! unzip -o -q "${tmp_dir}/${filename}" -d "$tmp_dir"; then
        rm -rf "$tmp_dir"
        die "解压失败"
    fi

    info "安装到 ${INSTALL_DIR}/${APP_NAME}..."
    mkdir -p "$INSTALL_DIR"
    cp "${tmp_dir}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
    chmod +x "${INSTALL_DIR}/${APP_NAME}"
    rm -rf "$tmp_dir"
    ok "二进制文件安装完成"
}

# =============================================================================
# 创建服务用户
# =============================================================================
create_user() {
    if id "$SERVICE_USER" &>/dev/null; then
        ok "用户 ${SERVICE_USER} 已存在，跳过创建"
        return
    fi
    info "创建服务用户 ${SERVICE_USER}..."
    useradd -r -s /usr/sbin/nologin "$SERVICE_USER"
    ok "用户创建完成"
}

# =============================================================================
# 生成配置文件
# =============================================================================
generate_config() {
    local config_file="${INSTALL_DIR}/data/config.json"

    if [[ -f "$config_file" ]]; then
        warn "配置文件已存在: ${config_file}，跳过生成"
        return
    fi

    info "生成配置文件: ${config_file}"
    cat > "$config_file" <<EOF
{
  "server": {
    "host": "0.0.0.0",
    "port": ${PORT}
  },
  "database": {
    "type": "sqlite",
    "path": "data/data.db"
  },
  "log": {
    "level": "info"
  }
}
EOF
    ok "配置文件已生成 (端口: ${PORT})"
}

# =============================================================================
# 初始化数据目录
# =============================================================================
setup_data_dir() {
    mkdir -p "${INSTALL_DIR}/data"
    chown -R "${SERVICE_USER}:${SERVICE_USER}" "$INSTALL_DIR"
    ok "数据目录就绪: ${INSTALL_DIR}/data"
}

# =============================================================================
# 安装 systemd 服务
# =============================================================================
install_service() {
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Octopus - LLM API Aggregation Service
After=network.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${APP_NAME} start
Restart=on-failure
RestartSec=5

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${INSTALL_DIR}/data

StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

[Install]
WantedBy=multi-user.target
EOF
    ok "systemd 服务文件已写入: ${SERVICE_FILE}"
}

# =============================================================================
# 启动服务
# =============================================================================
start_service() {
    systemctl daemon-reload
    systemctl enable --now "$APP_NAME"
    ok "服务已启动并设为开机自启"
    systemctl status "$APP_NAME" --no-pager -l || true
}

# =============================================================================
# 主流程
# =============================================================================
main() {
    parse_args "$@"
    check_root

    local arch
    arch=$(detect_arch)
    local version
    version=$(get_latest_version)

    echo ""
    echo "========================================="
    echo "  Octopus 安装脚本"
    echo "  版本: ${version}"
    echo "  架构: linux/${arch}"
    echo "  安装目录: ${INSTALL_DIR}"
    echo "========================================="
    echo ""

    download_binary "$version" "$arch"
    create_user
    setup_data_dir
    generate_config
    install_service
    start_service

    echo ""
    echo "========================================="
    ok "安装完成!"
    echo "  访问: http://<服务器IP>:${PORT}"
    echo "  日志: journalctl -u ${APP_NAME} -f"
    echo "  配置: ${INSTALL_DIR}/data/config.json"
    echo "========================================="
}

main "$@"
