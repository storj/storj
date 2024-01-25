// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * LocalData exposes methods to manage local storage.
 */
export class LocalData {
    private static selectedProjectId = 'selectedProjectId';
    private static bucketWasCreated = 'bucketWasCreated';
    private static demoBucketCreated = 'demoBucketCreated';
    private static bucketGuideHidden = 'bucketGuideHidden';
    private static serverSideEncryptionBannerHidden = 'serverSideEncryptionBannerHidden';
    private static serverSideEncryptionModalHidden = 'serverSideEncryptionModalHidden';
    private static sessionExpirationDate = 'sessionExpirationDate';
    private static customSessionDuration = 'customSessionDuration';
    private static projectLimitBannerHidden = 'projectLimitBannerHidden';
    private static projectTableViewEnabled = 'projectTableViewEnabled';
    private static browserCardViewEnabled = 'browserCardViewEnabled';
    private static sessionHasExpired = 'sessionHasExpired';

    public static getSelectedProjectId(): string | null {
        return localStorage.getItem(LocalData.selectedProjectId);
    }

    public static setSelectedProjectId(id: string): void {
        localStorage.setItem(LocalData.selectedProjectId, id);
    }

    public static getDemoBucketCreatedStatus(): string | null {
        const status = localStorage.getItem(LocalData.demoBucketCreated);
        if (!status) return null;

        return JSON.parse(status);
    }

    public static setDemoBucketCreatedStatus(): void {
        localStorage.setItem(LocalData.demoBucketCreated, 'true');
    }

    public static setBucketWasCreatedStatus(): void {
        localStorage.setItem(LocalData.bucketWasCreated, 'true');
    }

    public static getBucketWasCreatedStatus(): boolean | null {
        const status = localStorage.getItem(LocalData.bucketWasCreated);
        if (!status) return null;

        return JSON.parse(status);
    }

    /**
     * "Disable" showing the upload guide tooltip on the bucket page
     */
    public static setBucketGuideHidden(): void {
        localStorage.setItem(LocalData.bucketGuideHidden, 'true');
    }

    public static getBucketGuideHidden(): boolean {
        const value = localStorage.getItem(LocalData.bucketGuideHidden);
        return value === 'true';
    }

    /**
     * "Disable" showing the server-side encryption banner on the bucket page
     */
    public static setServerSideEncryptionBannerHidden(value: boolean): void {
        localStorage.setItem(LocalData.serverSideEncryptionBannerHidden, String(value));
    }

    public static getServerSideEncryptionBannerHidden(): boolean {
        const value = localStorage.getItem(LocalData.serverSideEncryptionBannerHidden);
        return value === 'true';
    }

    /**
     * "Disable" showing the server-side encryption modal during S3 creation process.
     */
    public static setServerSideEncryptionModalHidden(value: boolean): void {
        localStorage.setItem(LocalData.serverSideEncryptionModalHidden, String(value));
    }

    public static getServerSideEncryptionModalHidden(): boolean {
        const value = localStorage.getItem(LocalData.serverSideEncryptionModalHidden);
        return value === 'true';
    }

    public static getSessionExpirationDate(): Date | null {
        const data: string | null = localStorage.getItem(LocalData.sessionExpirationDate);
        if (data) {
            return new Date(data);
        }

        return null;
    }

    public static setSessionExpirationDate(date: Date): void {
        localStorage.setItem(LocalData.sessionExpirationDate, date.toISOString());
    }

    public static getCustomSessionDuration(): number | null {
        const value: string | null = localStorage.getItem(LocalData.customSessionDuration);
        if (value) return parseInt(value);

        return null;
    }

    public static setCustomSessionDuration(value: number): void {
        localStorage.setItem(LocalData.customSessionDuration, value.toString());
    }

    public static removeCustomSessionDuration(): void {
        localStorage.removeItem(LocalData.customSessionDuration);
    }

    public static getSessionHasExpired(): boolean {
        const value: string | null = localStorage.getItem(LocalData.sessionHasExpired);
        return value === 'true';
    }

    public static setSessionHasExpired(): void {
        localStorage.setItem(LocalData.sessionHasExpired, 'true');
    }

    public static removeSessionHasExpired(): void {
        localStorage.removeItem(LocalData.sessionHasExpired);
    }

    /**
     * "Disable" showing the project limit banner.
     */
    public static setProjectLimitBannerHidden(): void {
        localStorage.setItem(LocalData.projectLimitBannerHidden, 'true');
    }

    public static getProjectTableViewEnabled(): boolean {
        const value = localStorage.getItem(LocalData.projectTableViewEnabled);
        return value === 'true';
    }

    public static setProjectTableViewEnabled(enabled: boolean): void {
        localStorage.setItem(LocalData.projectTableViewEnabled, enabled.toString());
    }

    /*
    * Whether the file browser should use the card view.
    */
    public static getBrowserCardViewEnabled(): boolean {
        const value = localStorage.getItem(LocalData.browserCardViewEnabled);
        return value === 'true';
    }

    /*
    * Set whether the file browser should use the card view.
    */
    public static setBrowserCardViewEnabled(enabled: boolean): void {
        localStorage.setItem(LocalData.browserCardViewEnabled, enabled.toString());
    }

    /*
    * Whether a user defined setting has been made for the projects table
    * */
    public static hasProjectTableViewConfigured(): boolean {
        return localStorage.getItem(LocalData.projectTableViewEnabled) !== null;
    }

    /*
    * Remove the user defined setting for the projects table;
    * */
    public static removeProjectTableViewConfig() {
        return localStorage.removeItem(LocalData.projectTableViewEnabled);
    }
}
