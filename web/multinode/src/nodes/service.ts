// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { NodesClient } from '@/api/nodes';
import { CreateNodeFields, Node, NodeURL } from '@/nodes/index';

/**
 * exposes all nodes related logic
 */
export class Nodes {
    private readonly nodes: NodesClient;

    public constructor(nodes: NodesClient) {
        this.nodes = nodes;
    }

    /**
     * handles node addition.
     *
     * @param node - node to add.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async add(node: CreateNodeFields): Promise<void> {
        await this.nodes.add(node);
    }

    /**
     * returns list of node infos.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async list(): Promise<Node[]> {
        return await this.nodes.list();
    }

    /**
     * returns list of node infos by satellite.
     *
     * @param satelliteId - id of the satellite.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async listBySatellite(satelliteId: string): Promise<Node[]> {
        return await this.nodes.listBySatellite(satelliteId);
    }

    /**
     * updates nodes name.
     *
     * @param id - id of the node.
     * @param name - new node name.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async updateName(id: string, name: string): Promise<void> {
        await this.nodes.updateName(id, name);
    }

    /**
     * deletes node.
     *
     * @param id - id of the node.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async delete(id: string): Promise<void> {
        await this.nodes.delete(id);
    }

    /**
     * retrieves list of trusted satellites node urls for a node.
     */
    public async trustedSatellites(): Promise<NodeURL[]> {
        return await this.nodes.trustedSatellites();
    }
}
