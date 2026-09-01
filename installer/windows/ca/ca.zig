// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

//! MSI custom action DLL for the storagenode installer.
//! Each exported function reads MSI properties, validates them via validate.zig
//! and writes "1" (valid) or an error message into the *_VALID property.

const std = @import("std");
const windows = std.os.windows;
const v = @import("validate.zig");
const W = v.W;

const MSIHANDLE = u32;
const ERROR_SUCCESS: u32 = 0;

extern "msi" fn MsiGetPropertyW(h: MSIHANDLE, name: [*:0]const u16, buf: [*]u16, len: *u32) callconv(.winapi) u32;
extern "msi" fn MsiSetPropertyW(h: MSIHANDLE, name: [*:0]const u16, value: [*:0]const u16) callconv(.winapi) u32;
extern "kernel32" fn GetDiskFreeSpaceExW(path: [*:0]const u16, free: ?*u64, total: ?*u64, total_free: ?*u64) callconv(.winapi) windows.BOOL;
extern "kernel32" fn GetFileAttributesW(path: [*:0]const u16) callconv(.winapi) u32;
extern "kernel32" fn GetVolumePathNameW(path: [*:0]const u16, volume: [*]u16, len: u32) callconv(.winapi) windows.BOOL;
extern "kernel32" fn GetDriveTypeW(root: [*:0]const u16) callconv(.winapi) u32;

const DRIVE_FIXED: u32 = 3;

const ERROR_MORE_DATA: u32 = 234;
const ERROR_INSTALL_FAILURE: u32 = 1603;
const INVALID_FILE_ATTRIBUTES: u32 = 0xFFFFFFFF;
const FILE_ATTRIBUTE_DIRECTORY: u32 = 0x10;

const max_path = v.max_path;

const GetPropError = error{ TooLong, Failed };

/// Reads a property; an unset property is an empty slice, a value longer than buf is error.TooLong.
fn getProp(h: MSIHANDLE, name: [*:0]const u16, buf: []u16) GetPropError![]const u16 {
    var n: u32 = @intCast(buf.len - 1);
    return switch (MsiGetPropertyW(h, name, buf.ptr, &n)) {
        ERROR_SUCCESS => buf[0..n],
        ERROR_MORE_DATA => error.TooLong,
        else => error.Failed,
    };
}

fn readError(err: GetPropError) v.Msg {
    return switch (err) {
        error.TooLong => W("The value is too long."),
        error.Failed => W("Could not read the value."),
    };
}

fn setProp(h: MSIHANDLE, name: [*:0]const u16, val: [*:0]const u16) void {
    _ = MsiSetPropertyW(h, name, val);
}

fn toZ(buf: []u16, s: []const u16) [:0]const u16 {
    @memcpy(buf[0..s.len], s);
    buf[s.len] = 0;
    return buf[0..s.len :0];
}

fn attrs(p: []const u16) u32 {
    var z: [max_path + 1]u16 = undefined;
    if (p.len >= z.len) return INVALID_FILE_ATTRIBUTES;
    return GetFileAttributesW(toZ(&z, p).ptr);
}

fn dirExists(p: []const u16) bool {
    const a = attrs(p);
    return a != INVALID_FILE_ATTRIBUTES and (a & FILE_ATTRIBUTE_DIRECTORY) != 0;
}

fn fileExists(p: []const u16) bool {
    const a = attrs(p);
    return a != INVALID_FILE_ATTRIBUTES and (a & FILE_ATTRIBUTE_DIRECTORY) == 0;
}

/// Size of the volume containing p. GetDiskFreeSpaceEx needs an existing directory, and the storage
/// folder is usually created during install, so resolve the volume mount point first (like DriveInfo did).
fn driveSize(p: []const u16) ?u64 {
    var z: [max_path + 1]u16 = undefined;
    if (p.len >= z.len) return null;
    var volume: [max_path + 1]u16 = undefined;
    if (!GetVolumePathNameW(toZ(&z, p).ptr, &volume, volume.len).toBool()) return null;
    var total: u64 = 0;
    if (!GetDiskFreeSpaceExW(@ptrCast(&volume), null, &total, null).toBool()) return null;
    return total;
}

const fs: v.Fs = .{ .dirExists = dirExists, .fileExists = fileExists, .driveSize = driveSize };

fn report(h: MSIHANDLE, out: [*:0]const u16, err: ?v.Msg) u32 {
    setProp(h, out, err orelse W("1"));
    return ERROR_SUCCESS;
}

pub export fn ValidateIdentityDir(h: MSIHANDLE) callconv(.winapi) u32 {
    var buf: [1024]u16 = undefined;
    const dir = getProp(h, W("IDENTITYDIR"), &buf) catch |err| return report(h, W("STORJ_IDENTITYDIR_VALID"), readError(err));
    return report(h, W("STORJ_IDENTITYDIR_VALID"), v.checkIdentityDir(fs, dir));
}

