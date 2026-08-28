const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.resolveTargetQuery(.{ .cpu_arch = .x86_64, .os_tag = .windows, .abi = .gnu });
    const lib = b.addLibrary(.{
        .name = "Storj.CA",
        .linkage = .dynamic,
        .root_module = b.createModule(.{
            .root_source_file = b.path("ca.zig"),
            .target = target,
            .optimize = .ReleaseSmall,
        }),
    });
    lib.linkSystemLibrary("msi");
    lib.linkSystemLibrary("kernel32");
    // Only the DLL is needed; skip the import library and pdb so `--prefix` leaves nothing but bin/.
    b.getInstallStep().dependOn(&b.addInstallArtifact(lib, .{ .implib_dir = .disabled, .pdb_dir = .disabled }).step);

    const tests = b.addTest(.{ .root_module = b.createModule(.{
        .root_source_file = b.path("validate.zig"),
        .target = b.graph.host,
    }) });
    b.step("test", "Run validation tests").dependOn(&b.addRunArtifact(tests).step);
}
