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
import { Download } from '@/utils/download';
import { DownloadPrefixFormat } from '@/types/browser';
import { RestrictGrantMessage, useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

export enum ShareType {
    Object = 'object',
    Folder = 'folder',
    Bucket = 'bucket',
}

export class ShareInfo {
    public constructor(
        public readonly url: string,
        public readonly freeTrialExpiration: Date | null = null,
    ) { }
}

export function useLinksharing() {
    const agStore = useAccessGrantsStore();
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const bucketsStore = useBucketsStore();
    const objectBrowserStore = useObjectBrowserStore();

    const { generateAccess, restrictGrant } = useAccessGrantWorker();

    const selectedProject = computed<Project>(() => projectsStore.state.selectedProject);

    const linksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.internalLinksharing || configStore.state.config.linksharingURL;
    });

    const publicLinksharingURL = computed<string>(() => {
        return selectedProject.value.edgeURLOverrides?.publicLinksharing || configStore.state.config.publicLinksharingURL;
    });

    async function generateFileOrFolderShareURL(bucketName: string, prefix: string, objectKey: string, type: ShareType, accessName: string, expiration: Date | null): Promise<ShareInfo> {
        return generateShareURL(bucketName, prefix, objectKey, type, accessName, expiration);
    }

    async function generateBucketShareURL(bucketName: string, accessName: string, expiration: Date | null): Promise<ShareInfo> {
        return generateShareURL(bucketName, '', '', ShareType.Bucket, accessName, expiration);
    }

    async function generateShareURL(bucketName: string, prefix: string, objectKey: string, type: ShareType, accessName: string, expiration: Date | null): Promise<ShareInfo> {
        let fullPath = bucketName;
        if (prefix) fullPath = `${fullPath}/${prefix}`;
        if (objectKey) fullPath = `${fullPath}/${objectKey}`;
        if (type === ShareType.Folder) fullPath = `${fullPath}/`;

        const grant: AccessGrant = await agStore.createAccessGrant(accessName, selectedProject.value.id);
        const creds: EdgeCredentials = await generatePublicCredentials(grant.secret, fullPath, expiration);

        let url = `${publicLinksharingURL.value}/s/${creds.accessKeyId}/${bucketName}`;
        if (prefix) url = `${url}/${encodeURIComponent(prefix.trim())}`;
        if (objectKey) url = `${url}/${encodeURIComponent(objectKey.trim())}`;
        if (type === ShareType.Folder) url = `${url}/`;

        return new ShareInfo(url, creds.freeTierRestrictedExpiration);
    }

    async function downloadPrefix(bucketName: string, prefix: string, format: DownloadPrefixFormat): Promise<void> {
        const now = new Date();
        const expiresAt = new Date();
        expiresAt.setHours(now.getHours() + 1);

        let fullPath = bucketName;
        if (prefix) fullPath = `${fullPath}/${prefix}/`;

        const creds = await generatePublicCredentials(bucketsStore.state.apiKey, fullPath, expiresAt, bucketsStore.state.passphrase);

        let link = `${publicLinksharingURL.value}/s/${creds.accessKeyId}/${bucketName}`;
        if (prefix) link = `${link}/${encodeURIComponent(prefix.trim())}/`;

        const url = new URL(`${link}?download=1&download-kind=${format}`);

        Download.fileByLink(url.href);
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
        if (passphrase === undefined) passphrase = bucketsStore.state.passphrase;

        const macaroon = await generateAccess({
            apiKey: cleanAPIKey,
            passphrase: passphrase,
        }, selectedProject.value.id);

        let permissionsMsg: RestrictGrantMessage = {
            isDownload: true,
            isUpload: false,
            isList: true,
            isDelete: false,
            paths: [path],
            grant: macaroon,
        };
        if (expiration) permissionsMsg = Object.assign(permissionsMsg, { notAfter: expiration.toISOString() });

        const accessGrant = await restrictGrant(permissionsMsg);

        return agStore.getEdgeCredentials(accessGrant, true);
    }

    return {
        publicLinksharingURL,
        generatePublicCredentials,
        generateBucketShareURL,
        generateFileOrFolderShareURL,
        getObjectDistributionMap,
        downloadPrefix,
    };
}
