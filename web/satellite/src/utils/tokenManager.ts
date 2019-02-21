// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const tokenKey: string = 'tokenKey';

export function getToken(): string {
    return getCookie(tokenKey);
}

export function setToken(tokenValue: string): void {
    document.cookie = tokenKey + '=' + tokenValue + '; path=/';
}

export function removeToken(): void {
    document.cookie = tokenKey + '=; path=/';
}

function getCookie(cname: string): string {
    let name: string = cname + '=';
    let decodedCookie: string = decodeURIComponent(document.cookie);
    let ca: string[] = decodedCookie.split(';');

    for (let i = 0; i < ca.length; i++) {
        let c = ca[i];

        while (c.charAt(0) === ' ') {
            c = c.substring(1);
        }

        if (c.indexOf(name) === 0) {
            return c.substring(name.length, c.length);
        }
    }

    return '';
}
