// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Describes node online statuses.
 */
export enum NodeStatus {
    Online = 'online',
    Offline = 'offline',
}

/**
 * NodeInfo contains basic node internal state.
 */
export class Node {
    public status: NodeStatus = NodeStatus.Offline;
    private readonly STATUS_TRESHHOLD_MILISECONDS: number = 10.8e6;

    public constructor(
        public id: string,
        public name: string,
        public version: string,
        public lastContact: Date,
        public diskSpaceUsed: number,
        public diskSpaceLeft: number,
        public bandwidthUsed: number,
        public onlineScore: number,
        public auditScore: number,
        public suspensionScore: number,
        public earned: number,
    ) {
        const now = new Date();
        if (now.getTime() - this.lastContact.getTime() < this.STATUS_TRESHHOLD_MILISECONDS) {
            this.status = NodeStatus.Online;
        }
    }

    public get displayedName(): string {
        return this.name || this.id;
    }
}

/**
 * CreateNodeFields is a representation of storagenode, that SNO could add to the Multinode Dashboard.
 */
export class CreateNodeFields {
    public constructor(
        public id: string = '',
        public apiSecret: string = '',
        public publicAddress: string = '',
    ) {}
}

/**
 * NodeURL defines a structure for connecting to a node.
 */
export class NodeURL {
    public constructor(
        public id: string,
        public address: string,
    ) {}
}
