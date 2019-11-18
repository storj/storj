// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BandwidthInfo, Dashboard, DiskSpaceInfo, SatelliteInfo } from '@/storagenode/dashboard';
import {
    BandwidthUsed,
    Egress,
    EgressBandwidthUsed,
    Ingress, IngressBandwidthUsed,
    Metric,
    Satellite,
    Satellites,
    Stamp
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
        const json = (await (await httpGet('/api/dashboard')).json() as any).data;

        const satellitesJson = json.satellites ? json.satellites : [];

        const satellites: SatelliteInfo[] = satellitesJson.map((satellite: any) => {
            const disqualified: Date | null = satellite.disqualified ? new Date(satellite.disqualified) : null;

            return new SatelliteInfo(satellite.id, disqualified);
        });

        const diskSpace: DiskSpaceInfo = new DiskSpaceInfo(json.diskSpace.used, json.diskSpace.available);

        const bandwidth: BandwidthInfo = new BandwidthInfo(json.bandwidth.used, json.bandwidth.available);

        return new Dashboard(json.nodeID, json.wallet, satellites, diskSpace, bandwidth,
                                        new Date(json.lastPinged), new Date(json.startedAt), json.version, json.upToDate);
    }

    /**
     * parses satellite data from json
     * @returns satellite - new satellite instance filled with data from json
     */
    public async satellite(id: string): Promise<Satellite> {
        const url = '/api/satellite/' + id;

        const json = (await (await httpGet(url)).json() as any).data;

        const storageDailyJson = json.storageDaily ? json.storageDaily : [];
        const bandwidthDailyJson = json.bandwidthDaily ? json.bandwidthDaily : [];

        const storageDaily: Stamp[] = storageDailyJson.map((stamp: any) => {
            return new Stamp(stamp.atRestTotal, new Date(stamp.intervalStart));
        });

        const bandwidthDaily: BandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new BandwidthUsed(egress, ingress, new Date(bandwidth.intervalStart));
        });

        const egressBandwidthDaily: EgressBandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);

            return new EgressBandwidthUsed(egress, new Date(bandwidth.intervalStart));
        });

        const ingressBandwidthDaily: IngressBandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new IngressBandwidthUsed(ingress, new Date(bandwidth.intervalStart));
        });

        const audit: Metric = new Metric(json.audit.totalCount, json.audit.successCount, json.audit.alpha,
            json.audit.beta, json.audit.score);

        const uptime: Metric = new Metric(json.uptime.totalCount, json.uptime.successCount, json.uptime.alpha,
            json.uptime.beta, json.uptime.score);

        return new Satellite(json.id, storageDaily, bandwidthDaily, egressBandwidthDaily, ingressBandwidthDaily,
            json.storageSummary, json.bandwidthSummary, json.egressSummary, json.ingressSummary,
            audit, uptime);
    }

    /**
     * parses data for all satellites from json
     * @returns satellites - new satellites instance filled with data from json
     */
    public async satellites(): Promise<Satellites> {
        const json = (await (await httpGet('/api/satellites')).json() as any).data;

        const storageDailyJson = json.storageDaily ? json.storageDaily : [];
        const bandwidthDailyJson = json.bandwidthDaily ? json.bandwidthDaily : [];

        const storageDaily: Stamp[] = storageDailyJson.map((stamp: any) => {
            return new Stamp(stamp.atRestTotal, new Date(stamp.intervalStart));
        });

        const bandwidthDaily: BandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new BandwidthUsed(egress, ingress, new Date(bandwidth.intervalStart));
        });

        const egressBandwidthDaily: EgressBandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);

            return new EgressBandwidthUsed(egress, new Date(bandwidth.intervalStart));
        });

        const ingressBandwidthDaily: IngressBandwidthUsed[] =  bandwidthDailyJson.map((bandwidth: any) => {
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new IngressBandwidthUsed(ingress, new Date(bandwidth.intervalStart));
        });

        return new Satellites(storageDaily, bandwidthDaily, egressBandwidthDaily, ingressBandwidthDaily,
            json.storageSummary, json.bandwidthSummary, json.egressSummary, json.ingressSummary);
    }
}
