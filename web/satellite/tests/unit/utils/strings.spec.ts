// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { decimalShift } from '@/utils/strings';

describe('decimalShift', (): void => {
    it('shifts integers correctly', function() {
        ['', '-'].forEach(sign => {
            const decimal = sign+'123';
            expect(decimalShift(decimal, 0)).toBe(sign+'123');
            expect(decimalShift(decimal, 2)).toBe(sign+'1.23');
            expect(decimalShift(decimal, 5)).toBe(sign+'0.00123');
            expect(decimalShift(decimal, -2)).toBe(sign+'12300');
        });
    });

    it('shifts decimals correctly', function() {
        ['', '-'].forEach(sign => {
            const decimal = sign+'1.23';
            expect(decimalShift(decimal, 0)).toBe(sign+'1.23');
            expect(decimalShift(decimal, -2)).toBe(sign+'123');
            expect(decimalShift(decimal, 3)).toBe(sign+'0.00123');
            expect(decimalShift(decimal, -4)).toBe(sign+'12300');
        });
    });

    it('trims unnecessary characters', function() {
        ['', '-'].forEach(sign => {
            expect(decimalShift(sign+'0.0012300', -2)).toBe(sign+'0.123');
            expect(decimalShift(sign+'12300', 2)).toBe(sign+'123');
            expect(decimalShift(sign+'000.000', 1)).toBe('0');
        });
    });
});
