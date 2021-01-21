// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { BadRequestError, InternalError, UnauthorizedError } from '@/api/index';
import { CreateNodeFields, Node, NodeURL } from '@/nodes';
import { HttpClient } from '@/private/http/client';

/**
 * client for nodes controller of MND api.
 */
export class NodesClient {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/nodes';

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
        const path = `${this.ROOT_PATH}`;
        const response = await this.http.post(path, JSON.stringify(node));

        if (!response.ok) {
            await this.handleError(response);
        }
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
        const path = `${this.ROOT_PATH}/infos`;
        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const nodeListJson = await response.json();

        return nodeListJson.map(node => new Node(
            node.id,
            node.name,
            node.version,
            new Date(node.lastContact),
            node.diskSpaceUsed,
            node.diskSpaceLeft,
            node.bandwidthUsed,
            0,
            0,
            0,
            node.totalEarned,
        ));
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
        const path = `${this.ROOT_PATH}/infos/${satelliteId}`;
        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const nodeListJson = await response.json();

        return nodeListJson.map(node => new Node(
            node.id,
            node.name,
            node.version,
            new Date(node.lastContact),
            0,
            0,
            0,
            node.onlineScore,
            node.auditScore,
            node.suspensionScore,
            node.totalEarned,
        ));
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
        const path = `${this.ROOT_PATH}/${id}`;
        const response = await this.http.patch(path, JSON.stringify({name: name}));

        if (!response.ok) {
            await this.handleError(response);
        }
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
        const path = `${this.ROOT_PATH}/${id}`;
        const response = await this.http.delete(path);

        if (!response.ok) {
            await this.handleError(response);
        }
    }

    /**
     * retrieves list of trusted satellites node urls for a node.
     */
    public async trustedSatellites(): Promise<NodeURL[]> {
        const path = `${this.ROOT_PATH}/trusted-satellites`;
        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const urlListJson = await response.json();

        return urlListJson.map(url => new NodeURL(
            url.ID,
            url.Name,
        ));
    }

    /**
     * handles error due to response code.
     * @param response - response from server.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid ISBN number.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the ISBN number is valid, but no such book exists in the catalog.
     *
     * @throws {@link InternalError}
     * Thrown if the ISBN number is valid, but no such book exists in the catalog.
     *
     * @private
     */
    private async handleError(response: Response): Promise<void> {
        const body = await response.json();

        switch (response.status) {
            case 401: throw new UnauthorizedError(body.error);
            case 400: throw new BadRequestError(body.error);
            case 500:
            default:
                throw new InternalError(body.error);
        }
    }
}
