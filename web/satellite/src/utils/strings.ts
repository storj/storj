// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Size } from '@/utils/bytesSize';

/**
 * CENTS_MB_TO_DOLLARS_GB_SHIFT constant represents how many places to the left
 * a decimal point must be shifted to convert from cents/MB to dollars/GB.
 */
export const CENTS_MB_TO_DOLLARS_GB_SHIFT = -1;

/**
 * CENTS_MB_TO_DOLLARS_TB_SHIFT constant represents how many places to the left
 * a decimal point must be shifted to convert from cents/MB to dollars/TB.
 */
export const CENTS_MB_TO_DOLLARS_TB_SHIFT = -4;

/**
 * decimalShift shifts the decimal point of a number represented by the given string.
 * @param decimal - the string representation of the number
 * @param places - the amount that the decimal point is shifted left
 */
export function decimalShift(decimal: string, places: number): string {
    let sign = '';
    if (decimal[0] === '-') {
        sign = '-';
        decimal = decimal.substring(1);
    }

    const whole = decimal.replace('.', '');
    const dotIdx = (decimal.includes('.') ? decimal.indexOf('.') : decimal.length) - places;

    if (dotIdx < 0) {
        const frac = whole.padStart(whole.length-dotIdx, '0').replace(/0+$/, '');
        if (!frac) {
            return '0';
        }
        return sign + '0.' + frac;
    }

    if (dotIdx >= whole.length) {
        const int = whole.padEnd(dotIdx, '0').replace(/^0+/, '');
        if (!int) {
            return '0';
        }
        return sign + int;
    }

    const int = whole.substring(0, dotIdx).replace(/^0+/, '');
    const frac = whole.substring(dotIdx).replace(/0+$/, '');
    if (!int && !frac) {
        return '0';
    }
    return sign + (int || '0') + (frac ? '.' + frac : '');
}

/**
 * formatPrice formats the decimal string as a price.
 * @param decimal - the decimal string to format
 */
export function formatPrice(decimal: string) {
    let sign = '';
    if (decimal[0] === '-') {
        sign = '-';
        decimal = decimal.substring(1);
    }

    const parts = decimal.split('.');
    const int = parts[0]?.replace(/^0+/, '');
    let frac = '';
    if (parts.length > 1) {
        frac = parts[1].replace(/0+$/, '');
        if (frac) {
            frac = frac.padEnd(2, '0');
        }
    }
    if (!int && !frac) {
        return '$0';
    }

    return sign + '$' + (int || '0') + (frac ? '.' + frac : '');
}

/**
 * centsToDollars formats amounts in cents as dollars.
 * @param cents - the cent value
 */
export function centsToDollars(cents: number) {
    return formatPrice(decimalShift(cents.toString(), 2));
}

/**
 * bytesToBase10String Converts bytes to base-10 types.
 * @param amountInBytes
 */
export function bytesToBase10String(amountInBytes: number) {
    return Size.toBase10String(amountInBytes);
}