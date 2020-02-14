// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import loadScript from 'load-script';

import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

/**
 * Segmentio is a wrapper around segment.io analytics package.
 */
export class Segmentio {
    private analytics: SegmentAnalytics.AnalyticsJS;
    public init(key: string) {
        if (this.analytics || key.length === 0 || key.includes('SegmentIOPublicKey')) {
            return;
        }
        const source = `https://cdn.segment.com/analytics.js/v1/${key}/analytics.min.js`;
        loadScript(source, (error) => {
          if (error) {
            return;
          }

          this.analytics = window['analytics'];
        });

        this.page();
    }

    public page() {
        if (!this.analytics) {
            return;
        }

        this.analytics.page();
    }

    public identify(userId: string, traits?: Object, options?: SegmentAnalytics.SegmentOpts, callback?: () => void) {
        if (!this.analytics) {
            return;
        }

        this.analytics.identify(userId, traits, options, callback);
    }

    public track(event: SegmentEvent, properties?: Object, options?: SegmentAnalytics.SegmentOpts, callback?: () => void) {
        if (!this.analytics) {
            return;
        }

        this.analytics.track(event, properties, options, callback);
    }
}

export class SegmentioPlugin {
    public install(Vue) {
        Vue.prototype.$segment = new Segmentio();
    }
}
