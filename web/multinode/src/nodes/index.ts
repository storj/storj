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
        public id: string = '',
        public name: string = '',
        public version: string = '',
        public lastContact: Date = new Date(),
        public diskSpaceUsed: number = 0,
        public diskSpaceLeft: number = 0,
        public bandwidthUsed: number = 0,
        public onlineScore: number = 0,
        public auditScore: number = 0,
        public suspensionScore: number = 0,
        public earned: number = 0,
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

/**
 * UpdateNodeModel defines a structure for updating node name.
 */
export class UpdateNodeModel {
    public constructor(
        public id: string,
        public name: string,
    ) {}
}
