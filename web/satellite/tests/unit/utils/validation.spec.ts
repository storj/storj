// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { describe, it, expect } from 'vitest';

import { Validator } from '@/utils/validation';

describe('validation', (): void => {
    it('email regex works correctly', () => {
        const testString1 = 'test';
        const testString2 = '        ';
        const testString3 = 'test@';
        const testString4 = 'test.test';
        const testString5 = 'test1@23.3';
        const testString6 = '';
        const testString7 = '@teSTt.1123';

        expect(Validator.email(testString1)).toBe(false);
        expect(Validator.email(testString2)).toBe(false);
        expect(Validator.email(testString3)).toBe(false);
        expect(Validator.email(testString4)).toBe(false);
        expect(Validator.email(testString5)).toBe(true);
        expect(Validator.email(testString6)).toBe(false);
        expect(Validator.email(testString7)).toBe(true);
    });
});
