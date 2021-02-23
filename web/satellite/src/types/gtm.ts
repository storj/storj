// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { LoadScript } from '@/utils/loadScript';
import { MetaUtils } from '@/utils/meta';

/**
 * GoogleTagManager is a conversion tracking tag utility.
 */
export class GoogleTagManager {
    private gtmScript: LoadScript;

    /**
     * Initializes google tag manager in DOM.
     */
    public init(): void {
        const gtmID: string = MetaUtils.getMetaContent('gtm-id');
        const url = `https://www.googletagmanager.com/gtm.js?id=${gtmID}`;

        this.gtmScript = new LoadScript(url,
            () => {
                window['dataLayer'].push({
                    event: 'gtm.js',
                    'gtm.start': new Date().getTime(),
                });
            },
            (error) => {
                throw error;
            },
        );
    }

    /**
     * Removes google tag manager from DOM.
     */
    public remove(): void {
        this.gtmScript.remove();
    }
}
