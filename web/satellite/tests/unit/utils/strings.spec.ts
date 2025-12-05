// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { describe, it, expect } from 'vitest';

import { decimalShift, formatPrice, hexToBase64 } from '@/utils/strings';

describe('decimalShift', (): void => {
    it('handles empty strings', (): void => {
        expect(decimalShift('', 0)).toBe('0');
        expect(decimalShift('', 2)).toBe('0');
        expect(decimalShift('', -2)).toBe('0');
    });

    it('shifts integers correctly', (): void => {
        ['', '-'].forEach(sign => {
            const decimal = sign+'123';
            expect(decimalShift(decimal, 0)).toBe(sign+'123');
            expect(decimalShift(decimal, 2)).toBe(sign+'1.23');
            expect(decimalShift(decimal, 5)).toBe(sign+'0.00123');
            expect(decimalShift(decimal, -2)).toBe(sign+'12300');
        });
    });

    it('shifts decimals correctly', (): void => {
        ['', '-'].forEach(sign => {
            const decimal = sign+'1.23';
            expect(decimalShift(decimal, 0)).toBe(sign+'1.23');
            expect(decimalShift(decimal, -2)).toBe(sign+'123');
            expect(decimalShift(decimal, 3)).toBe(sign+'0.00123');
            expect(decimalShift(decimal, -4)).toBe(sign+'12300');
        });
    });

    it('trims unnecessary characters', (): void => {
        ['', '-'].forEach(sign => {
            expect(decimalShift(sign+'0.0012300', -2)).toBe(sign+'0.123');
            expect(decimalShift(sign+'12300', 2)).toBe(sign+'123');
            expect(decimalShift(sign+'000.000', 1)).toBe('0');
        });
    });
});

describe('formatPrice', (): void => {
    it('handles empty strings', (): void => {
        expect(formatPrice('')).toBe('$0');
    });

    it('formats correctly', (): void => {
        ['', '-'].forEach(sign => {
            expect(formatPrice(sign+'123')).toBe(sign+'$123');
            expect(formatPrice(sign+'1.002')).toBe(sign+'$1.002');
        });
    });

    it('adds zeros when necessary', (): void => {
        ['', '-'].forEach(sign => {
            expect(formatPrice(sign+'12.3')).toBe(sign+'$12.30');
            expect(formatPrice(sign+'.123')).toBe(sign+'$0.123');
        });
    });

    it('trims unnecessary characters', (): void => {
        ['', '-'].forEach(sign => {
            expect(formatPrice(sign+'0.0')).toBe('$0');
            expect(formatPrice(sign+'00123.00')).toBe(sign+'$123');
        });
    });
});

describe('hexToBase64', () => {
    it('rejects non-hex strings', () => {
        expect(() => hexToBase64('foobar')).toThrowError();
    });

    it('rejects short strings', () => {
        expect(() => hexToBase64('abc')).toThrowError();
    });

    it('handles empty strings', () => {
        expect(hexToBase64('')).toBe('');
    });

    it('encodes properly', () => {
        expect(hexToBase64('14fb9c03d97e')).toBe('FPucA9l-');
        expect(hexToBase64('14fb9c03d9')).toBe('FPucA9k=');
        expect(hexToBase64('14fb9c03')).toBe('FPucAw==');
    });
});
