// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import VueSegmentAnalytics from 'vue-segment-analytics';
import { isDoNotTrackEnabled } from '@/utils/doNotTrack';

const Analytics = {
    install(Vue, options) {
        const isDoNotTrack = isDoNotTrackEnabled();

        if (isDoNotTrack && options.router != undefined) {
            delete options.router;
        }

        VueSegmentAnalytics.install(Vue, options);

        /* tslint:disable-next-line */
        if (isDoNotTrack) {
            Vue.$segment.methods.forEach(method => {
                Vue.$segment[method] = () => undefined;
            });
        }
    }
};

export default Analytics;
