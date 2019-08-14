// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const USER_ID: string = 'userID';

export function setUserId(userId: string) {
    localStorage.setItem(USER_ID, userId);
}

export function getUserId() {
    return localStorage.getItem(USER_ID);
}
