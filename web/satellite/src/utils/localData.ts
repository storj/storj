// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * LocalData exposes methods to manage local storage.
 */
export class LocalData {
    private static userId: string = 'userId';
    private static selectedProjectId: string = 'selectedProjectId';

    public static getUserId(): string | null {
        return localStorage.getItem(LocalData.userId);
    }

    public static setUserId(id: string): void {
        localStorage.setItem(LocalData.userId, id);
    }

    public static removeUserId(): void {
        localStorage.removeItem(LocalData.userId);
    }

    public static getSelectedProjectId(): string | null {
        return localStorage.getItem(LocalData.selectedProjectId);
    }

    public static setSelectedProjectId(id: string): void {
        localStorage.setItem(LocalData.selectedProjectId, id);
    }

    public static removeSelectedProjectId(): void {
        localStorage.removeItem(LocalData.selectedProjectId);
    }
}
