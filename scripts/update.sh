#!/bin/bash
set -euo pipefail

# =============================================================================
# 配置
# =============================================================================
readonly APP_NAME="octopus"
readonly REPO="gclm/octopus"
readonly INSTALL_DIR="/opt/octopus"
readonly SERVICE_USER="octopus"
readonly BINARY="${INSTALL_DIR}/${APP_NAME}"

# 颜色输出（全部走 stderr）
info()  { echo -e "\033[36m[INFO]\033[0m  $*" >&2; }
ok()    { echo -e "\033[32m[OK]\033[0m    $*" >&2; }
warn()  { echo -e "\033[33m[WARN]\033[0m  $*" >&2; }
die()   { echo -e "\033[31m[ERROR]\033[0m $*" >&2; exit 1; }

usage() {
    cat <<EOF
用法: sudo bash $0 [选项]

选项:
  -y, --yes             跳过确认直接更新
  -h, --help            显示帮助信息
EOF
    exit 0
}

SKIP_CONFIRM=false
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -y|--yes) SKIP_CONFIRM=true; shift ;;
            -h|--help) usage ;;
            *) die "未知选项: $1" ;;
        esac
    done
}

# =============================================================================
# 前置检查
# =============================================================================
check_root() {
    [[ $EUID -eq 0 ]] || die "请使用 root 用户或 sudo 运行此脚本"
}

check_installation() {
    [[ -x "$BINARY" ]] || die "未找到已安装的 ${APP_NAME}，请先运行 install.sh"
}

# =============================================================================
# 版本对比
# =============================================================================
get_local_version() {
    "$BINARY" version 2>/dev/null | grep "^Version:" | awk '{print $2}'
}

get_latest_version() {
    info "获取最新版本信息..."
    local version
    version=$(curl -fsSL --connect-timeout 10 --max-time 30 \
        "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
    [[ -n "$version" ]] || die "无法获取最新版本号"
    echo "$version"
}

# =============================================================================
# 下载与替换
# =============================================================================
download_binary() {
    local version="$1"
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)   arch="x86_64" ;;
        aarch64)  arch="arm64" ;;
        armv7l)   arch="armv7" ;;
        i386|i686) arch="x86" ;;
        *)        die "不支持的架构: $arch" ;;
    esac

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

    info "解压..."
    unzip -o -q "${tmp_dir}/${filename}" -d "$tmp_dir"
    cp "${tmp_dir}/${APP_NAME}" "${BINARY}.new"
    chmod +x "${BINARY}.new"
    rm -rf "$tmp_dir"
}

do_update() {
    info "停止服务..."
    systemctl stop "$APP_NAME" || true

    # 备份当前二进制
    cp "$BINARY" "${BINARY}.bak"

    # 替换
    mv "${BINARY}.new" "$BINARY"
    ok "二进制文件已替换"

    info "启动服务..."
    systemctl start "$APP_NAME"
    ok "服务已启动"

    # 验证新版本
    local new_version
    new_version=$(get_local_version)
    ok "当前版本: ${new_version}"

    # 替换成功后清理备份
    rm -f "${BINARY}.bak"
}

# =============================================================================
# 主流程
# =============================================================================
main() {
    parse_args "$@"
    check_root
    check_installation

    local local_version latest_version
    local_version=$(get_local_version)
    latest_version=$(get_latest_version)

    echo ""
    echo "========================================="
    echo "  Octopus 更新"
    echo "  当前版本: ${local_version}"
    echo "  最新版本: ${latest_version}"
    echo "========================================="
    echo ""

    if [[ "$local_version" == "$latest_version" ]]; then
        ok "已是最新版本，无需更新"
        exit 0
    fi

    if [[ "$SKIP_CONFIRM" == false ]]; then
        read -rp "确认更新? [y/N] " confirm
        [[ "$confirm" =~ ^[yY]$ ]] || { info "已取消"; exit 0; }
    fi

    download_binary "$latest_version"
    do_update

    echo ""
    ok "更新完成! ${local_version} → ${latest_version}"
}

main "$@"
