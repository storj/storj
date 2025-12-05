// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/// <reference types="vite/client" />

declare module '*.vue' {
    import type { DefineComponent } from 'vue';
    // eslint-disable-next-line
    const component: DefineComponent<{}, {}, any>;
    export default component;
}
