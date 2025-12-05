// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

interface GeneralCaveats {
    isDownload: boolean;
    isUpload: boolean;
    isList: boolean;
    isDelete: boolean;
}

export interface SetPermissionsMessage extends GeneralCaveats {
    buckets: string;
    apiKey: string;
    notBefore?: string;
    notAfter?: string;
    isPutObjectRetention?: boolean;
    isGetObjectRetention?: boolean;
    isBypassGovernanceRetention?: boolean;
    isPutObjectLegalHold?: boolean;
    isGetObjectLegalHold?: boolean;
    isPutObjectLockConfiguration?: boolean;
    isGetObjectLockConfiguration?: boolean;
}

export interface RestrictGrantMessage extends GeneralCaveats {
    paths: string[],
    grant: string,
}

export interface GenerateAccessGrantMessage {
    apiKey: string,
    passphrase: string,
}

export function useAccessGrantWorker() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();

    const notify = useNotify();

    const worker = computed((): Worker | null => agStore.state.accessGrantsWebWorker);

    async function setPermissions(message: SetPermissionsMessage): Promise<string> {
        if (!worker.value) throw new Error('Worker is not defined');

        message['type'] = 'SetPermission';

        worker.value.postMessage(message);

        return await handleEvent();
    }

    async function generateAccess(message: GenerateAccessGrantMessage, projectID: string): Promise<string> {
        if (!worker.value) throw new Error('Worker is not defined');

        message['salt'] = await projectsStore.getProjectSalt(projectID);
        message['encryptPath'] = projectsStore.selectedProjectConfig.encryptPath;
        message['satelliteNodeURL'] = configStore.state.config.satelliteNodeURL;
        message['type'] = 'GenerateAccess';

        worker.value.postMessage(message);

        return await handleEvent();
    }

    async function restrictGrant(message: RestrictGrantMessage): Promise<string> {
        if (!worker.value) throw new Error('Worker is not defined');

        message['type'] = 'RestrictGrant';

        worker.value.postMessage(message);

        return await handleEvent();
    }

    async function handleEvent(): Promise<string> {
        const event: MessageEvent = await new Promise(resolve => {
            if (worker.value) worker.value.onmessage = resolve;
        });
        if (event.data.error) throw new Error(event.data.error);

        return event.data.value;
    }

    async function start(): Promise<void> {
        const worker = new Worker(new URL('@/utils/accessGrant.worker.js', import.meta.url));

        worker.postMessage({ 'type': 'Setup' });

        const event: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
        if (event.data.error) throw new Error(event.data.error);
        if (event.data !== 'configured') throw new Error('Failed to configure access grants web worker');

        worker.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.ACCESS_GRANTS_WEB_WORKER);
            throw new Error(error.message);
        };

        agStore.setWorker(worker);
    }

    function stop(): void {
        worker.value?.terminate();
        agStore.setWorker(null);
    }

    return {
        start,
        stop,
        setPermissions,
        generateAccess,
        restrictGrant,
    };
}
