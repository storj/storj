// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Size } from '@/utils/bytesSize';

/**
 * CENTS_MB_TO_DOLLARS_GB_SHIFT constant represents how many places to the left
 * a decimal point must be shifted to convert from cents/MB to dollars/GB.
 */
export const CENTS_MB_TO_DOLLARS_GB_SHIFT = -1;

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
        const frac = whole.padStart(whole.length - dotIdx, '0').replace(/0+$/, '');
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
 * microDollarsToCents converts micro dollars to cents.
 * @param microDollars - the micro dollars value
 */
export function microDollarsToCents(microDollars: number): number {
    return microDollars / 10000;
}

/**
 * bytesToBase10String Converts bytes to base-10 types.
 * @param amountInBytes
 */
export function bytesToBase10String(amountInBytes: number) {
    return Size.toBase10String(amountInBytes);
}

/**
 * Returns a human-friendly form of an array, inserting commas and "and"s where necessary.
 * @param arr - the array
 */
export function humanizeArray(arr: string[]): string {
    const len = arr.length;
    switch (len) {
    case 0: return '';
    case 1: return arr[0];
    case 2: return arr.join(' and ');
    default: return `${arr.slice(0, len - 1).join(', ')}, and ${arr[len - 1]}`;
    }
}

const b64Chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';

/**
 * Returns the URL-safe base64 representation of a hexadecimal string.
 * @param str - the hex string
 */
export function hexToBase64(str: string): string {
    if (!str) return '';
    if (str.length % 2) {
        throw new Error(`Invalid length ${str.length} for hex string`);
    }

    const bytes = new Uint8Array(str.length / 2);
    for (let i = 0; i < str.length; i += 2) {
        const byteStr = str.substring(i, i + 2);
        const n = parseInt(byteStr, 16);
        if (isNaN(n)) {
            throw new Error(`Invalid hex byte '${byteStr}' at position ${i}`);
        }
        bytes[i / 2] = parseInt(str.substring(i, i + 2), 16);
    }

    let out = '';
    for (let i = 0; i < bytes.length; i += 3) {
        out += b64Chars[(bytes[i] & 0b11111100) >> 2];

        let nextSextet = (bytes[i] & 0b00000011) << 4;
        if (i + 1 >= bytes.length) {
            out += b64Chars[nextSextet];
            break;
        }
        nextSextet |= (bytes[i + 1] & 0b11110000) >> 4;
        out += b64Chars[nextSextet];

        nextSextet = (bytes[i + 1] & 0b00001111) << 2;
        if (i + 2 >= bytes.length) {
            out += b64Chars[nextSextet];
            break;
        }
        nextSextet |= (bytes[i + 2] & 0b11000000) >> 6;
        out += b64Chars[nextSextet];

        out += b64Chars[bytes[i + 2] & 0b00111111];
    }

    if (out.length % 4) {
        return out.padEnd(out.length + (4 - (out.length % 4)), '=');
    }
    return out;
}
