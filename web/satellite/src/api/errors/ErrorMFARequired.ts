// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorMFARequired is a custom error type for indicating when MFA verification is required.
 */
export class ErrorMFARequired extends Error {
    public constructor(message = 'MFA verification required') {
        super(message);
    }
}
