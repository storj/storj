// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export function validateEmail(email: string): boolean {
    const rgx = /.*@.*\..*$/;

    return rgx.test(email);
}

export function validatePassword(password: string): boolean {
    return typeof password != 'undefined' && password.length >= 6;
}
