// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    Dashboard, DiskSpace,
    Satellite,
    SatelliteByDayInfo,
    SatelliteInfo,
    Satellites,
    SatelliteScores,
    Traffic,
} from '@/storagenode/sno/sno';
import { HttpClient } from '@/storagenode/utils/httpClient';

/**
 * Used to get dashboard and satellite data from json.
 */
export class StorageNodeApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/sno';

    /**
     * Gets dashboard data from server.
     * @returns dashboard - new dashboard instance filled with data from json.
     */
    public async dashboard(): Promise<Dashboard> {
        const response = await this.client.get(this.ROOT_PATH);

        if (!response.ok) {
            throw new Error('can not get node information');
        }

        const data = await response.json();

        const satellitesJson = data.satellites || [];

        const satellites: SatelliteInfo[] = satellitesJson.map((satellite: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            const disqualified: Date | null = satellite.disqualified ? new Date(satellite.disqualified) : null;
            const suspended: Date | null = satellite.suspended ? new Date(satellite.suspended) : null;
            const vettedAt: Date | null = satellite.vettedAt ? new Date(satellite.vettedAt) : null;

            return new SatelliteInfo(satellite.id, satellite.url, disqualified, suspended, vettedAt);
        });

        const diskSpace: DiskSpace = new DiskSpace(data.diskSpace.used, data.diskSpace.allocated, data.diskSpace.trash, data.diskSpace.overused, data.diskSpace.reclaimable, data.diskSpace.reserved);
        const bandwidth: Traffic = new Traffic(data.bandwidth.used);

        return new Dashboard(data.nodeID, data.wallet, data.walletFeatures || [], satellites, diskSpace, bandwidth,
            new Date(data.lastPinged), new Date(data.startedAt), data.version, data.allowedVersion, data.upToDate, data.quicStatus, data.configuredPort, new Date(data.lastQuicPingedAt));
    }

    /**
     * Gets satellite data from server.
     * @returns satellite - new satellite instance filled with data from json.
     */
    public async satellite(id: string): Promise<Satellite> {
        const url = `${this.ROOT_PATH}/satellite/${id}`;

        const response = await this.client.get(url);

        if (!response.ok) {
            throw new Error('can not get satellite information');
        }

        const data = await response.json();

        const satelliteByDayInfo = new SatelliteByDayInfo(data);

        const audits: SatelliteScores = new SatelliteScores(
            data.audits.satelliteName,
            data.audits.auditScore,
            data.audits.suspensionScore,
            data.audits.onlineScore,
        );

        return new Satellite(
            data.id,
            satelliteByDayInfo.storageDaily,
            satelliteByDayInfo.bandwidthDaily,
            satelliteByDayInfo.egressDaily,
            satelliteByDayInfo.ingressDaily,
            data.storageSummary,
            data.averageUsageBytes,
            data.bandwidthSummary,
            data.egressSummary,
            data.ingressSummary,
            audits,
            new Date(data.nodeJoinedAt),
        );
    }

    /**
     * Gets data for all satellites from server.
     * @returns satellites - new satellites instance filled with data from json.
     */
    public async satellites(): Promise<Satellites> {
        const url = `${this.ROOT_PATH}/satellites`;

        const response = await this.client.get(url);

        if (!response.ok) {
            throw new Error('can not get all satellites information');
        }

        const data = await response.json();

        const satelliteByDayInfo = new SatelliteByDayInfo(data);

        const satellitesScores = data.audits.map(scoreInfo => {
            return new SatelliteScores(
                scoreInfo.satelliteName,
                scoreInfo.auditScore,
                scoreInfo.suspensionScore,
                scoreInfo.onlineScore,
            );
        });

        return new Satellites(
            satelliteByDayInfo.storageDaily,
            satelliteByDayInfo.bandwidthDaily,
            satelliteByDayInfo.egressDaily,
            satelliteByDayInfo.ingressDaily,
            data.storageSummary,
            data.averageUsageBytes,
            data.bandwidthSummary,
            data.egressSummary,
            data.ingressSummary,
            new Date(data.earliestJoinedAt),
            satellitesScores,
        );
    }
}
