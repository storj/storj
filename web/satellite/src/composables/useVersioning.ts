// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { Versioning } from '@/types/versioning';

export function useVersioning() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();

    const worker = computed((): Worker | null => agStore.state.accessGrantsWebWorker);

    /**
     * Returns edge credentials for bucket creation from store.
     */
    const edgeCredentialsForVersioning = computed((): EdgeCredentials => {
        return bucketsStore.state.edgeCredentialsForVersioning;
    });

    /**
     * Returns object browser api key from store.
     */
    const apiKey = computed((): string => {
        return bucketsStore.state.apiKey;
    });

    /**
     * Toggles versioning for the bucket between Suspended and Enabled.
     */
    async function toggleVersioning(bucketName: string, currentVersioning: Versioning): Promise<void> {
        const projectID = projectsStore.state.selectedProject.id;

        if (!worker.value) {
            throw new Error('Worker is not defined');
        }

        const now = new Date();

        if (!apiKey.value) {
            const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
            bucketsStore.setApiKey(cleanAPIKey.secret);
        }

        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        worker.value.postMessage({
            'type': 'SetPermission',
            'isDownload': false,
            'isUpload': true,
            'isList': false,
            'isDelete': false,
            'notAfter': inOneHour.toISOString(),
            'buckets': JSON.stringify([bucketName]),
            'apiKey': apiKey.value,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);
        const satelliteNodeURL: string = configStore.state.config.satelliteNodeURL;

        worker.value.postMessage({
            'type': 'GenerateAccess',
            'apiKey': grantEvent.data.value,
            'passphrase': '',
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessGrantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        if (accessGrantEvent.data.error) {
            throw new Error(accessGrantEvent.data.error);
        }

        const accessGrant = accessGrantEvent.data.value;

        const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        bucketsStore.setEdgeCredentialsForVersioning(edgeCredentials);
        await bucketsStore.setVersioning(bucketName, currentVersioning !== Versioning.Enabled);
    }

    return {
        toggleVersioning,
    };
}
