// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class RequestResponse<T> {
    public isSuccess: boolean;
    public errorMessage: string;
    public data: T;

    public constructor() {
        this.isSuccess = false;
        this.errorMessage = '';

        this.data = {} as T;
    }
}
