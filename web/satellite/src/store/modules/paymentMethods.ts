// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_PAYMENT_METHODS_MUTATIONS } from '@/store/mutationConstants';
import { PROJECT_PAYMENT_METHODS_ACTIONS } from '@/utils/constants/actionNames';
import {
    addProjectPaymentMethodRequest,
    deletePaymentMethodRequest,
    fetchProjectPaymentMethods,
    setDefaultPaymentMethodRequest
} from '@/api/paymentMethods';

export const projectPaymentsMethodsModule = {
    state: {
        paymentMethods: [] as PaymentMethod[],
    },
    mutations: {
        [PROJECT_PAYMENT_METHODS_MUTATIONS.FETCH](state: any, invoices: PaymentMethod[]) {
            state.paymentMethods = invoices;
        },
        [PROJECT_PAYMENT_METHODS_MUTATIONS.CLEAR](state: any) {
            state.paymentMethods = [] as PaymentMethod[];
        }
    },
    actions: {
        [PROJECT_PAYMENT_METHODS_ACTIONS.ADD]: async function ({commit, rootGetters, state}, input: AddPaymentMethodInput): Promise<RequestResponse<null>> {
            const projectID = rootGetters.selectedProject.id;
            if (state.paymentMethods.length == 0) {
                input.makeDefault = true;
            }

            return await addProjectPaymentMethodRequest(projectID, input.token, input.makeDefault);
        },
        [PROJECT_PAYMENT_METHODS_ACTIONS.FETCH]: async function ({commit, rootGetters}): Promise<RequestResponse<PaymentMethod[]>> {
            const projectId = rootGetters.selectedProject.id;

            let result = await fetchProjectPaymentMethods(projectId);
            if (result.isSuccess) {
                commit(PROJECT_PAYMENT_METHODS_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_PAYMENT_METHODS_ACTIONS.CLEAR]: function ({commit}) {
            commit(PROJECT_PAYMENT_METHODS_MUTATIONS.CLEAR);
        },
        [PROJECT_PAYMENT_METHODS_ACTIONS.SET_DEFAULT]: async function ({commit, rootGetters}, projectPaymentID: string) {
            const projectID = rootGetters.selectedProject.id;

            return await setDefaultPaymentMethodRequest(projectID, projectPaymentID);
        },
        [PROJECT_PAYMENT_METHODS_ACTIONS.DELETE]: async function ({commit}, projectPaymentID: string) {
            return await deletePaymentMethodRequest(projectPaymentID);
        }
    },
};
