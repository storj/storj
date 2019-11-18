// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class Segmentio {
    public page(eventName: string) {
        const segmentio = window['analytics'];
        if (!segmentio) {
            return;
        }

        segmentio.page(eventName);
    }

    public identify() {
        const segmentio = window['analytics'];
        if (!segmentio) {
            return;
        }

        segmentio.identify();
    }

    public track(eventName: string, attributes: object) {
        const segmentio = window['analytics'];
        if (!segmentio) {
            return;
        }

        segmentio.track(eventName, attributes);
    }
}

export class SegmentioPlugin {
    public install(Vue) {
        Vue.prototype.$segment = new Segmentio();
    }
}
