// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class EmailInput {
    public value: string;
    public error: boolean;

    constructor() {
        this.value = '';
        this.error = false;
    }

    public setError(error: boolean) {
        this.error = error;
    }
}
