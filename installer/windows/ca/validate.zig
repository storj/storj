// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

//! Pure validation logic for the storagenode MSI custom actions.
//! No Windows dependencies, so `zig build test` runs anywhere.

const std = @import("std");

pub inline fn W(comptime s: []const u8) *const [std.unicode.calcUtf16LeLen(s) catch unreachable:0]u16 {
    comptime @setEvalBranchQuota(10_000);
    return std.unicode.utf8ToUtf16LeStringLiteral(s);
}
pub const Msg = [:0]const u16;

const GB: u64 = 1000 * 1000 * 1000;
const TB: u64 = 1000 * GB;
/// Minimum total size of the drive holding the storage folder (500 GB + 10% overhead), as in the .NET version.
pub const min_drive_size: u64 = 550 * GB;

/// Upper bound for paths handled here; MSI properties are read into smaller buffers, this leaves room for joining.
pub const max_path = 4096;

pub const Fs = struct {
    dirExists: *const fn ([]const u16) bool,
    fileExists: *const fn ([]const u16) bool,
    /// Total size in bytes of the volume containing the path. The path itself does not need to exist.
    driveSize: *const fn ([]const u16) ?u64,
};

/// Returns null when valid, otherwise the user-facing error message.
pub fn checkIdentityDir(fs: Fs, dir: []const u16) ?Msg {
    if (dir.len == 0) return W("You must select an identity folder.");
    if (!fs.dirExists(dir)) return W("The selected identity folder does not exist.");
    var buf: [max_path]u16 = undefined;
    for ([_][:0]const u16{ W("ca.cert"), W("identity.cert"), W("identity.key") }, [_]Msg{
        W("File 'ca.cert' not found in the selected folder."),
        W("File 'identity.cert' not found in the selected folder."),
        W("File 'identity.key' not found in the selected folder."),
    }) |name, msg| {
        const path = join(&buf, dir, name) orelse return W("The selected identity folder path is too long.");
        if (!fs.fileExists(path)) return msg;
    }
    return null;
}

pub fn checkWallet(wallet: []const u16) ?Msg {
    if (wallet.len == 0) return W("The payout address cannot be empty.");
    if (!std.mem.startsWith(u16, wallet, W("0x"))) return W("The payout address must start with a '0x' prefix.");
    if (wallet.len - 2 != 40) return W("The payout address must have 40 characters after the '0x' prefix.");
    return null;
}

pub fn checkStorageDir(fs: Fs, dir: []const u16) ?Msg {
    if (dir.len == 0) return W("You must select a storage folder.");
    const size = fs.driveSize(dir) orelse return W("Could not determine the size of the selected drive.");
    if (size < min_drive_size) return W("The selected drive is smaller than 550 GB. The minimum required is 550 GB.");
    return null;
}

pub fn checkStorage(fs: Fs, raw_storage_str: []const u16, dir: []const u16) ?Msg {
    // The .NET version used NumberStyles.Number, which ignores surrounding whitespace.
    const storage_str = std.mem.trim(u16, raw_storage_str, W(" \t\r\n"));
    if (storage_str.len == 0) return W("The value cannot be empty.");
    var ascii: [64]u8 = undefined;
    if (storage_str.len > ascii.len) return W("The value is not a valid number.");
    for (storage_str, 0..) |ch, i| {
        if (ch > 0x7f) return W("The value is not a valid number.");
        ascii[i] = @intCast(ch);
    }
    const storage = std.fmt.parseFloat(f64, ascii[0..storage_str.len]) catch return W("The value is not a valid number.");
    if (!(storage >= 0.5)) return W("The allocated disk space cannot be less than 0.5 TB.");
    if (dir.len == 0) return W("The storage directory cannot be empty.");
    if (storage > 1000 * 1000) return W("The allocated disk space is too large.");
    const with_overhead: u64 = @intFromFloat(storage * 1.1 * @as(f64, @floatFromInt(TB)));
    const size = fs.driveSize(dir) orelse return W("Could not determine the size of the selected drive.");
    if (size < with_overhead) return W("The disk size on the selected drive is less than the allocated disk space plus the 10% overhead.");
    return null;
}

/// Extracts the path from `--config-dir "<path>"` in the service ImagePath.
pub fn extractInstallDir(cmd: []const u16) ?[]const u16 {
    const prefix = W("--config-dir \"");
    const start = (std.mem.indexOf(u16, cmd, prefix) orelse return null) + prefix.len;
    // A Windows path cannot contain '"', so the next quote ends the value even if more quoted arguments follow.
    const end = std.mem.indexOfScalarPos(u16, cmd, start, '"') orelse return null;
    if (end == start) return null;
    return cmd[start..end];
}

