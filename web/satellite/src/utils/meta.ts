// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * MetaUtils extracting content from meta.
 */
export class MetaUtils {
    public static getMetaContent(metaName: string): string {
        const meta = document.querySelector(`meta[name='${metaName}']`);

        if (meta) {
            return  meta.getAttribute('content') as string;
        }

        return '';
    }
}
