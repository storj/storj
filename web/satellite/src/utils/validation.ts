// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: move functions to Validator class
export function validateEmail(email: string): boolean {
    const rgx = /.*@.*\..*$/;

    /**
     * Checks string to satisfy email rules.
     */
    public static email(email: string): boolean {
        const rgx = /.*@.*\..*$/;

        return rgx.test(email);
    }

    /**
     * Checks string to satisfy password rules.
     */
    public static password(password: string): boolean {
        return typeof password !== 'undefined' && password.length >= 6;
    }
}
