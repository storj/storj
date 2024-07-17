// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { Domain } from '@/types/domains';
import { useLinksharing } from '@/composables/useLinksharing';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DomainsHttpAPI } from '@/api/domains';

export class DomainsState {
    public domains: Domain[] = [];
}

export const useDomainsStore = defineStore('domains', () => {
    const state = reactive<DomainsState>(new DomainsState());

    const api = new DomainsHttpAPI();

    async function checkDNSRecord(domain: string): Promise<void> {
        await api.checkDNSRecord(domain);
    }

    async function generateDomainCredentials(accessName: string, bucket: string, passphrase: string): Promise<string> {
        const agStore = useAccessGrantsStore();
        const projectsStore = useProjectsStore();

        accessName = `${accessName}-${new Date().getTime()}`;

        const apiKey = await agStore.createAccessGrant(accessName, projectsStore.state.selectedProject.id);

        const { generateCredentials } = useLinksharing();

        const creds = await generateCredentials(apiKey.secret, `sj://${bucket}`, null, passphrase);

        // TODO: rework when we have a way to store those.
        state.domains.push(new Domain(creds.accessKeyId, accessName, new Date()));

        return creds.accessKeyId;
    }

    return {
        state,
        checkDNSRecord,
        generateDomainCredentials,
    };
});
