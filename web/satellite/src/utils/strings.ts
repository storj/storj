// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * decimalShift shifts the decimal point of a number represented by the given string.
 * @param decimal - the string representation of the number
 * @param places - the amount that the decimal point is shifted left
 */
export function decimalShift(decimal: string, places: number): string {
    let sign = '';
    if (decimal[0] == '-') {
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
