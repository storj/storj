// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorTooManyRequests is a custom error type for performing 'too many requests' operations.
 */
export class ErrorTooManyRequests extends Error {
    public constructor(message = 'Too many requests') {
        super(message);
    }
}
