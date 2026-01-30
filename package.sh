#!/bin/bash
# DevOrch 배포 패키지 생성 스크립트

VERSION="v0.1.0"
BUILD_DIR="build"
RELEASE_DIR="release"

echo "📦 Creating release packages for DevOrch $VERSION..."
echo ""

# Release 디렉토리 생성
mkdir -p "$RELEASE_DIR"

# 플랫폼별 패키지 생성
PLATFORMS=(
    "darwin-amd64:macOS-Intel"
    "darwin-arm64:macOS-AppleSilicon"
    "linux-amd64:Linux-x64"
    "linux-arm64:Linux-ARM64"
    "windows-amd64:Windows-x64"
    "windows-arm64:Windows-ARM64"
)

for PLATFORM_INFO in "${PLATFORMS[@]}"; do
    PLATFORM=${PLATFORM_INFO%:*}
    FRIENDLY_NAME=${PLATFORM_INFO#*:}
    
    BINARY="devorch-$PLATFORM"
    if [[ $PLATFORM == windows* ]]; then
        BINARY="${BINARY}.exe"
    fi
    
    if [ ! -f "$BUILD_DIR/$BINARY" ]; then
        echo "⚠️  Skipping $FRIENDLY_NAME (binary not found)"
        continue
    fi
    
    PACKAGE_NAME="devorch-${VERSION}-${FRIENDLY_NAME}"
    PACKAGE_DIR="$RELEASE_DIR/$PACKAGE_NAME"
    
    echo "📦 Creating package: $PACKAGE_NAME"
    
    # 패키지 디렉토리 생성
    mkdir -p "$PACKAGE_DIR"
    
    # 바이너리 복사
    cp "$BUILD_DIR/$BINARY" "$PACKAGE_DIR/"
    
    # README 복사
    cp "$BUILD_DIR/README.md" "$PACKAGE_DIR/"
    
    # 라이선스 복사 (있다면)
    if [ -f "LICENSE" ]; then
        cp "LICENSE" "$PACKAGE_DIR/"
    fi
    
    # 압축 (플랫폼별)
    cd "$RELEASE_DIR"
    if [[ $PLATFORM == windows* ]]; then
        # Windows는 zip
        zip -r -q "${PACKAGE_NAME}.zip" "$PACKAGE_NAME"
        echo "   ✓ Created: ${PACKAGE_NAME}.zip"
    else
        # Unix 계열은 tar.gz
        tar -czf "${PACKAGE_NAME}.tar.gz" "$PACKAGE_NAME"
        echo "   ✓ Created: ${PACKAGE_NAME}.tar.gz"
    fi
    cd - > /dev/null
    
    # 임시 디렉토리 삭제
    rm -rf "$PACKAGE_DIR"
    
    echo ""
done

echo "✨ Release packages created in '$RELEASE_DIR' directory!"
echo ""
echo "📋 Summary:"
ls -lh "$RELEASE_DIR"
echo ""
echo "🚀 Ready to release!"
