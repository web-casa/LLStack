#!/bin/bash
set -euo pipefail

# LLStack SELinux Policy Setup
# Run as root after install to configure SELinux contexts and policies.
# Usage: selinux-setup.sh

LLSTACK_DIR="/opt/llstack"
PANEL_PORT=30333
API_PORT=8001

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[SELinux]${NC} $*"; }
err() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Check if SELinux is enabled
if ! command -v getenforce &>/dev/null || [[ "$(getenforce)" == "Disabled" ]]; then
    log "SELinux is disabled or not installed. No action needed."
    exit 0
fi

log "SELinux mode: $(getenforce)"

# 1. Install required tools
log "Installing SELinux tools..."
dnf install -y policycoreutils-python-utils selinux-policy-devel 2>&1 | tail -1

# 2. Set file contexts for panel directories
log "Setting file contexts..."

# Panel backend — httpd_sys_script_exec_t for scripts
semanage fcontext -a -t httpd_sys_content_t "$LLSTACK_DIR/web/dist(/.*)?" 2>/dev/null || \
    semanage fcontext -m -t httpd_sys_content_t "$LLSTACK_DIR/web/dist(/.*)?" 2>/dev/null || true

# Panel data — needs read/write
semanage fcontext -a -t httpd_sys_rw_content_t "$LLSTACK_DIR/data(/.*)?" 2>/dev/null || \
    semanage fcontext -m -t httpd_sys_rw_content_t "$LLSTACK_DIR/data(/.*)?" 2>/dev/null || true

# Panel logs
semanage fcontext -a -t httpd_log_t "$LLSTACK_DIR/logs(/.*)?" 2>/dev/null || \
    semanage fcontext -m -t httpd_log_t "$LLSTACK_DIR/logs(/.*)?" 2>/dev/null || true

# Scripts — executable
semanage fcontext -a -t bin_t "$LLSTACK_DIR/scripts(/.*)?" 2>/dev/null || \
    semanage fcontext -m -t bin_t "$LLSTACK_DIR/scripts(/.*)?" 2>/dev/null || true

# Apply contexts
restorecon -Rv "$LLSTACK_DIR" 2>&1 | head -5

# 3. Port labels
log "Configuring port labels..."

# Panel HTTPS port (30333)
semanage port -a -t http_port_t -p tcp $PANEL_PORT 2>/dev/null || \
    semanage port -m -t http_port_t -p tcp $PANEL_PORT 2>/dev/null || true

# Internal API port (8001) — allow httpd to bind
semanage port -a -t http_port_t -p tcp $API_PORT 2>/dev/null || \
    semanage port -m -t http_port_t -p tcp $API_PORT 2>/dev/null || true

# 4. SELinux booleans
log "Setting SELinux booleans..."

# Set booleans one by one (setsebool -P is slow, each takes ~5s)
for BOOL in httpd_can_network_connect httpd_can_network_connect_db \
            httpd_execmem httpd_can_sendmail httpd_read_user_content \
            httpd_enable_homedirs; do
    log "  Setting $BOOL = on ..."
    setsebool -P "$BOOL" on 2>/dev/null || true
done

# 5. Custom policy module (if compile tools available)
log "Building custom policy module..."
POLICY_DIR="$LLSTACK_DIR/config/selinux"
if [[ -f "$POLICY_DIR/llstack.te" ]] && command -v checkmodule &>/dev/null; then
    cd "$POLICY_DIR"
    checkmodule -M -m -o llstack.mod llstack.te 2>&1 && \
    semodule_package -o llstack.pp -m llstack.mod 2>&1 && \
    semodule -i llstack.pp 2>&1 && \
    log "Custom policy module installed" || \
    log "Custom policy module failed (non-critical, booleans are set)"
    rm -f llstack.mod llstack.pp 2>/dev/null
else
    log "Policy compile tools not available, using booleans only"
fi

# 6. Fix LiteHttpd contexts
log "Setting LiteHttpd contexts..."
semanage fcontext -a -t httpd_exec_t "/usr/local/lsws/bin/(.*)" 2>/dev/null || true
semanage fcontext -a -t httpd_config_t "/usr/local/lsws/conf(/.*)?" 2>/dev/null || true
semanage fcontext -a -t httpd_log_t "/usr/local/lsws/logs(/.*)?" 2>/dev/null || true
semanage fcontext -a -t httpd_sys_content_t "/usr/local/lsws/html(/.*)?" 2>/dev/null || true
restorecon -Rv /usr/local/lsws/ 2>&1 | head -5

# 7. Verify
log "Verification:"
echo "  SELinux mode: $(getenforce)"
echo "  httpd_can_network_connect: $(getsebool httpd_can_network_connect 2>/dev/null | awk '{print $NF}')"
echo "  httpd_can_network_connect_db: $(getsebool httpd_can_network_connect_db 2>/dev/null | awk '{print $NF}')"
echo "  Port 8001: $(semanage port -l 2>/dev/null | grep ":.*8001" | head -1)"
echo "  Port 30333: $(semanage port -l 2>/dev/null | grep ":.*30333" | head -1)"
echo ""
log "SELinux setup complete. Restart services: systemctl restart llstack lshttpd"
