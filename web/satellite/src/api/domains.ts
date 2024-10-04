// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { CheckDNSResponse, DomainsAPI } from '@/types/domains';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

export class DomainsHttpAPI implements DomainsAPI {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/domains';

    public async checkDNSRecords(domain: string, cname: string, txt: string[]): Promise<CheckDNSResponse> {
        const path = `${this.ROOT_PATH}/check-dns?domain=${domain}`;
        const response = await this.client.post(path, JSON.stringify({ cname, txt }));
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot check DNS records',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return {
            isSuccess: result.isSuccess,
            isVerifyError: result.isVerifyError,
            expectedCNAME: result.expectedCNAME,
            expectedTXT: result.expectedTXT ?? [],
            gotCNAME: result.gotCNAME,
            gotTXT: result.gotTXT ?? [],
        };
    }
}
