// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';
import { HttpRequest } from '@smithy/types';
import { Sha256 } from '@aws-crypto/sha256-browser';
import { SignatureV4 } from '@smithy/signature-v4';

import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { Project } from '@/types/projects';

const WORKER_ERR_MSG = 'Worker is not defined';

export enum ShareType {
    Object = 'object',
    Folder = 'folder',
    Bucket = 'bucket',
}

export function useLinksharing() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();
    const objectBrowserStore = useObjectBrowserStore();

    const worker = computed((): Worker | null => agStore.state.accessGrantsWebWorker);

    const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);

    const linksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.internalLinksharing || configStore.state.config.linksharingURL;
    });

    const publicLinksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.publicLinksharing || configStore.state.config.publicLinksharingURL;
    });

    async function generateFileOrFolderShareURL(bucketName: string, prefix: string, objectKey: string, type: ShareType): Promise<string> {
        return generateShareURL(bucketName, prefix, objectKey, type);
    }

    async function generateBucketShareURL(bucketName: string): Promise<string> {
        return generateShareURL(bucketName, '', '', ShareType.Bucket);
    }

    async function generateShareURL(bucketName: string, prefix: string, objectKey: string, type: ShareType): Promise<string> {
        if (!worker.value) throw new Error(WORKER_ERR_MSG);

        let fullPath = bucketName;
        if (prefix) fullPath = `${fullPath}/${prefix}`;
        if (objectKey) fullPath = `${fullPath}/${objectKey}`;
        if (type === ShareType.Folder) fullPath = `${fullPath}/`;

        const LINK_SHARING_AG_NAME = `${fullPath}_shared-${type}_${new Date().toISOString()}`;
        const grant: AccessGrant = await agStore.createAccessGrant(LINK_SHARING_AG_NAME, selectedProject.value.id);
        const creds: EdgeCredentials = await generatePublicCredentials(grant.secret, fullPath, null);

        let url = `${publicLinksharingURL.value}/s/${creds.accessKeyId}/${bucketName}`;
        if (prefix) url = `${url}/${encodeURIComponent(prefix.trim())}`;
        if (objectKey) url = `${url}/${encodeURIComponent(objectKey.trim())}`;
        if (type === ShareType.Folder) url = `${url}/`;

        return url;
    }

    async function getObjectDistributionMap(path: string): Promise<Blob> {
        if (objectBrowserStore.state.s3 === null) throw new Error(
            'ObjectsModule: S3 Client is uninitialized',
        );

        const url = new URL(`${linksharingURL.value}/s/${objectBrowserStore.state.accessKey}/${path}`);
        const request: HttpRequest = {
            method: 'GET',
            protocol: url.protocol,
            hostname: url.hostname,
            port: parseFloat(url.port),
            path: url.pathname,
            headers: {
                'host': url.host,
            },
            query: {
                'map': '1',
            },
        };

        const creds = await objectBrowserStore.state.s3.config.credentials();
        const signer = new SignatureV4({
            applyChecksum: true,
            uriEscapePath: false,
            credentials: creds,
            region: 'eu1',
            service: 'linksharing',
            sha256: Sha256,
        });

        const signedRequest: HttpRequest = await signer.sign(request);
        const requestURL = `${linksharingURL.value}${signedRequest.path}?map=1`;

        const response = await fetch(requestURL, signedRequest);
        if (response.ok) {
            return await response.blob();
        }

        throw new Error();
    }

    async function generatePublicCredentials(cleanAPIKey: string, path: string, expiration: Date | null, passphrase?: string): Promise<EdgeCredentials> {
        if (!worker.value) throw new Error(WORKER_ERR_MSG);

        const satelliteNodeURL = configStore.state.config.satelliteNodeURL;
        const salt = await projectsStore.getProjectSalt(selectedProject.value.id);
        if (passphrase === undefined) passphrase = bucketsStore.state.passphrase;

        worker.value.postMessage({
            'type': 'GenerateAccess',
            'apiKey': cleanAPIKey,
            'passphrase': passphrase,
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
        publicLinksharingURL,
        generatePublicCredentials,
        generateBucketShareURL,
        generateFileOrFolderShareURL,
        getObjectDistributionMap,
    };
}
