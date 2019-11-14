// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import VueSegmentAnalytics from 'vue-segment-analytics';

import { isDoNotTrackEnabled } from '@/utils/doNotTrack';

export class AnalyticsPlugin {
    public install(Vue, options) {
        const isDoNotTrack = isDoNotTrackEnabled();
        const hasSegmentID = options.id && options.id.length > 0;

        if (!hasSegmentID) {
            options.id = 'fake id';
        }

        if ((isDoNotTrack || !hasSegmentID) && options.router !== undefined) {
            delete options.router;
        }

        VueSegmentAnalytics.install(Vue, options);

        // tslint:disable-next-line
        if (isDoNotTrack || !hasSegmentID) {
            Vue.$segment.methods.forEach(method => {
                Vue.$segment[method] = () => undefined;
            });
        }
    }
}
