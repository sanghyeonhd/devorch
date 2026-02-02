#!/bin/bash
# DevOrch 크로스 플랫폼 빌드 스크립트

VERSION="v0.1.0"
BUILD_DIR="build"
BIN_NAME="devorch"

echo "🚀 Building DevOrch $VERSION for multiple platforms..."
echo ""

# 빌드 디렉토리 생성
mkdir -p "$BUILD_DIR"

# 빌드할 플랫폼 목록
PLATFORMS=(
    "darwin/amd64"   # macOS Intel
    "darwin/arm64"   # macOS Apple Silicon
    "linux/amd64"    # Linux x64
    "linux/arm64"    # Linux ARM64
    "windows/amd64"  # Windows x64
    "windows/arm64"  # Windows ARM64
)

# 각 플랫폼별로 빌드
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    
    OUTPUT_NAME="${BIN_NAME}-${GOOS}-${GOARCH}"
    
    # Windows의 경우 .exe 확장자 추가
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    OUTPUT_PATH="$BUILD_DIR/$OUTPUT_NAME"
    
    echo "📦 Building for $GOOS/$GOARCH..."
    
    # 빌드 실행
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$OUTPUT_PATH" \
        ./cmd/devorch
    
    if [ $? -eq 0 ]; then
        # 파일 크기 확인
        SIZE=$(du -h "$OUTPUT_PATH" | cut -f1)
        echo "   ✓ Built: $OUTPUT_PATH ($SIZE)"
    else
        echo "   ✗ Build failed for $GOOS/$GOARCH"
    fi
    
    echo ""
done

echo "✨ Build complete! Binaries are in the '$BUILD_DIR' directory."
echo ""
echo "📋 Summary:"
ls -lh "$BUILD_DIR"
