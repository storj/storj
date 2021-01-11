// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * NodeToAdd is a representation of storagenode, that SNO could add to the Multinode Dashboard.
 */
export class NodeToAdd {
    public id: string; // TODO: create ts analog of storj.NodeID;
    /**
     * apiSecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
     */
    public apiSecret: string; // TODO: change to Uint8Array[];
    public publicAddress: string;
    public name: string;
}

/**
 * Describes node online statuses.
 */
export enum NodeStatus {
    Online = 'online',
    Offline = 'offline',
}

// TODO: refactor this
/**
 * Node holds all information of node for the Multinode Dashboard.
 */
export class Node {
    public constructor(
        public id: string = '',
        public name: string = '',
        public diskSpaceUsed: number = 0,
        public diskSpaceLeft: number = 0,
        public bandwidthUsed: number = 0,
        public earned: number = 0,
        public version: string = '',
        public status: NodeStatus = NodeStatus.Offline,
    ) {}
}
