// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

export class Download {
    static fileByLink(link: string): void {
        const a = document.createElement('a');
        a.href = link;
        a.download = '';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    }
}
