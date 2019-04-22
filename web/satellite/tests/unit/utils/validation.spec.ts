// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    validatePassword,
    validateEmail
 } from '@/utils/validation';

describe('validation', () => {
    it('validatePassword regex works correctly', () => {
        const testString1 = 'test';
        const testString2 = '        ';
        const testString3 = 'test %%%';
        const testString4 = 'testtest';
        const testString5 = 'test1233';
        const testString6 = 'test1';
        const testString7 = 'teSTt1123';

        expect(validatePassword(testString1)).toBe(false);
        expect(validatePassword(testString2)).toBe(false);
        expect(validatePassword(testString3)).toBe(false);
        expect(validatePassword(testString4)).toBe(false);
        expect(validatePassword(testString5)).toBe(true);
        expect(validatePassword(testString6)).toBe(false);
        expect(validatePassword(testString7)).toBe(true);
    });
    
    it('validateEmail regex works correctly', () => {
        const testString1 = 'test';
        const testString2 = '        ';
        const testString3 = 'test@';
        const testString4 = 'test.test';
        const testString5 = 'test1@23.3';
        const testString6 = '';
        const testString7 = '@teSTt.1123';

        expect(validateEmail(testString1)).toBe(false);
        expect(validateEmail(testString2)).toBe(false);
        expect(validateEmail(testString3)).toBe(false);
        expect(validateEmail(testString4)).toBe(false);
        expect(validateEmail(testString5)).toBe(true);
        expect(validateEmail(testString6)).toBe(false);
        expect(validateEmail(testString7)).toBe(true);
    });
});
