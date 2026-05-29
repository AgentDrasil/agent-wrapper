# Update git submodules recursively
submodule-update:
    git submodule update --init --recursive
    rm -rf out/ghostty

# Build libghostty-vt using Docker
build-libghostty:
    @if [ ! -f "third_party/ghostty/build.zig" ]; then \
        @echo "Error: third_party/ghostty/build.zig not found." >&2; \
        @echo "Please run 'just submodule-update' first." >&2; \
        @exit 1; \
    fi
    @echo "Building libghostty-vt via Docker Buildx..."
    docker buildx build \
        --output=out/ghostty \
        -f Dockerfile.libghostty \
        .
    sed -i 's|^prefix=/ghostty-dist|prefix='"$PWD"'/out/ghostty|g' out/ghostty/share/pkgconfig/*.pc
    @echo "Successfully built and exported libghostty-vt to out/ghostty/"

# Build Go project using CGO and Zig cross-compilation
build:
    @sed -i 's|^prefix=/ghostty-dist|prefix='"$PWD"'/out/ghostty|g' out/ghostty/share/pkgconfig/*.pc 2>/dev/null || true
    CGO_ENABLED=1 \
    GOOS=linux GOARCH=amd64 \
    CC="zig cc -target x86_64-linux-gnu" \
    CXX="zig c++ -target x86_64-linux-gnu" \
    CGO_CFLAGS="-I$PWD/out/ghostty/include -DGHOSTTY_STATIC" \
    CGO_LDFLAGS="-L$PWD/out/ghostty/lib -lghostty-vt" \
    PKG_CONFIG_PATH="$PWD/out/ghostty/share/pkgconfig" \
    go build ./...

# Clean all build artifacts, exported libraries, and Go build cache
clean:
    @echo "Cleaning build artifacts..."
    rm -rf out agent-wrapper .zig-cache
    go clean -cache
