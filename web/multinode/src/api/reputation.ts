// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { Audit, AuditWindow, Stats } from '@/reputation';

/**
 * ReputationClient is a reputation api client.
 */
export class ReputationClient extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/reputation';

    /**
     * stats handles retrieval of a node reputation for particular satellite.
     * @param satelliteId - id of satellite.
     */
    public async stats(satelliteId: string): Promise<Stats[]> {
        const path = `${this.ROOT_PATH}/satellites/${satelliteId}`;

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return result.map(
            (stats: Stats) => new Stats(
                stats.nodeId,
                stats.nodeName,
                new Audit(
                    stats.audit.totalCount,
                    stats.audit.successCount,
                    stats.audit.alpha,
                    stats.audit.beta,
                    stats.audit.unknownAlpha,
                    stats.audit.unknownBeta,
                    stats.audit.score,
                    stats.audit.suspensionScore,
                    stats.audit.history.map(
                        (auditWindow: AuditWindow) => new AuditWindow(
                            new Date(auditWindow.windowStart),
                            auditWindow.totalCount,
                            auditWindow.onlineCount,
                        ),
                    ),
                ),
                stats.onlineScore,
                new Date(stats.disqualifiedAt),
                new Date(stats.suspendedAt),
                new Date(stats.offlineSuspendedAt),
                new Date(stats.offlineUnderReviewAt),
                new Date(stats.updatedAt),
                new Date(stats.joinedAt),
            ),
        );
    }
}
