// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { Segmentio } from '@/app/telemetry/segmnetio/segmentio';

/**
 * Telemetry exposes all telemetry related functionality.
 */
export interface Telemetry {
    /**
     * Init method initialize all dependencies and parameters.
     */
    init(key: string);
    /**
     * The view method lets you record how many times view, component or page was viewed in your application.
     */
    view(view: TelemetryViews);

    /**
     * Updates information about user with specified NodeId.
     */
    identify(nodeId: string);
}

/**
 * TelemetryPlugin is a Vue Plugin that allows to extend vue context with Telemetry functionality.
 */
export class TelemetryPlugin {
    public install(Vue) {
        Vue.prototype.$telemetry = new Segmentio();
    }
}

export enum TelemetryViews {
    Dashboard = 'SNO Dashboard Web App',
    MainPage = 'Main Page',
    Payments = 'Payments Page',
    Notifications = 'Notifications Page',
}
