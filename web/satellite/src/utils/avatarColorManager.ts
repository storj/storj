// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// getColor - returns color string depends on first symbol of first name
export function getColor(symbol): string {
    switch (symbol) {
        case 'A':
        case 'I':
        case 'Q':
        case 'Y':
            return '#373737';
        case 'B':
        case 'J':
        case 'R':
        case 'Z':
            return '#5B58FF';
        case 'C':
        case 'K':
        case 'S':
            return '#FFD058';
        case 'D':
        case 'L':
        case 'T':
            return '#58B9FF';
        case 'E':
        case 'M':
        case 'U':
            return '#95D486';
        case 'F':
        case 'N':
        case 'V':
            return '#5F5E8D';
        case 'G':
        case 'O':
        case 'W':
            return '#FF4F4D';
        default: // case 'H', 'P', 'X'
            return '#FF8658';
    }
}
