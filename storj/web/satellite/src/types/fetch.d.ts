// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare type Answer = {
    isSuccess: boolean;
    error: {
        code: any;
        message: any;
    };
};

declare type OnPageClickCallback = (index: number) => Promise<any>;

declare type CheckSelected = (index: number) => boolean;
