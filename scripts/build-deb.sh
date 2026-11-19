#!/usr/bin/env bash
set -euo pipefail

APP_NAME="mikromon"
ARCH="${1:-amd64}"
RAW_VER="${2:-1.0.0}"
VERSION="${RAW_VER#v}"
BUILD_DIR="dist/${APP_NAME}_${VERSION}_${ARCH}"

mkdir -p "${BUILD_DIR}/DEBIAN" \
         "${BUILD_DIR}/usr/local/bin" \
         "${BUILD_DIR}/lib/systemd/system" \
         "${BUILD_DIR}/etc/${APP_NAME}" \
         "${BUILD_DIR}/var/lib/${APP_NAME}"

echo "==> Building static binary for linux/${ARCH} (v${VERSION})..."
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${BUILD_DIR}/usr/local/bin/${APP_NAME}" .
chmod 755 "${BUILD_DIR}/usr/local/bin/${APP_NAME}"

cat > "${BUILD_DIR}/DEBIAN/control" <<EOF
Package: ${APP_NAME}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: MikroMon Maintainer <admin@example.com>
Description: MikroTik RouterOS (8728/8729) traffic & behavior monitor
 Monitors interfaces, 1h-30d traffic history, live connections,
 DNS-based application classification, per-IP bandwidth, and Torch.
EOF

cat > "${BUILD_DIR}/etc/${APP_NAME}/mikromon.env" <<EOF
MIKROMON_LISTEN=:8080
MIKROMON_DATA=/var/lib/mikromon
EOF

cat > "${BUILD_DIR}/DEBIAN/conffiles" <<EOF
/etc/mikromon/mikromon.env
EOF

cat > "${BUILD_DIR}/lib/systemd/system/${APP_NAME}.service" <<EOF
[Unit]
Description=MikroMon MikroTik Monitor Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/mikromon/mikromon.env
ExecStart=/usr/local/bin/mikromon
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

cat > "${BUILD_DIR}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
mkdir -p /var/lib/mikromon
systemctl daemon-reload || true
systemctl enable mikromon.service || true
systemctl restart mikromon.service || true
EOF
chmod 755 "${BUILD_DIR}/DEBIAN/postinst"

cat > "${BUILD_DIR}/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "remove" ]; then
  systemctl stop mikromon.service || true
  systemctl disable mikromon.service || true
fi
EOF
chmod 755 "${BUILD_DIR}/DEBIAN/prerm"

dpkg-deb --build --root-owner-group "${BUILD_DIR}"
echo "==> Done! Package generated: ${BUILD_DIR}.deb"
