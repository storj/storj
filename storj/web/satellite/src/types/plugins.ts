// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';

import { Notificator } from '@/utils/plugins/notificator';
import { Segmentio } from '@/utils/plugins/segment';

declare module 'vue/types/vue' {
    interface Vue {
        $segment: Segmentio; // define real typings here if you want
        $notify: Notificator;
    }
}
