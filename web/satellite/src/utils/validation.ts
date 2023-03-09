// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { MetaUtils } from '@/utils/meta';

/**
 * Validator holds validation check methods for strings.
 */
export class Validator {
    public static readonly PASS_MIN_LENGTH = parseInt(MetaUtils.getMetaContent('password-minimum-length'));
    public static readonly PASS_MAX_LENGTH = parseInt(MetaUtils.getMetaContent('password-maximum-length'));

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
        return typeof password !== 'undefined'
            && password.length >= this.PASS_MIN_LENGTH
            && password.length <= this.PASS_MAX_LENGTH;
    }

    /**
     * Checks string to satisfy bucket name rules.
     */
    public static bucketName(value: string): boolean {
        const rgx = /^[a-z0-9][a-z0-9.-]+[a-z0-9]$/;

        return rgx.test(value);
    }
}
