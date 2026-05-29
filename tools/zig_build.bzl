def _zig_build_libghostty_impl(ctx):
    # 1. Retrieve the Zig toolchain and target platform information
    zig_toolchain = ctx.toolchains["@rules_zig//zig:toolchain_type"].zigtoolchaininfo
    zig_target = ctx.toolchains["@rules_zig//zig/target:toolchain_type"].zigtargetinfo
    
    # 2. Get the hermetic zig executable file
    zig_exe = zig_toolchain.zig_exe.file
    
    # 3. Declare the output static library file
    lib_output = ctx.actions.declare_file("libghostty-vt.a")
    
    # 4. Map Bazel's compilation mode to Zig's optimize flag
    compilation_mode = ctx.var.get("COMPILATION_MODE", "fastbuild")
    if compilation_mode == "opt":
        optimize = "ReleaseFast"
    else:
        optimize = "Debug"
        
    # 5. Extract target platform triple (e.g. x86_64-linux)
    target = zig_target.target
    
    # 6. Gather all input source files + toolchain validation/libraries
    inputs = [zig_toolchain.validation]
    if zig_toolchain.zig_lib.file:
        inputs.append(zig_toolchain.zig_lib.file)
    inputs.extend(ctx.files.srcs)
    
    # 7. Construct absolute paths and run the build command
    command = """
    set -e
    OUTPUT_ABS="$(pwd)/{output_path}"
    ZIG_ABS="$(pwd)/{zig_path}"
    CACHE_DIR="$(pwd)/zig-cache"
    mkdir -p "$CACHE_DIR"
    
    # Create a clean, real directory for the build to avoid symlink issues with Zig's directory iteration
    BUILD_DIR="$(pwd)/ghostty-build"
    mkdir -p "$BUILD_DIR"
    
    # Copy everything dereferencing symlinks
    cp -rL third_party/ghostty/* "$BUILD_DIR/"
    
    cd "$BUILD_DIR"
    
    export ZIG_GLOBAL_CACHE_DIR="$CACHE_DIR"
    export ZIG_LOCAL_CACHE_DIR="$CACHE_DIR"
    
    "$ZIG_ABS" build \
        -Demit-lib-vt \
        -Dtarget={target} \
        -Doptimize={optimize} \
        --prefix "$(pwd)/dist"
        
    cp "$(pwd)/dist/lib/libghostty-vt.a" "$OUTPUT_ABS"
    """.format(
        output_path = lib_output.path,
        zig_path = zig_exe.path,
        target = target,
        optimize = optimize,
    )
    
    ctx.actions.run_shell(
        inputs = inputs,
        outputs = [lib_output],
        command = command,
        env = {
            "PATH": "/usr/bin:/bin:/usr/local/bin",
        },
        tools = [zig_exe],
        execution_requirements = {
            "requires-network": "",  # needed to fetch Zig module dependencies
        },
        mnemonic = "ZigBuildLibghostty",
        progress_message = "Building libghostty-vt via zig build",
    )
    
    return [
        DefaultInfo(files = depset([lib_output])),
    ]

zig_build_libghostty = rule(
    implementation = _zig_build_libghostty_impl,
    doc = """Builds the ghostty-vt static library via its native `build.zig`.

Standard `rules_zig` rules (like `zig_library`, `zig_static_library`) do not execute
`zig build` and instead call `zig build-lib` or `zig build-exe` directly on the source files.
However, Ghostty requires executing its `build.zig` to run its internal code-generation
processes and fetch/resolve external lazy dependencies from `build.zig.zon` (like SIMD libraries
and fonts).

This rule extracts the hermetic Zig executable from `@rules_zig`'s toolchain and runs `zig build`
directly inside a copied, symlink-resolved workspace with network access allowed, producing
`libghostty-vt.a` in a cross-compilation aware manner.
""",
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "The source files of the Zig project",
        ),
    },
    toolchains = [
        "@rules_zig//zig:toolchain_type",
        "@rules_zig//zig/target:toolchain_type",
    ],
)
