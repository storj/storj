// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    validateEmail,
    validatePassword, Validator,
} from '@/utils/validation';

describe('validation', () => {
    it('validatePassword regex works correctly', () => {
        const testString1 = 'test';
        const testString2 = '        '.trim();
        const testString3 = 'test %%%';
        const testString4 = 'testtest';
        const testString5 = 'test1233';
        const testString6 = 'test1';
        const testString7 = 'teSTt1123';

        expect(validatePassword(testString1)).toBe(false);
        expect(validatePassword(testString2)).toBe(false);
        expect(validatePassword(testString3)).toBe(true);
        expect(validatePassword(testString4)).toBe(true);
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

    it('Validator validateTokenAmount works correctly', () => {
        const expectedValidAmounts = [
            '10',
            '0.1',
            '00.1111',
            '0001.2',
            '01',
            '023.11',
        ];

        const expectedInvalidAmounts = [
            '-21',
            're',
            '',
            '21.',
            's.12',
            '.23',
            '23,21',
            '-',
        ];

        expectedValidAmounts.forEach(amount => {
            expect(Validator.validateTokenAmount(amount)).toBe(true);
        });
        expectedInvalidAmounts.forEach(amount => {
            expect(Validator.validateTokenAmount(amount)).toBe(false);
        });
    });
});
