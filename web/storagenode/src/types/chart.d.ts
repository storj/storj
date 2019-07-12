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

declare interface BandwidthChartData {
    From: string;
    To: string;
    egress: Egress;
    ingress: Ingress;
    summary: number;

    getLabels(): any;
}

declare type FetchedBandwidthChartData = {
    From: string;
    To: string;
    egress: Egress;
    ingress: Ingress;
};
