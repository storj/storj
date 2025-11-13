// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare module '*.vue' {
    import type { DefineComponent } from 'vue';
    // eslint-disable-next-line
    const component: DefineComponent<any, any, any>;
    export default component;
}
