// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all AB testing related functionality.
 */
export interface ABTestApi {
    /**
     * Used to get all AB testing values.
     *
     * @throws Error
     */
    fetchABTestValues(): Promise<ABTestValues>;

    /**
     * Used to send an action event.
     *
     * @throws Error
     */
    sendHit(action: ABHitAction): Promise<void>;
}

/**
 * ABTestValues class holds all configurable AB test values.
 */
export class ABTestValues {
    public constructor(
        public hasNewUpgradeBanner: boolean = false,
    ) {}
}

// Enum of all AB test hit actions
export enum ABHitAction {
    UPGRADE_ACCOUNT_CLICKED= 'upgrade_clicked',
}
