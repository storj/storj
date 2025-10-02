// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { CreateSSHKeyRequest, IComputeAPI, SSHKey } from '@/types/compute';
import { ComputeAPI } from '@/api/compute';
import { useConfigStore } from '@/store/modules/configStore';

export class ComputeState {
    public sshKeys: SSHKey[] = [];
}

export const useComputeStore = defineStore('compute', () => {
    const state = reactive<ComputeState>(new ComputeState());

    const api: IComputeAPI = new ComputeAPI();

    const configStore = useConfigStore();

    async function createSSHKey(req: CreateSSHKeyRequest): Promise<SSHKey> {
        const key = await api.createSSHKey(configStore.state.config.computeGatewayURL, req);

        state.sshKeys.push(key);

        return key;
    }

    async function getSSHKeys(): Promise<SSHKey[]> {
        const keys = await api.getSSHKeys(configStore.state.config.computeGatewayURL);

        state.sshKeys = keys;

        return keys;
    }

    async function deleteSSHKey(id: string): Promise<void> {
        await api.deleteSSHKey(configStore.state.config.computeGatewayURL, id);

        state.sshKeys = state.sshKeys.filter(key => key.id !== id);
    }

    return {
        state,
        createSSHKey,
        getSSHKeys,
        deleteSSHKey,
    };
});
