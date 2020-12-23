// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Node is a representation of storagenode, that SNO could add to the Multinode Dashboard.
 */
export class Node {
    public id: string; // TODO: create ts analog of storj.NodeID;
    /**
     * apiSecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
     */
    public apiSecret: string; // TODO: change to Uint8Array[];
    public publicAddress: string;
    public name: string;
}
