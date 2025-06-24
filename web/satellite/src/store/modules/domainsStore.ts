// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import { CheckDNSResponse, CreateDomainRequest, DomainsCursor, DomainsOrderBy, DomainsPage } from '@/types/domains';
import { useLinksharing } from '@/composables/useLinksharing';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DomainsHttpAPI } from '@/api/domains';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { SortDirection } from '@/types/common';

export class DomainsState {
    public cursor: DomainsCursor = new DomainsCursor();
    public page: DomainsPage = new DomainsPage();
    public allDomainNames: string[] = [];
}

export const useDomainsStore = defineStore('domains', () => {
    const state = reactive<DomainsState>(new DomainsState());

    const api = new DomainsHttpAPI();

    const projectsStore = useProjectsStore();
    const configStore = useConfigStore();

    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    async function checkDNSRecords(domain: string, cname: string, txt: string[]): Promise<CheckDNSResponse> {
        return await api.checkDNSRecords(domain, cname, txt);
    }

    async function generateDomainCredentials(accessName: string, bucket: string, passphrase: string): Promise<string> {
        const agStore = useAccessGrantsStore();

        const apiKey = await agStore.createAccessGrant(accessName, projectsStore.state.selectedProject.id);

        const { generatePublicCredentials } = useLinksharing();

        const creds = await generatePublicCredentials(apiKey.secret, bucket, null, passphrase);

        return creds.accessKeyId;
    }

    async function storeDomain(request: CreateDomainRequest): Promise<void> {
        await api.create(projectsStore.state.selectedProject.id, request, csrfToken.value);
    }

    async function fetchDomains(page: number, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
        state.cursor.page = page;
        state.cursor.limit = limit;

        state.page = await api.getPaged(projectsStore.state.selectedProject.id, state.cursor);
    }

    async function deleteDomain(name: string): Promise<void> {
        await api.delete(projectsStore.state.selectedProject.id, name, csrfToken.value);
    }

    async function getAllDomainNames(projectID: string): Promise<void> {
        state.allDomainNames = await api.getAllNames(projectID);
    }

    function setSearchQuery(query: string): void {
        state.cursor.search = query;
    }

    function setSortingBy(order: DomainsOrderBy): void {
        state.cursor.order = order;
    }

    function setSortingDirection(direction: SortDirection): void {
        state.cursor.orderDirection = direction;
    }

    function clear(): void {
        state.allDomainNames = [];
        state.page = new DomainsPage();
        state.cursor = new DomainsCursor();
    }

    return {
        state,
        checkDNSRecords,
        generateDomainCredentials,
        storeDomain,
        fetchDomains,
        setSearchQuery,
        setSortingBy,
        setSortingDirection,
        deleteDomain,
        getAllDomainNames,
        clear,
    };
});
