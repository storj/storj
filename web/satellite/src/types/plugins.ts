// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';

import { Notificator } from '@/utils/plugins/notificator';

declare module 'vue/types/vue' {
    interface Vue {
        $segment: any; // define real typings here if you want$nf
        $notify: Notificator;
    }
}
