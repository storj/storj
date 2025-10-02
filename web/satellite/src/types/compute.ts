// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all compute related functionality.
 */
export interface IComputeAPI {
    createSSHKey(baseURL: string, request: CreateSSHKeyRequest): Promise<SSHKey>;
    getSSHKeys(baseURL: string): Promise<SSHKey[]>;
    deleteSSHKey(baseURL: string, id: string): Promise<void>;
}

export interface CreateSSHKeyRequest {
    name: string;
    publicKey: string;
}

export class SSHKey {
    constructor(
        public id: string,
        public name: string,
        public publicKey: string,
        public created: Date,
    ) { }
}
