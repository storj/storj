// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import {
    AccessInspectResult,
    AccessManagementHttpApiV1,
} from '@/api/client.gen';

class AccessState {}

export const useAccessStore = defineStore('access', () => {
    const state = reactive<AccessState>(new AccessState());

    const accessApi = new AccessManagementHttpApiV1();

    async function inspectAccess(access: string): Promise<AccessInspectResult> {
        return accessApi.inspectAccess({ access });
    }

    async function revokeAccess(tail: string, apiKeyID: string, reason: string): Promise<void> {
        return accessApi.revokeAccess({ tail, apiKeyID, reason });
    }

    return {
        state,
        inspectAccess,
        revokeAccess,
    };
});