pub export fn ValidateWallet(h: MSIHANDLE) callconv(.winapi) u32 {
    var buf: [1024]u16 = undefined;
    const wallet = getProp(h, W("STORJ_WALLET"), &buf) catch |err| return report(h, W("STORJ_WALLET_VALID"), readError(err));
    return report(h, W("STORJ_WALLET_VALID"), v.checkWallet(wallet));
}

pub export fn ValidateStorageDir(h: MSIHANDLE) callconv(.winapi) u32 {
    var buf: [1024]u16 = undefined;
    const dir = getProp(h, W("STORAGEDIR"), &buf) catch |err| return report(h, W("STORJ_STORAGEDIR_VALID"), readError(err));
    return report(h, W("STORJ_STORAGEDIR_VALID"), v.checkStorageDir(fs, dir));
}

pub export fn ValidateStorage(h: MSIHANDLE) callconv(.winapi) u32 {
    var sbuf: [64]u16 = undefined;
    var dbuf: [1024]u16 = undefined;
    const storage = getProp(h, W("STORJ_STORAGE"), &sbuf) catch |err| return report(h, W("STORJ_STORAGE_VALID"), readError(err));
    const dir = getProp(h, W("STORAGEDIR"), &dbuf) catch |err| return report(h, W("STORJ_STORAGE_VALID"), readError(err));
    return report(h, W("STORJ_STORAGE_VALID"), v.checkStorage(fs, storage, dir));
}

/// On upgrade, reuse the install folder of the already registered service.
/// Runs before CostFinalize, so setting the directory property overrides the default location.
pub export fn ExtractInstallDir(h: MSIHANDLE) callconv(.winapi) u32 {
    var buf: [max_path]u16 = undefined;
    var z: [max_path]u16 = undefined;
    // A truncated command line must not silently leave the install folder unset; fail the upgrade instead.
    const cmd = getProp(h, W("STORJ_SERVICE_COMMAND"), &buf) catch return ERROR_INSTALL_FAILURE;
    if (v.extractInstallDir(cmd)) |dir| setProp(h, W("INSTALLFOLDER"), toZ(&z, dir));
    return ERROR_SUCCESS;
}

/// Preselects the identity folder created by `identity create storagenode` when it exists.
/// Only ever fills in an empty IDENTITYDIR: the action runs in both the UI and the execute sequence,
/// and the execute sequence must keep what the operator picked in the wizard.
pub export fn SetDefaultIdentityDir(h: MSIHANDLE) callconv(.winapi) u32 {
    var buf: [1024]u16 = undefined;
    var path: [1024]u16 = undefined;
    const current = getProp(h, W("IDENTITYDIR"), &buf) catch return ERROR_SUCCESS;
    if (current.len != 0) return ERROR_SUCCESS;
    const appdata = getProp(h, W("AppDataFolder"), &buf) catch return ERROR_SUCCESS;
    const suffix = W("Storj\\Identity\\storagenode\\");
    if (appdata.len == 0 or appdata.len + suffix.len >= path.len) return ERROR_SUCCESS;
    @memcpy(path[0..appdata.len], appdata);
    @memcpy(path[appdata.len..][0..suffix.len], suffix);
    path[appdata.len + suffix.len] = 0;
    const dir = path[0 .. appdata.len + suffix.len :0];
    if (dirExists(dir)) setProp(h, W("IDENTITYDIR"), dir);
    return ERROR_SUCCESS;
}

/// Replacement for WixUIValidatePath: the install folder must be on a local fixed drive.
/// Sets WIXUI_INSTALLDIR_VALID to "1" or "0"; the dialogs spawn InvalidDirDlg when it is not "1".
pub export fn ValidateInstallDir(h: MSIHANDLE) callconv(.winapi) u32 {
    var nbuf: [256]u16 = undefined;
    var buf: [max_path]u16 = undefined;
    var z: [max_path + 1]u16 = undefined;
    var volume: [max_path + 1]u16 = undefined;
    setProp(h, W("WIXUI_INSTALLDIR_VALID"), W("0"));
    // WIXUI_INSTALLDIR holds the name of the directory property to validate (INSTALLFOLDER).
    const name = getProp(h, W("WIXUI_INSTALLDIR"), &nbuf) catch return ERROR_SUCCESS;
    if (name.len == 0 or name.len >= z.len) return ERROR_SUCCESS;
    const dir = getProp(h, toZ(&z, name).ptr, &buf) catch return ERROR_SUCCESS;
    if (dir.len == 0 or dir.len >= z.len) return ERROR_SUCCESS;
    if (!GetVolumePathNameW(toZ(&z, dir).ptr, &volume, volume.len).toBool()) return ERROR_SUCCESS;
    if (GetDriveTypeW(@ptrCast(&volume)) != DRIVE_FIXED) return ERROR_SUCCESS;
    setProp(h, W("WIXUI_INSTALLDIR_VALID"), W("1"));
    return ERROR_SUCCESS;
}
