// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_INVOICE_MUTATIONS } from '@/store/mutationConstants';
import { PROJECT_INVOICE_ACTIONS } from '@/utils/constants/actionNames';
import { fetchProjectInvoices } from '@/api/invoices';

export const projectInvoicesModule = {
    state: {
        invoices: [] as ProjectInvoice[],
    },
    mutations: {
        [PROJECT_INVOICE_MUTATIONS.FETCH](state: any, invoices: ProjectInvoice[]) {
            state.invoices = invoices;
        },
        [PROJECT_INVOICE_MUTATIONS.CLEAR](state: any) {
            state.invoices = [] as ProjectInvoice[];
        }
    },
    actions: {
        [PROJECT_INVOICE_ACTIONS.FETCH]: async function({commit, rootGetters}): Promise<RequestResponse<ProjectInvoice[]>> {
            const projectId = rootGetters.selectedProject.id;

            let result = await fetchProjectInvoices(projectId);
            if (result.isSuccess) {
                commit(PROJECT_INVOICE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_INVOICE_ACTIONS.CLEAR]: function({commit}) {
            commit(PROJECT_INVOICE_MUTATIONS.CLEAR)
        }
    },
};
