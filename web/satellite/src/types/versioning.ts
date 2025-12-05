// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// Versioning is the version state of a project or bucket.
export enum Versioning {
    NotSupported = 'Not Supported',
    Unversioned = 'Unversioned',
    Enabled = 'Enabled',
    Suspended = 'Suspended',
}

export function getVersioning(value: number): Versioning {
    switch (value) {
    case 0:
        return Versioning.NotSupported;
    case 1:
        return Versioning.Unversioned;
    case 2:
        return Versioning.Enabled;
    case 3:
        return Versioning.Suspended;
    default:
        return Versioning.NotSupported;
    }
}