/// Returns null when the joined path does not fit in buf.
fn join(buf: []u16, dir: []const u16, name: []const u16) ?[]const u16 {
    const sep: usize = if (dir[dir.len - 1] == '\\' or dir[dir.len - 1] == '/') 0 else 1;
    if (dir.len + sep + name.len > buf.len) return null;
    @memcpy(buf[0..dir.len], dir);
    if (sep == 1) buf[dir.len] = '\\';
    @memcpy(buf[dir.len + sep ..][0..name.len], name);
    return buf[0 .. dir.len + sep + name.len];
}

// ---- tests ----

const testing = std.testing;

fn yes(_: []const u16) bool {
    return true;
}
fn no(_: []const u16) bool {
    return false;
}
fn onlyCerts(p: []const u16) bool {
    return std.mem.endsWith(u16, p, W(".cert"));
}
fn bigDrive(_: []const u16) ?u64 {
    return 10 * TB;
}
fn smallDrive(_: []const u16) ?u64 {
    return 100 * GB;
}

test "wallet" {
    try testing.expect(checkWallet(W("0x" ++ "a" ** 40)) == null);
    try testing.expect(checkWallet(W("")) != null);
    try testing.expect(checkWallet(W("abc")) != null);
    try testing.expect(checkWallet(W("0xabc")) != null);
}

test "identity dir" {
    try testing.expect(checkIdentityDir(.{ .dirExists = yes, .fileExists = yes, .driveSize = bigDrive }, W("C:\\id")) == null);
    try testing.expect(checkIdentityDir(.{ .dirExists = yes, .fileExists = yes, .driveSize = bigDrive }, W("")) != null);
    try testing.expect(checkIdentityDir(.{ .dirExists = no, .fileExists = yes, .driveSize = bigDrive }, W("C:\\id")) != null);
    try testing.expect(checkIdentityDir(.{ .dirExists = yes, .fileExists = onlyCerts, .driveSize = bigDrive }, W("C:\\id\\")) != null);
}

test "identity dir too long" {
    const long = [_]u16{'a'} ** max_path;
    try testing.expect(checkIdentityDir(.{ .dirExists = yes, .fileExists = yes, .driveSize = bigDrive }, &long) != null);
}

test "storage dir" {
    try testing.expect(checkStorageDir(.{ .dirExists = yes, .fileExists = yes, .driveSize = bigDrive }, W("D:\\")) == null);
    try testing.expect(checkStorageDir(.{ .dirExists = yes, .fileExists = yes, .driveSize = smallDrive }, W("D:\\")) != null);
}

test "storage" {
    const fs: Fs = .{ .dirExists = yes, .fileExists = yes, .driveSize = bigDrive };
    try testing.expect(checkStorage(fs, W("1.5"), W("D:\\")) == null);
    try testing.expect(checkStorage(fs, W("9"), W("D:\\")) == null);
    try testing.expect(checkStorage(fs, W("9.1"), W("D:\\")) != null); // 9.1*1.1 > 10 TB
    try testing.expect(checkStorage(fs, W("0.4"), W("D:\\")) != null);
    try testing.expect(checkStorage(fs, W(" 1.5 "), W("D:\\")) == null);
    try testing.expect(checkStorage(fs, W("abc"), W("D:\\")) != null);
    try testing.expect(checkStorage(fs, W(""), W("D:\\")) != null);
    try testing.expect(checkStorage(fs, W("1"), W("")) != null);
}

test "extract install dir" {
    const cmd = W("\"C:\\Program Files\\Storj\\storagenode.exe\" run --config-dir \"C:\\Program Files\\Storj\\\" --log.output x");
    try testing.expectEqualSlices(u16, W("C:\\Program Files\\Storj\\"), extractInstallDir(cmd).?);
    const trailing = W("\"C:\\x\\storagenode.exe\" run --config-dir \"C:\\x\\\" --log.output \"winfile:///C:\\x\\log\"");
    try testing.expectEqualSlices(u16, W("C:\\x\\"), extractInstallDir(trailing).?);
    try testing.expect(extractInstallDir(W("storagenode.exe run --config-dir \"\"")) == null);
    try testing.expect(extractInstallDir(W("storagenode.exe run")) == null);
    try testing.expect(extractInstallDir(W("")) == null);
}
