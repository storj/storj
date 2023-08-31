// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { Project } from '@/types/projects';

const WORKER_ERR_MSG = 'Worker is not defined';

export function useLinksharing() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();

    const worker = computed((): Worker | null => agStore.state.accessGrantsWebWorker);

    const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);

    const linksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.internalLinksharing || configStore.state.config.linksharingURL;
    });

    const publicLinksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.publicLinksharing || configStore.state.config.publicLinksharingURL;
    });

    async function generateFileOrFolderShareURL(bucketName: string, path: string, isFolder = false): Promise<string> {
        const fullPath = `${bucketName}/${path}`;
        const type = isFolder ? 'folder' : 'object';
        return generateShareURL(fullPath, type);
    }

    async function generateBucketShareURL(bucketName: string): Promise<string> {
        return generateShareURL(bucketName, 'bucket');
    }

    async function generateShareURL(path: string, type: string): Promise<string> {
        if (!worker.value) throw new Error(WORKER_ERR_MSG);

        const LINK_SHARING_AG_NAME = `${path}_shared-${type}_${new Date().toISOString()}`;
        const grant: AccessGrant = await agStore.createAccessGrant(LINK_SHARING_AG_NAME, selectedProject.value.id);
        const credentials: EdgeCredentials = await generateCredentials(grant.secret, path, null);

        return `${publicLinksharingURL.value}/${credentials.accessKeyId}/${encodeURIComponent(path.trim())}`;
    }

    async function generateObjectPreviewAndMapURL(bucketName: string, path: string): Promise<string> {
        if (!worker.value) throw new Error(WORKER_ERR_MSG);

        path = bucketName + '/' + path;
        const now = new Date();
        const inOneDay = new Date(now.setDate(now.getDate() + 1));
        const creds: EdgeCredentials = await generateCredentials(bucketsStore.state.apiKey, path, inOneDay);

        return `${linksharingURL.value}/s/${creds.accessKeyId}/${encodeURIComponent(path.trim())}`;
    }

    async function generateCredentials(cleanAPIKey: string, path: string, expiration: Date | null): Promise<EdgeCredentials> {
        if (!worker.value) throw new Error(WORKER_ERR_MSG);

        const satelliteNodeURL = configStore.state.config.satelliteNodeURL;
        const salt = await projectsStore.getProjectSalt(selectedProject.value.id);

        worker.value.postMessage({
            'type': 'GenerateAccess',
            'apiKey': cleanAPIKey,
            'passphrase': bucketsStore.state.passphrase,
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        const grantData = grantEvent.data;
        if (grantData.error) {
            throw new Error(grantData.error);
        }

        let permissionsMsg = {
            'type': 'RestrictGrant',
            'isDownload': true,
            'isUpload': false,
            'isList': true,
            'isDelete': false,
            'paths': [path],
            'grant': grantData.value,
        };

        if (expiration) {
            permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': expiration.toISOString() });
        }

        worker.value.postMessage(permissionsMsg);

        const event: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        const data = event.data;
        if (data.error) {
            throw new Error(data.error);
        }

        return agStore.getEdgeCredentials(data.value, true);
    }

    return {
        generateBucketShareURL,
        generateFileOrFolderShareURL,
        generateObjectPreviewAndMapURL,
    };
}
