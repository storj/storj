// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all compute related functionality.
 */
export interface IComputeAPI {
    createSSHKey(baseURL: string, authToken: string, request: CreateSSHKeyRequest): Promise<SSHKey>;
    getSSHKeys(baseURL: string, authToken: string): Promise<SSHKey[]>;
    deleteSSHKey(baseURL: string, authToken: string, id: string): Promise<void>;

    createInstance(baseURL: string, authToken: string, request: CreateInstanceRequest): Promise<Instance>;
    getInstance(baseURL: string, authToken: string, id: string): Promise<Instance>;
    getInstances(baseURL: string, authToken: string): Promise<Instance[]>;
    updateInstanceType(baseURL: string, authToken: string, id: string, instanceType: string): Promise<Instance>;
    deleteInstance(baseURL: string, authToken: string, id: string): Promise<void>;

    getAvailableInstanceTypes(baseURL: string, authToken: string): Promise<string[]>;
    getAvailableImages(baseURL: string, authToken: string): Promise<string[]>;
    getAvailableLocations(baseURL: string, authToken: string): Promise<string[]>;
}

export interface CreateSSHKeyRequest {
    name: string;
    publicKey: string;
}

export class SSHKey {
    constructor(
        public id: string = '',
        public name: string = '',
        public publicKey: string = '',
        public created: Date = new Date(),
    ) { }
}

export interface CreateInstanceRequest {
    name: string;
    hostname: string;
    instanceType: string;
    bootDiskSizeGB: number;
    image: string;
    location: string;
    sshKeys: string[];
}

export interface Remote {
    type: string;
    ipv4Address: string;
    port: number;
}

export class Instance {
    constructor(
        public id: string = '',
        public name: string = '',
        public status: string = '',
        public hostname: string = '',
        public ipv4Address: string = '',
        public created: Date = new Date(),
        public updated: Date = new Date(),
        public remote: Remote = {
            type: '',
            ipv4Address: '',
            port: 0,
        },
        public password: string = '',
        public deleting: boolean = false,
    ) { }
}
