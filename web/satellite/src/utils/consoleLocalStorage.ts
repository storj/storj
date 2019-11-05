// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const USER_ID: string = 'userID';
const SELECTED_PROJECT_ID: string = 'selectedProjectId';

export function setUserId(userId: string): void {
    localStorage.setItem(USER_ID, userId);
}

export function getUserId(): string | null {
    return localStorage.getItem(USER_ID);
}

export function setSelectedProjectId(projectId: string): void {
    localStorage.setItem(SELECTED_PROJECT_ID, projectId);
}

export function getSelectedProjectId(): string | null {
    return localStorage.getItem(SELECTED_PROJECT_ID);
}

export function removeSelectedProjectId(): void {
    localStorage.removeItem(SELECTED_PROJECT_ID);
}
