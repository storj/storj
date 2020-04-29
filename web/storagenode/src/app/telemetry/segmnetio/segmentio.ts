// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { Telemetry, TelemetryViews } from '@/app/telemetry/telemetry';
import { LoadScript } from '@/app/utils/loadScript';

/**
 * Segmentio implements all telemetry related functionality of Telemetry interface.
 */
export class Segmentio implements Telemetry {
    private analytics: SegmentAnalytics.AnalyticsJS;
    private loadScript: LoadScript;

    /**
     * Segment.io client.
     * @param key - public key to your source.
     */
    public init(key: string): void {
        if (key === '' || key.includes('SegmentIOPublicKey')) {
            return;
        }

        const url = `https://cdn.segment.com/analytics.js/v1/${key}/analytics.min.js`;

        this.loadScript = new LoadScript(url,
            () => {
                this.analytics = window['analytics'];
                this.view(TelemetryViews.Dashboard);
            },
            (error) => {
                throw error;
            },
        );
    }

    /**
     * The view method lets you record how many times view, component or page was viewed in your application.
     */
    public view(view: TelemetryViews): void {
        if (!this.analytics) {
            return;
        }

        this.analytics.page(view);
    }

    /**
     * Updates information about user with specified NodeId.
     */
    public identify(id: string): void {
        if (!this.analytics) {
            return;
        }

        this.analytics.identify(id);
    }
}
