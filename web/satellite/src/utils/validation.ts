// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: move functions to Validator class
export function validateEmail(email: string): boolean {
    const rgx = /.*@.*\..*$/;

    return rgx.test(email);
}

export function validatePassword(password: string): boolean {
    return typeof password !== 'undefined' && password.length >= 6;
}

export class Validator {
    public static validateTokenAmount(amount: string): boolean {
        const rgx = /^\d+(\.\d+)?$/;

        return rgx.test(amount);
    }
}
