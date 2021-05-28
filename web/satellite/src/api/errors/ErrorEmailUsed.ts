// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorEmailUsed is a custom error type for performing 'email is already in use' operations.
 */
export class ErrorEmailUsed extends Error {
    public constructor(message: string = 'email used') {
        super(message);
    }
}
