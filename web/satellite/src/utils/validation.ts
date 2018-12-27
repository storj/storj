// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

export function validateEmail(email: string) : boolean {
    const rgx = /.*@.*\..*$/;

    return rgx.test(email);
}
