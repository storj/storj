// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BandwidthInfo, Dashboard, DiskSpaceInfo, SatelliteInfo } from '@/storagenode/dashboard';
import {
    BandwidthUsed,
    Egress,
    EgressUsed,
    Ingress,
    IngressUsed,
    Metric,
    Satellite,
    Satellites,
    Stamp,
} from '@/storagenode/satellite';

/**
 * Implementation for HTTP GET requests
 * @param url - holds url of request target
 * @throws Error - holds error message if request wasn't successful
 */
async function httpGet(url): Promise<Response> {
    const response = await fetch(url);

    if (response.ok) {
        return response;
    }

    throw new Error(response.statusText);
}

/**
 * used to get dashboard and satellite data from json
 */
export class SNOApi {
    /**
     * parses dashboard data from json
     * @returns dashboard - new dashboard instance filled with data from json
     */
    public async dashboard(): Promise<Dashboard> {
        const json = await (await httpGet('/api/sno')).json();

        const satellitesJson = json.satellites || [];

        const satellites: SatelliteInfo[] = satellitesJson.map((satellite: any) => {
            const disqualified: Date | null = satellite.disqualified ? new Date(satellite.disqualified) : null;
            const suspended: Date | null = satellite.suspended ? new Date(satellite.suspended) : null;

            return new SatelliteInfo(satellite.id, satellite.url, disqualified, suspended);
        });

        const diskSpace: DiskSpaceInfo = new DiskSpaceInfo(json.diskSpace.used, json.diskSpace.available);
        const bandwidth: BandwidthInfo = new BandwidthInfo(json.bandwidth.used);

        return new Dashboard(json.nodeID, json.wallet, satellites, diskSpace, bandwidth,
            new Date(json.lastPinged), new Date(json.startedAt), json.version, json.allowedVersion, json.upToDate);
    }

    /**
     * parses satellite data from json
     * @returns satellite - new satellite instance filled with data from json
     */
    public async satellite(id: string): Promise<Satellite> {
        const url = `/api/sno/satellite/${id}`;

        const json = await (await httpGet(url)).json();

        const satelliteByDayInfo = new SatelliteByDayInfo(json);

        const audit: Metric = new Metric(json.audit.totalCount, json.audit.successCount, json.audit.alpha,
            json.audit.beta, json.audit.score);

        const uptime: Metric = new Metric(json.uptime.totalCount, json.uptime.successCount, json.uptime.alpha,
            json.uptime.beta, json.uptime.score);

        return new Satellite(
            json.id,
            satelliteByDayInfo.storageDaily,
            satelliteByDayInfo.bandwidthDaily,
            satelliteByDayInfo.egressDaily,
            satelliteByDayInfo.ingressDaily,
            json.storageSummary,
            json.bandwidthSummary,
            json.egressSummary,
            json.ingressSummary,
            audit,
            uptime,
            new Date(json.nodeJoinedAt),
        );
    }

    /**
     * parses data for all satellites from json
     * @returns satellites - new satellites instance filled with data from json
     */
    public async satellites(): Promise<Satellites> {
        const json = await (await httpGet('/api/sno/satellites')).json();

        const satelliteByDayInfo = new SatelliteByDayInfo(json);

        return new Satellites(
            satelliteByDayInfo.storageDaily,
            satelliteByDayInfo.bandwidthDaily,
            satelliteByDayInfo.egressDaily,
            satelliteByDayInfo.ingressDaily,
            json.storageSummary,
            json.bandwidthSummary,
            json.egressSummary,
            json.ingressSummary,
            new Date(json.earliestJoinedAt),
        );
    }
}

/**
 * SatelliteByDayInfo holds by day bandwidth metrics.
 */
class SatelliteByDayInfo {
    public storageDaily: Stamp[];
    public bandwidthDaily: BandwidthUsed[];
    public egressDaily: EgressUsed[];
    public ingressDaily: IngressUsed[];

    public constructor(json) {
        const storageDailyJson = json.storageDaily || [];
        const bandwidthDailyJson = json.bandwidthDaily || [];

        this.storageDaily = storageDailyJson.map((stamp: any) => {
            return new Stamp(stamp.atRestTotal, new Date(stamp.intervalStart));
        });

        this.bandwidthDaily = bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new BandwidthUsed(egress, ingress, new Date(bandwidth.intervalStart));
        });

        this.egressDaily = bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);

            return new EgressUsed(egress, new Date(bandwidth.intervalStart));
        });

        this.ingressDaily = bandwidthDailyJson.map((bandwidth: any) => {
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new IngressUsed(ingress, new Date(bandwidth.intervalStart));
        });
    }
}
