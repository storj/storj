// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/private/http/client';

/**
 * ErrorUnauthorized is a custom error type for performing unauthorized operations.
 */
export class UnauthorizedError extends Error {
    public constructor(message = 'authorization required') {
        super(message);
    }
}

/**
 * BadRequestError is a custom error type for performing bad request.
 */
export class BadRequestError extends Error {
    public constructor(message = 'bad request') {
        super(message);
    }
}

/**
 * InternalError is a custom error type for internal server error.
 */
export class InternalError extends Error {
    public constructor(message = 'internal server error') {
        super(message);
    }
}

/**
 * APIClient is base client that holds http client and error handler.
 */
export class APIClient {
    protected readonly http: HttpClient = new HttpClient();

    /**
     * handles error due to response code.
     * @param response - response from server.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid ISBN number.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the ISBN number is valid, but no such book exists in the catalog.
     *
     * @throws {@link InternalError}
     * Thrown if the ISBN number is valid, but no such book exists in the catalog.
     *
     * @private
     */
    protected async handleError(response: Response): Promise<void> {
        const body = await response.json();

        switch (response.status) {
        case 401: throw new UnauthorizedError(body.error);
        case 400: throw new BadRequestError(body.error);
        case 500:
        default:
            throw new InternalError(body.error);
        }
    }
}
