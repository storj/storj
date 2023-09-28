// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Custom string id generator.
 */
export function getId(): string {
    return '_' + Math.random().toString(36).substr(2, 9);
}

/**
 * Returns random UUID in "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" format.
 */
export function randomUUID(): string {
    const randHex = (numBytes: number): string => {
        let str = '';
        for (let i = 0; i < numBytes; i++) {
            str += Math.round(Math.random()*255).toString(16).padStart(2, '0');
        }
        return str;
    };
    return [randHex(4), randHex(2), randHex(2), randHex(2), randHex(6)].join('-');
}
