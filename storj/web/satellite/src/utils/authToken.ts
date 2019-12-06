// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// AuthToken exposes methods to manage auth cookie
export class AuthToken {
    private static tokenKey: string = '_tokenKey';

    public static get(): string {
        return AuthToken.getCookie(AuthToken.tokenKey);
    }

    public static set(tokenValue: string): void {
        document.cookie = AuthToken.tokenKey + '=' + tokenValue + '; path=/';
    }

    public static remove(): void {
        document.cookie = AuthToken.tokenKey + '=; path=/';
    }

    private static getCookie(cname: string): string {
        const name: string = cname + '=';
        const decodedCookie: string = decodeURIComponent(document.cookie);
        const ca: string[] = decodedCookie.split(';');

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
}
