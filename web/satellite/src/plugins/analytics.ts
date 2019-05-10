// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import _Vue from 'vue';

export function AnalyticsPlugin(Vue: typeof _Vue, options?: any): void {
    Vue.prototype.analytics = {
        track,
        page,
        identity,
    };
}

function track(event: string, options?: object): void {
    if (isDoNotTrack()) {
        return;
    }

    (<any>window).analytics.track(event, options);
}

function page(name: string, options?: object): void {
    if (isDoNotTrack()) {
        return;
    }

    (<any>window).analytics.page(name, options); 
}

function identity(userId: string, options?: object): void {
    if (isDoNotTrack()) {
        return;
    }

    (<any>window).analytics.identity(userId, options);
}

function isDoNotTrack() {
    switch (window.navigator.doNotTrack) {
        case 'no':
            return true;
        default:
            return false;
    }
}
