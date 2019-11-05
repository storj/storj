// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const LOCAL_STORAGE = {
    USER_ID: 'userId',
    SELECTED_PROJECT_ID: 'selectedProjectId',
};

export class LocalData {
    public static get(key: string): string | null {
        return localStorage.getItem(key);
    }

    public static set(key: string, id: string): void {
        localStorage.setItem(key, id);
    }

    public static remove(key: string): void {
        localStorage.removeItem(key);
    }
}
