// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorConflict is a custom error type indicating operation failure due to a resource conflict.
 */
export class ErrorConflict extends Error {
    public constructor(message = 'Resource conflict') {
        super(message);
    }
}
