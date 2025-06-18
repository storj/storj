// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import { CheckDNSResponse, CreateDomainRequest, Domain } from '@/types/domains';
import { useLinksharing } from '@/composables/useLinksharing';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DomainsHttpAPI } from '@/api/domains';
import { useConfigStore } from '@/store/modules/configStore';

export class DomainsState {
    public domains: Domain[] = [];
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

        // TODO: rework when we have a way to store those.
        state.domains.push(new Domain(accessName, new Date()));

        return creds.accessKeyId;
    }

    async function storeDomain(request: CreateDomainRequest): Promise<void> {
        await api.create(projectsStore.state.selectedProject.id, request, csrfToken.value);
    }

    return {
        state,
        checkDNSRecords,
        generateDomainCredentials,
        storeDomain,
    };
});
