// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    CheckDNSResponse,
    CreateDomainRequest,
    Domain,
    DomainsAPI,
    DomainsCursor,
    DomainsPage,
} from '@/types/domains';
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

    public async create(projectID: string, request: CreateDomainRequest): Promise<void> {
        const path = `${this.ROOT_PATH}/project/${projectID}`;
        const response = await this.client.post(path, JSON.stringify(request));

        if (!response.ok) {
            const result = await response.json();
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot create domain',
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    public async delete(projectID: string, subdomain: string): Promise<void> {
        const path = `${this.ROOT_PATH}/project/${projectID}`;
        const response = await this.client.delete(path, subdomain);

        if (!response.ok) {
            const result = await response.json();
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot delete domain',
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    public async getPaged(projectID: string, cursor: DomainsCursor): Promise<DomainsPage> {
        const path = `${this.ROOT_PATH}/project/${projectID}/paged?search=${cursor.search}&limit=${cursor.limit}&page=${cursor.page}&order=${cursor.order}&orderDirection=${cursor.orderDirection}`;
        const response = await this.client.get(path);

        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot get domains',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return this.getDomainsPage(result);
    }

    private getDomainsPage(page: any): DomainsPage { // eslint-disable-line @typescript-eslint/no-explicit-any
        if (!(page && page.domains)) {
            return new DomainsPage();
        }

        const domainsPage: DomainsPage = new DomainsPage();

        domainsPage.domains = page.domains.map(d => new Domain(
            d.name,
            new Date(d.createdAt),
        ));

        domainsPage.search = page.search;
        domainsPage.limit = page.limit;
        domainsPage.order = page.order;
        domainsPage.orderDirection = page.orderDirection;
        domainsPage.pageCount = page.pageCount;
        domainsPage.currentPage = page.currentPage;
        domainsPage.totalCount = page.totalCount;

        return domainsPage;
    }
}
