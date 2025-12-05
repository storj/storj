// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 *  A custom error class with status and requestID properties.
 */
export class APIError extends Error {
    public status: number;
    public requestID: string | null;

    constructor(data: { status: number, message: string, requestID: string | null }) {
        super(data.message);
        this.status = data.status;
        this.message = data.message;
        this.requestID = data.requestID;
    }
}

/**
 *  A custom error class for reporting error for object deletes.
 */
export class ObjectDeleteError extends Error {
    constructor(
        public deletedCount: number,
        public message: string,
    ) {
        super(message);
    }
}