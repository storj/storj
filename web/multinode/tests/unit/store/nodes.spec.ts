// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import store, { nodesService } from '../mock/store';

import { RootState } from '@/app/store';
import { CreateNodeFields, Node, NodeURL } from '@/nodes';

const node = new Node();
const nodes = [node];
const satellite = new NodeURL('testId', '127.0.0.1:test');
const trustedSatellites = [satellite];
const nodeToAdd = new CreateNodeFields('newId', 'secret', 'newAddress');

const state = store.state as RootState;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('populates', () => {
        store.commit('nodes/populate', nodes);

        expect(state.nodes.nodes.length).toBe(1);
    });

    it('saves trusted satellites', () => {
        store.commit('nodes/saveTrustedSatellites', trustedSatellites);

        expect(state.nodes.trustedSatellites.length).toBe(1);
    });

    it('saves selected satellite', () => {
        store.commit('nodes/setSelectedSatellite', satellite.id);

        const selectedSatellite = state.nodes.selectedSatellite;
        expect(selectedSatellite).toBeDefined();
        if (selectedSatellite) expect(selectedSatellite.address).toBe(satellite.address);
    });

    it('saves selected node', () => {
        expect(state.nodes.selectedNode).toBe(null);

        store.commit('nodes/setSelectedNode', node.id);

        const selectedNode = state.nodes.selectedNode;
        expect(selectedNode).toBeDefined();
        if (selectedNode) expect(selectedNode.id).toBe(node.id);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        store.commit('nodes/populate', []);
        store.commit('nodes/saveTrustedSatellites', []);
        store.commit('nodes/setSelectedSatellite', null);
    });

    it('throws error on failed nodes fetch', async() => {
        jest.spyOn(nodesService, 'list').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/fetch');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.nodes.length).toBe(0);
        }
    });

    it('success get nodes', async() => {
        jest.spyOn(nodesService, 'list').mockReturnValue(
            Promise.resolve(nodes),
        );

        await store.dispatch('nodes/fetch');

        expect(state.nodes.nodes.length).toBe(1);
    });

    it('throws error on failed node addition', async() => {
        jest.spyOn(nodesService, 'add').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/add');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.nodes.length).toBe(0);
        }
    });

    it('success adds node', async() => {
        const addSpy = jest.fn();

        jest.spyOn(nodesService, 'add').mockImplementation(addSpy);

        await store.dispatch('nodes/add', nodeToAdd);

        expect(addSpy).toBeCalled();
    });

    it('throws error on failed node deletion', async() => {
        jest.spyOn(nodesService, 'delete').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/delete');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.nodes.length).toBe(0);
        }
    });

    it('success deletes node', async() => {
        const deleteSpy = jest.fn();

        jest.spyOn(nodesService, 'delete').mockImplementation(deleteSpy);

        await store.dispatch('nodes/delete', node.id);

        expect(deleteSpy).toBeCalled();
    });

    it('throws error on failed node name update', async() => {
        jest.spyOn(nodesService, 'updateName').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/updateName');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.nodes.length).toBe(0);
        }
    });

    it('success updates node name', async() => {
        const updateNameSpy = jest.fn();

        jest.spyOn(nodesService, 'updateName').mockImplementation(updateNameSpy);

        await store.dispatch('nodes/updateName', nodeToAdd);

        expect(updateNameSpy).toBeCalled();
    });

    it('throws error on failed trusted satellites', async() => {
        jest.spyOn(nodesService, 'trustedSatellites').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/trustedSatellites');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.trustedSatellites.length).toBe(0);
        }
    });

    it('success get trusted satellites', async() => {
        jest.spyOn(nodesService, 'trustedSatellites').mockReturnValue(
            Promise.resolve(trustedSatellites),
        );

        await store.dispatch('nodes/trustedSatellites');

        expect(state.nodes.trustedSatellites.length).toBe(1);
    });

    it('throws error on failed satellite selection', async() => {
        jest.spyOn(nodesService, 'list').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('nodes/selectSatellite');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.nodes.selectedSatellite).toBe(null);
        }
    });

    it('success get trusted satellites', async() => {
        jest.spyOn(nodesService, 'listBySatellite').mockReturnValue(
            Promise.resolve(nodes),
        );
        jest.spyOn(nodesService, 'trustedSatellites').mockReturnValue(
            Promise.resolve(trustedSatellites),
        );

        store.commit('nodes/saveTrustedSatellites', trustedSatellites);

        await store.dispatch('nodes/selectSatellite', satellite.id);

        const selectedSatellite = state.nodes.selectedSatellite;
        expect(selectedSatellite).toBeDefined();
        if (selectedSatellite) expect(selectedSatellite.address).toBe(satellite.address);

        expect(state.nodes.nodes.length).toBe(1);
    });

    it('success set selected node', async() => {
        store.commit('nodes/populate', nodes);

        await store.dispatch('nodes/selectNode', node.id);

        const selectedNode = state.nodes.selectedNode;
        expect(selectedNode).toBeDefined();
        if (selectedNode) expect(selectedNode.id).toBe(node.id);
    });
});
