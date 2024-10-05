// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 *  A custom error class with status and requestID properties.
 */
export class APIError extends Error {
    public status: number;
    public requestID: string | null;

    constructor(data: {status: number, message: string, requestID: string | null}) {
        super(data.message);
        this.message = data.message;
        this.requestID = data.requestID;
    }

    /**
     * Returns a new APIError with the same status and requestID but with a different message.
     */
    public withMessage(message: string): APIError {
        return new APIError({
            status: this.status,
            message,
            requestID: this.requestID,
        });
    }
}

/**
 *  A custom error class for reporting duplicate file uploads.
 */
export class DuplicateUploadError extends Error {
    constructor(
        public files: string[],
    ) {
        super('Duplicate upload');
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