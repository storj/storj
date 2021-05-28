// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Validator holds validation check methods for strings.
 */
export class Validator {

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

    /**
     * Checks string to satisfy bucket name rules.
     */
    public static bucketName(value: string): boolean {
        const rgx = /^[a-z0-9]+$/;

        return rgx.test(value);
    }

    /**
     * Checks string to consist of 1 word.
     */
    public static oneWordString(value: string): boolean {
        const trimmed = value.trim();

        return trimmed.indexOf(' ') === -1;
    }
}
