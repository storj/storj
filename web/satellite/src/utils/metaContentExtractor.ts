// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class MetaRepository {
    public static getMetaContent(metaName: string) {
        const meta = document.querySelector(`meta[name='${metaName}']`);

        if (meta) {
            return  meta.getAttribute('content');
        }

        return '';
    }
}
