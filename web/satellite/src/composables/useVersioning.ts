// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { Versioning } from '@/types/versioning';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

export function useVersioning() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();

    const { setPermissions, generateAccess } = useAccessGrantWorker();

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
        const now = new Date();

        if (!apiKey.value) {
            const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
            bucketsStore.setApiKey(cleanAPIKey.secret);
        }

        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        const macaroon = await setPermissions({
            isDownload: false,
            isUpload: true,
            isList: false,
            isDelete: false,
            notAfter: inOneHour.toISOString(),
            buckets: JSON.stringify([bucketName]),
            apiKey: apiKey.value,
        });

        const accessGrant = await generateAccess({
            apiKey: macaroon,
            passphrase: '',
        }, projectsStore.state.selectedProject.id);

        const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        bucketsStore.setEdgeCredentialsForVersioning(edgeCredentials);
        await bucketsStore.setVersioning(bucketName, currentVersioning !== Versioning.Enabled);
    }

    return {
        toggleVersioning,
    };
}
