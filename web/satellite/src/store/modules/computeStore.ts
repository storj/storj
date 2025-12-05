// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    CreateInstanceRequest,
    CreateSSHKeyRequest,
    IComputeAPI,
    Instance,
    SSHKey,
} from '@/types/compute';
import { ComputeAPI } from '@/api/compute';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

export class ComputeState {
    public sshKeys: SSHKey[] = [];
    public instances: Instance[] = [];
    public availableInstanceTypes: string[] = [];
    public availableImages: string[] = [];
    public availableLocations: string[] = [];
}

export const useComputeStore = defineStore('compute', () => {
    const state = reactive<ComputeState>(new ComputeState());

    const api: IComputeAPI = new ComputeAPI();

    const configStore = useConfigStore();
    const computeGatewayURL = computed<string>(() => configStore.state.config.computeGatewayURL);
    const projectsStore = useProjectsStore();
    const computeAuthToken = computed<string>(() => projectsStore.state.selectedProjectConfig.computeAuthToken);

    async function createSSHKey(req: CreateSSHKeyRequest): Promise<SSHKey> {
        const key = await api.createSSHKey(computeGatewayURL.value, computeAuthToken.value, req);

        state.sshKeys.push(key);

        return key;
    }

    async function getSSHKeys(): Promise<SSHKey[]> {
        const keys = await api.getSSHKeys(computeGatewayURL.value, computeAuthToken.value);

        state.sshKeys = keys;

        return keys;
    }

    async function deleteSSHKey(id: string): Promise<void> {
        await api.deleteSSHKey(computeGatewayURL.value, computeAuthToken.value, id);

        state.sshKeys = state.sshKeys.filter(key => key.id !== id);
    }

    async function createInstance(req: CreateInstanceRequest): Promise<Instance> {
        const instance = await api.createInstance(computeGatewayURL.value, computeAuthToken.value, req);

        state.instances.push(instance);

        return instance;
    }

    async function getInstances(): Promise<Instance[]> {
        const instances = await api.getInstances(computeGatewayURL.value, computeAuthToken.value);

        state.instances = instances;

        return instances;
    }

    async function getInstance(id: string): Promise<Instance> {
        const instance = await api.getInstance(computeGatewayURL.value, computeAuthToken.value, id);

        const index = state.instances.findIndex(i => i.id === id);
        if (index !== -1) {
            state.instances[index] = instance;
        } else {
            state.instances.push(instance);
        }

        return instance;
    }

    async function updateInstanceType(id: string, instanceType: string): Promise<Instance> {
        const instance = await api.updateInstanceType(computeGatewayURL.value, computeAuthToken.value, id, instanceType);

        const index = state.instances.findIndex(i => i.id === id);
        if (index !== -1) {
            state.instances[index] = instance;
        } else {
            state.instances.push(instance);
        }

        return instance;
    }

    async function deleteInstance(id: string): Promise<void> {
        await api.deleteInstance(computeGatewayURL.value, computeAuthToken.value, id);

        state.instances = state.instances.filter(i => i.id !== id);
    }

    async function getAvailableInstanceTypes(): Promise<string[]> {
        const types = await api.getAvailableInstanceTypes(computeGatewayURL.value, computeAuthToken.value);

        state.availableInstanceTypes = types;

        return types;
    }

    async function getAvailableImages(): Promise<string[]> {
        const images = await api.getAvailableImages(computeGatewayURL.value, computeAuthToken.value);

        state.availableImages = images;

        return images;
    }

    async function getAvailableLocations(): Promise<string[]> {
        const locations = await api.getAvailableLocations(computeGatewayURL.value, computeAuthToken.value);

        state.availableLocations = locations;

        return locations;
    }

    async function clear() {
        state.sshKeys = [];
        state.instances = [];
        state.availableInstanceTypes = [];
        state.availableImages = [];
        state.availableLocations = [];
    }

    return {
        state,
        createSSHKey,
        getSSHKeys,
        deleteSSHKey,
        createInstance,
        getInstances,
        getInstance,
        updateInstanceType,
        deleteInstance,
        getAvailableInstanceTypes,
        getAvailableImages,
        getAvailableLocations,
        clear,
    };
});
