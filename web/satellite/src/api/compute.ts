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

    public async createSSHKey(baseURL: string, request: CreateSSHKeyRequest): Promise<SSHKey> {
        const path = `${baseURL}/api/v1/ssh-key`;
        const response = await this.http.post(path, JSON.stringify(request));
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

    public async getSSHKeys(baseURL: string): Promise<SSHKey[]> {
        const path = `${baseURL}/api/v1/ssh-key`;
        const response = await this.http.get(path);
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

    public async deleteSSHKey(baseURL: string, id: string): Promise<void> {
        const path = `${baseURL}/api/v1/ssh-key/${id}`;
        const response = await this.http.delete(path);
        const result = await response.json();

        if (response.status !== 204) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not delete SSH Key',
                requestID: result.requestID,
            });
        }
    }

    public async createInstance(baseURL: string, request: CreateInstanceRequest): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance`;
        const response = await this.http.post(path, JSON.stringify(request));
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

    public async getInstance(baseURL: string, id: string): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.get(path);
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

    public async getInstances(baseURL: string): Promise<Instance[]> {
        const path = `${baseURL}/api/v1/instance`;
        const response = await this.http.get(path);
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

    public async updateInstanceType(baseURL: string, id: string, instanceType: string): Promise<Instance> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.patch(path, JSON.stringify({ instanceType }));
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

    public async deleteInstance(baseURL: string, id: string): Promise<void> {
        const path = `${baseURL}/api/v1/instance/${id}`;
        const response = await this.http.delete(path);
        const result = await response.json();

        if (response.status !== 204) {
            throw new APIError({
                status: response.status,
                message: result.message || 'Can not delete Instance',
                requestID: result.requestID,
            });
        }
    }

    private instanceFromJSON(instance: Record<string, never>): Instance {
        return new Instance(
            instance.id,
            instance.name,
            instance.status,
            instance.hostname,
            instance.ipv4Address,
            new Date(instance.created),
            new Date(instance.updated),
            instance.remote,
            instance.password ?? '',
            instance.deleting ?? false,
        );
    }
}
