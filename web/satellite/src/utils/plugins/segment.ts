// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import loadScript from 'load-script';

export class Segmentio {
    analytics: any;
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

    public identify() {
        if (!this.analytics) {
            return;
        }

        this.analytics.identify();
    }

    public track(eventName: string, attributes: object) {
        if (!this.analytics) {
            return;
        }

        this.analytics.track(eventName, attributes);
    }
}

export class SegmentioPlugin {
    public install(Vue) {
        Vue.prototype.$segment = new Segmentio();
    }
}
