// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { Telemetry } from '@/app/telemetry/telemetry';

declare module 'vue/types/vue' {
    interface Vue {
        $telemetry: Telemetry;
    }
}
