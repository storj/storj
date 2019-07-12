import { NODE_ACTIONS, NODE_MUTATIONS } from '@/utils/constants';
import { httpGet } from '@/api/storagenode';
import { formatBytes } from '@/utils/converter.ts';
import { BandwidthChartDataFormatter } from '@/utils/chartModule'

export const nodeModule = {
    state: {
        node: {
            id: '',
            status: '',
            version: '',
            wallet: '',
            isLastVersion: false
        },
        satellites: [],
        selectedSatellite: null,
        bandwidthChartData: new BandwidthChartDataFormatter([]).getFormattedData(),
        bandwidth: {
            egress: null,
            ingress: null,
            used: 0,
            remaining: 1,
            available: 1,
        },
        diskSpace: {
            used: 0,
            remaining: 1,
            available: 1,
        },
        checks: {
            uptime: '0',
            audit: '23.5',
        },
    },
    mutations: {
        [NODE_MUTATIONS.POPULATE_STORE](state: any, nodeInfo: any): void {
            const versionInfo = nodeInfo.versionInfo.version;
            state.node.id = nodeInfo.nodeId;
            state.isLastVersion = nodeInfo.isLastVersion;
            state.node.version = `v${versionInfo.major}.${versionInfo.minor}.${versionInfo.patch}`;
            state.node.wallet = nodeInfo.walletAddress;
            state.diskSpace.used = formatBytes(nodeInfo.diskSpace.used);
            state.diskSpace.remaining = formatBytes(nodeInfo.diskSpace.available - nodeInfo.diskSpace.used);
            state.diskSpace.available = formatBytes(nodeInfo.diskSpace.available);
            state.bandwidth.used = formatBytes(nodeInfo.bandwidth.used);
            state.bandwidth.remaining = formatBytes(nodeInfo.bandwidth.remaining);
            state.bandwidth.available = formatBytes(nodeInfo.bandwidth.remaining + nodeInfo.bandwidth.used);
            state.satellites = nodeInfo.satellites;
            state.bandwidthChartData = new BandwidthChartDataFormatter(nodeInfo.bandwidthChartData).getFormattedData();
            console.log(nodeInfo);
        },

        [NODE_MUTATIONS.SELECT_SATELLITE](state, id: any): void {
            if (id) {
                state.satellites.forEach(satellite => {
                    if (id === satellite) {
                        state.selectedSatellite = satellite;
                    }
                });
            }
            else {
                state.selectedSatellite = null;
            }
        },
    },
    actions: {
        [NODE_ACTIONS.GET_NODE_INFO]: async function ({commit}: any, url: string): Promise<any> {
            let response = await httpGet(url);
            if (response.data) {
                commit(NODE_MUTATIONS.POPULATE_STORE, response.data);

                return;
            }

            console.error('Error while fetching Node info!');
        },
        [NODE_ACTIONS.SELECT_SATELLITE]: function ({commit}, id: any): void {
            commit(NODE_MUTATIONS.SELECT_SATELLITE, id);
        },
    },
};
