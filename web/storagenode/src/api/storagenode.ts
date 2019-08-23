// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 *
 * @param url
 * @throws error
 */
async function httpGet(url): Promise<any> {
    let response = await fetch(url);

    if (response.ok) {
        return response.json();
    }

    throw new Error(response.statusText);
}

// Dashboard encapsulates dashboard stale data.
class Dashboard {
    public nodeID: string;
    public wallet: string;

    public satellites: SatelliteInfo[];

    public diskSpace: DiskSpaceInfo;
    public bandwidth: BandwidthInfo;

    public lastPinged: Date;
    public lastQueried: Date;

    public version: Version;
    public isUpToDate: boolean;
}

// SemVer represents a semantic version
class Version {
    public Major: number;
    public Minor: number;
    public Patch: number;
}

// SatelliteInfo encapsulates satellite ID and disqualification
class SatelliteInfo {
    public id: string;
    public disqualified: Date;
}

// DiskSpaceInfo stores all info about storagenode disk space usage
class DiskSpaceInfo {
    public used: number;
    public available: number;
}


// Egress stores info about storage node egress usage
class Egress {
    public Repair: number;
    public Audit: number;
    public Usage: number;
}

// Ingress stores info about storage node ingress usage
class Ingress {
    public repair: number;
    public usage: number;
}

// BandwidthInfo stores all info about storage node bandwidth usage
class BandwidthInfo {
    public used: number;
    public available: number;
}

// BandwidthUsed stores bandwidth usage information over the period of time
class BandwidthUsed {
    public egress: Egress;
    public ingress: Ingress;

    public from: Date;
    public to: Date;
}


/**
 * Implementation for HTTP GET requests
 * @param {string} url=
 */
export class SNOApi {

    /**
     *
     */
    public async dashboard(): any {
        const response = httpGet('/api/dashboard');
    }
}
