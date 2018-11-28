// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

const tokenKey : string = 'tokenKey';

export function getToken() : string|null  {
    return sessionStorage.getItem(tokenKey);
}

export function setToken(tokenValue : string) : void {
    sessionStorage.setItem(tokenKey, tokenValue);
}

export function removeToken() : void {
    sessionStorage.removeItem(tokenKey);
};