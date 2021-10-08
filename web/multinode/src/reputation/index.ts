// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Stats encapsulates node reputation data.
 */
export class Stats {
    public constructor(
        public nodeId: string,
        public nodeName: string,
        public audit: Audit,
        public onlineScore: number,
        public disqualifiedAt: Date,
        public suspendedAt: Date,
        public offlineSuspendedAt: Date,
        public offlineUnderReviewAt: Date,
        public updatedAt: Date,
        public joinedAt: Date,
    ) {}
}

/**
 * Audit contains audit reputation metrics.
 */
export class Audit {
    public constructor(
        public totalCount: number,
        public successCount: number,
        public alpha: number,
        public beta: number,
        public unknownAlpha: number,
        public unknownBeta: number,
        public score: number,
        public suspensionScore: number,
        public history: AuditWindow[],
    ) {}
}

/**
 * AuditWindow contains audit count for particular time frame.
 */
export class AuditWindow {
    public constructor(
        public windowStart: Date,
        public totalCount: number,
        public onlineCount: number,
    ) {}
}
