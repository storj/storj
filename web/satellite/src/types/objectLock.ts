// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { ObjectLockLegalHoldStatus, ObjectLockMode } from '@aws-sdk/client-s3';

export const NO_MODE_SET = 'Not Set';
export const COMPLIANCE_LOCK = ObjectLockMode.COMPLIANCE;
export const GOVERNANCE_LOCK = ObjectLockMode.GOVERNANCE;
export type ObjLockMode = typeof GOVERNANCE_LOCK | typeof COMPLIANCE_LOCK;

export function capitalizedMode(mode: ObjLockMode): string {
    return mode.charAt(0).toUpperCase() + mode.slice(1).toLowerCase();
}

export const LEGAL_HOLD_ON = ObjectLockLegalHoldStatus.ON;
export const LEGAL_HOLD_OFF = ObjectLockLegalHoldStatus.OFF;

export enum DefaultObjectLockPeriodUnit {
    DAYS = 'Days',
    YEARS = 'Years',
}

export class Retention {
    mode: ObjectLockMode | '';
    retainUntil: Date;

    constructor(mode: ObjectLockMode | '', retainUntil: Date) {
        this.mode = mode;
        this.retainUntil = retainUntil;
    }

    public static empty(): Retention {
        return new Retention('', new Date());
    }

    // returns whether the retention configuration is enabled.
    public get enabled(): boolean {
        return this.mode === ObjectLockMode.COMPLIANCE || this.mode === ObjectLockMode.GOVERNANCE;
    }

    // returns whether the retention configuration is enabled
    // and active as of the current time.
    public get active(): boolean {
        return this.enabled && new Date() < this.retainUntil;
    }
}

export interface ObjectLockStatus {
    retention: Retention;
    legalHold: boolean;
}