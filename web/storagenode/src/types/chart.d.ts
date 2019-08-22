// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare type Egress = {
    audit: number;
    repair: number;
    usage: number;
};

declare type Ingress = {
    repair: number;
    usage: number;
};

declare type FetchedBandwidthChartData = {
    from: string;
    to: string;
    egress: Egress;
    ingress: Ingress;
};

declare type FetchedStorageChartData = {
    atRestTotal: number;
    timestamp: string;
};
