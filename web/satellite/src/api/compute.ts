// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import {
    CreateInstanceRequest,
    CreateSSHKeyRequest,
    IComputeAPI,
    Instance,
    SSHKey,
} from '@/types/compute';
import { APIError } from '@/utils/error';

export class ComputeAPI implements IComputeAPI {
    private readonly http: HttpClient = new HttpClient();

    public async createSSHKey(baseURL: string, authToken: string, request: CreateSSHKeyRequest): Promise<SSHKey> {
        const path = `${baseURL}/api/v1/ssh-key`;
        const response = await this.http.post(path, JSON.stringify(request), { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not create SSH Key',
                requestID: result.requestID,
            });
        }

        return new SSHKey(result.id, result.name, result.publicKey, new Date(result.created));
    }

    public async getSSHKeys(baseURL: string, authToken: string): Promise<SSHKey[]> {
        const path = `${baseURL}/api/v1/ssh-key`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get SSH Keys',
                requestID: result.requestID,
            });
        }

        return (result ?? []).map(key => new SSHKey(
            key.id,
            key.name,
            key.publicKey,
            new Date(key.created),
        ));
    }

    public async deleteSSHKey(baseURL: string, authToken: string, id: string): Promise<void> {
        const path = `${baseURL}/api/v1/ssh-key/${id}`;
        const response = await this.http.delete(path, null, { authToken });

        if (response.status !== 204) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.message || 'Can not delete SSH Key',
                requestID: result.requestID,
            });
        }
    }

    public async createInstance(baseURL: string, authToken: string, request: CreateInstanceRequest): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance`;
        const response = await this.http.post(path, JSON.stringify(request), { authToken });
        const result = await response.json();

        if (response.status !== 201) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not create Instance',
                requestID: result.requestID,
            });
        }

        return this.instanceFromJSON(result);
    }

    public async getInstance(baseURL: string, authToken: string, id: string): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get Instance',
                requestID: result.requestID,
            });
        }

        return this.instanceFromJSON(result);
    }

    public async getInstances(baseURL: string, authToken: string): Promise<Instance[]> {
        const path = `${baseURL}/api/v1/instance`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get Instances',
                requestID: result.requestID,
            });
        }

        return (result ?? []).map((instance: Record<string, never>) => this.instanceFromJSON(instance));
    }

    public async updateInstanceType(baseURL: string, authToken: string, id: string, instanceType: string): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.patch(path, JSON.stringify({ instanceType }), { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not update Instance type',
                requestID: result.requestID,
            });
        }

        return this.instanceFromJSON(result);
    }

    public async deleteInstance(baseURL: string, authToken: string, id: string): Promise<void> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.delete(path, null, { authToken });

        if (response.status !== 204) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.message || 'Can not delete Instance',
                requestID: result.requestID,
            });
        }
    }

    public async getAvailableInstanceTypes(baseURL: string, authToken: string): Promise<string[]> {
        const path = `${baseURL}/api/v1/instance-type`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get instance types',
                requestID: result.requestID,
            });
        }

        return (result ?? []).map((type => type.name));
    }

    public async getAvailableImages(baseURL: string, authToken: string): Promise<string[]> {
        const path = `${baseURL}/api/v1/image`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get images',
                requestID: result.requestID,
            });
        }

        return (result ?? []).map((image => image.name));
    }

    public async getAvailableLocations(baseURL: string, authToken: string): Promise<string[]> {
        const path = `${baseURL}/api/v1/location`;
        const response = await this.http.get(path, { authToken });
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not get locations',
                requestID: result.requestID,
            });
        }

        return (result ?? []).map((loc => loc.name));
    }

    private instanceFromJSON(instance: Record<string, never>): Instance {
        return new Instance(
            instance.id,
            instance.name,
            instance.status,
            instance.hostname,
            instance.ipv4Address,
            instance.created ? new Date(instance.created) : new Date(),
            instance.updated ? new Date(instance.updated) : new Date(),
            instance.remote ?? { type: '', ipv4Address: '', port: 0 },
            instance.password ?? '',
            instance.deleting ?? false,
        );
    }
}
