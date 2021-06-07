// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Represents by day and total bandwidth.
 */
export class BandwidthTraffic {
    public constructor(
        public bandwidthDaily: BandwidthRollup[] = [],
        public bandwidthSummary: number = 0,
        public egressSummary: number = 0,
        public ingressSummary: number = 0,
    ) {}
}

/**
 * Represents by day bandwidth.
 */
export class BandwidthRollup {
    public constructor(
        public egress: Egress = new Egress(),
        public ingress: Ingress = new Ingress(),
        public deletes: number = 0,
        public intervalStart: Date = new Date(),
    ) {}
}

/**
 * Stores info about storage node egress usage.
 */
export class Egress {
    public constructor(
        public repair: number = 0,
        public audit: number = 0,
        public usage: number = 0,
    ) {}
}

/**
 * Stores info about storage node ingress usage.
 */
export class Ingress {
    public constructor(
        public repair: number = 0,
        public usage: number = 0,
    ) {}
}
