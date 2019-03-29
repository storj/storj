// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export function validateEmail(email: string): boolean {
    const rgx = /.*@.*\..*$/;

    return rgx.test(email);
}

export function validatePassword(password: string): boolean {
    const rgx = /^(?=.*[0-9])(?=.*[a-zA-Z])([a-zA-Z0-9]+){6,}$/;

    return rgx.test(password);
}
