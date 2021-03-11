// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorUnauthorized is a custom error type for performing unauthorized operations.
 */
export class UnauthorizedError extends Error {
    public constructor(message: string = 'authorization required') {
        super(message);
    }
}

/**
 * BadRequestError is a custom error type for performing bad request.
 */
export class BadRequestError extends Error {
    public constructor(message: string = 'bad request') {
        super(message);
    }
}

/**
 * InternalError is a custom error type for internal server error.
 */
export class InternalError extends Error {
    public constructor(message: string = 'internal server error') {
        super(message);
    }
}
