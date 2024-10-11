// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorTooManyAttempts is a custom error type for indicating when user makes auth-related actions ton many times.
 */
export class ErrorTooManyAttempts extends Error {
    public constructor(message = 'Too many attempts, please try again later') {
        super(message);
    }
}
