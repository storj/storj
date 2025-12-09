// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Divider to convert payout amounts to cents.
 */
const PRICE_DIVIDER = 10000;

/**
 * Describes node online statuses.
 */
export enum NodeStatus {
    'online' = 'Online',
    'offline' = 'Offline',
    'not reachable' = 'Not Reachable',
    'unauthorized' = 'Unauthorized',
    'storagenode internal error' = 'Internal Error',
}

/**
 * NodeInfo contains basic node internal state.
 */
export class Node {
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
        public status: NodeStatus = NodeStatus['online'],
    ) {}

    /**
     * displayedName handles displayed name of the node.
     */
    public get displayedName(): string {
        return this.name || this.id;
    }

    /**
     * earnedCents returns earned value in cents.
     */
    public get earnedCents(): number {
        return this.earned / PRICE_DIVIDER;
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
        public name: string = '',
    ) {}
}

/**
 * NodeURL defines a structure for connecting to a node.
 */
export class NodeURL {
    public constructor(
        public id: string = '',
        public address: string = '',
    ) {}
}

/**
 * UpdateNodeModel defines a structure for updating node name.
 */
export class UpdateNodeModel {
    public constructor(
        public id: string = '',
        public name: string = '',
    ) {}
}
