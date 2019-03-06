// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare type RequestResponse<T> = {
    isSuccess: boolean,
    errorMessage: string,
    data: T
};
