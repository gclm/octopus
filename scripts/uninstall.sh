#!/bin/bash
set -euo pipefail

# =============================================================================
# 配置
# =============================================================================
readonly APP_NAME="octopus"
readonly INSTALL_DIR="/opt/octopus"
readonly SERVICE_USER="octopus"
readonly SERVICE_FILE="/etc/systemd/system/octopus.service"

# 颜色输出
info()  { echo -e "\033[36m[INFO]\033[0m  $*" >&2; }
ok()    { echo -e "\033[32m[OK]\033[0m    $*" >&2; }
warn()  { echo -e "\033[33m[WARN]\033[0m  $*" >&2; }
die()   { echo -e "\033[31m[ERROR]\033[0m $*" >&2; exit 1; }

usage() {
    cat <<EOF
用法: sudo bash $0 [选项]

选项:
  --purge               同时删除数据目录 (${INSTALL_DIR}/data)
  --keep-data           仅删除二进制，保留数据目录（默认）
  -y, --yes             跳过确认直接卸载（默认保留数据）
  -h, --help            显示帮助信息
EOF
    exit 0
}

PURGE=false
SKIP_CONFIRM=false
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --purge)    PURGE=true;         shift ;;
            --keep-data) PURGE=false;       shift ;;
            -y|--yes)   SKIP_CONFIRM=true;  shift ;;
            -h|--help)  usage ;;
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

# =============================================================================
# 卸载
# =============================================================================
stop_service() {
    if systemctl is-active "$APP_NAME" &>/dev/null; then
        info "停止服务..."
        systemctl stop "$APP_NAME"
        ok "服务已停止"
    fi
}

remove_service() {
    if [[ -f "$SERVICE_FILE" ]]; then
        info "移除 systemd 服务文件..."
        systemctl disable "$APP_NAME" 2>/dev/null || true
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
        ok "服务文件已移除"
    fi
}

remove_user() {
    if id "$SERVICE_USER" &>/dev/null; then
        info "删除用户 ${SERVICE_USER}..."
        userdel "$SERVICE_USER"
        ok "用户已删除"
    fi
}

remove_files() {
    if [[ -d "$INSTALL_DIR" ]]; then
        if [[ "$PURGE" == true ]]; then
            info "删除安装目录及所有数据..."
            rm -rf "$INSTALL_DIR"
            ok "已删除: ${INSTALL_DIR}"
        else
            info "删除二进制文件（保留数据目录）..."
            rm -f "${INSTALL_DIR}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}.bak"
            ok "已删除二进制文件，数据目录保留: ${INSTALL_DIR}/data"
        fi
    fi
}

# =============================================================================
# 主流程
# =============================================================================
main() {
    parse_args "$@"
    check_root

    echo ""
    echo "========================================="
    echo "  Octopus 卸载"
    echo "========================================="
    echo ""

    # 交互式选择模式
    if [[ "$SKIP_CONFIRM" == false ]]; then
        echo "请选择卸载模式:"
        echo "  1) 保留数据 — 仅删除二进制和服务"
        echo "  2) 完全卸载 — 删除二进制、服务和所有数据"
        echo "  0) 取消"
        echo ""
        read -rp "请输入 [0-2]: " choice
        case "$choice" in
            1) PURGE=false ;;
            2) PURGE=true ;;
            *) info "已取消"; exit 0 ;;
        esac
        echo ""
        if [[ "$PURGE" == true ]]; then
            warn "即将删除所有数据，此操作不可恢复!"
        fi
        read -rp "确认执行? [y/N] " confirm
        [[ "$confirm" =~ ^[yY]$ ]] || { info "已取消"; exit 0; }
    fi

    stop_service
    remove_service
    remove_files
    remove_user

    echo ""
    ok "卸载完成"
}

main "$@"
