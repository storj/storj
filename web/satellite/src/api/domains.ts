// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { DomainsAPI } from '@/types/domains';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

export class DomainsHttpAPI implements DomainsAPI {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/domains';

    public async checkDNSRecords(domain: string, cname: string, txt: string[]): Promise<void> {
        const path = `${this.ROOT_PATH}/check-dns?domain=${domain}`;
        const response = await this.client.post(path, JSON.stringify({ cname, txt }));

        if (!response.ok) {
            const result = await response.json();
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot check DNS records',
                requestID: response.headers.get('x-request-id'),
            });
        }
    }
}
