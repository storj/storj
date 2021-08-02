// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';

declare global {
    namespace JSX {
        // tslint:disable no-empty-interface
        type Element = VNode

        // tslint:disable no-empty-interface
        type ElementClass = Vue

        interface IntrinsicElements {
            [elem: string]: any;
        }
    }
}
