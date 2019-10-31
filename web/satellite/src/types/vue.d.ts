// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare module '*.vue' {
    import Vue from 'vue';
    export default Vue;
}

declare module 'vue/types/vue' {
    interface Vue {
        $segment: any; // define real typings here if you want
    }
}
