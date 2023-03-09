// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare module '*.svg' {
    import Vue, { VueConstructor } from 'vue';
    const content: VueConstructor<Vue>;
    export default content;
}
