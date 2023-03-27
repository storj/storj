// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';
import S3, { Bucket } from 'aws-sdk/clients/s3';

import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { MetaUtils } from '@/utils/meta';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

export const FILE_BROWSER_AG_NAME = 'Web file browser API key';

export class ObjectsState {
    public apiKey = '';
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForDelete: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForCreate: EdgeCredentials = new EdgeCredentials();
    public s3Client: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public s3ClientForDelete: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public s3ClientForCreate: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public bucketsList: Bucket[] = [];
    public passphrase = '';
    public promptForPassphrase = true;
    public fileComponentBucketName = '';
    public leaveRoute = '';
}

export const useObjectsStore = defineStore('objects', () => {
    const state = reactive<ObjectsState>(new ObjectsState());

    function setPromptForPassphrase(value: boolean): void {
        state.promptForPassphrase = value;
    }

    function setApiKey(apiKey: string): void {
        state.apiKey = apiKey;
    }

    function setEdgeCredentials(credentials: EdgeCredentials): void {
        state.edgeCredentials = credentials;
    }

    function setEdgeCredentialsForDelete(credentials: EdgeCredentials): void {
        state.edgeCredentialsForDelete = credentials;

        const s3Config = {
            accessKeyId: state.edgeCredentialsForDelete.accessKeyId,
            secretAccessKey: state.edgeCredentialsForDelete.secretKey,
            endpoint: state.edgeCredentialsForDelete.endpoint,
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        };

        state.s3ClientForDelete = new S3(s3Config);
    }

    function setEdgeCredentialsForCreate(credentials: EdgeCredentials): void {
        state.edgeCredentialsForCreate = credentials;

        const s3Config = {
            accessKeyId: state.edgeCredentialsForCreate.accessKeyId,
            secretAccessKey: state.edgeCredentialsForCreate.secretKey,
            endpoint: state.edgeCredentialsForCreate.endpoint,
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        };

        state.s3ClientForCreate = new S3(s3Config);
    }

    async function setS3Client(projectID: string): Promise<void> {
        const {
            createAccessGrant,
            deleteAccessGrantByNameAndProjectID,
            accessGrantsState,
            getEdgeCredentials,
        } = useAccessGrantsStore();

        if (!state.apiKey) {
            await deleteAccessGrantByNameAndProjectID(projectID, FILE_BROWSER_AG_NAME);
            const cleanAPIKey: AccessGrant = await createAccessGrant(projectID, FILE_BROWSER_AG_NAME);
            setApiKey(cleanAPIKey.secret);
        }

        const now = new Date();
        const inThreeDays = new Date(now.setDate(now.getDate() + 3));

        const worker = accessGrantsState.accessGrantsWebWorker;
        if (!worker) {
            throw new Error('Worker is not defined');
        }

        worker.onerror = (error: ErrorEvent) => {
            throw new Error(error.message);
        };

        await worker.postMessage({
            'type': 'SetPermission',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'notAfter': inThreeDays.toISOString(),
            'buckets': [],
            'apiKey': state.apiKey,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        const { getProjectSalt } = useProjectsStore();

        const salt = await getProjectSalt(projectID);
        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

        if (!state.passphrase) {
            throw new Error('Passphrase can\'t be empty');
        }

        worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': grantEvent.data.value,
            'passphrase': state.passphrase,
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessGrantEvent: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
        if (accessGrantEvent.data.error) {
            throw new Error(accessGrantEvent.data.error);
        }

        const accessGrant = accessGrantEvent.data.value;
        state.edgeCredentials = await getEdgeCredentials(accessGrant);

        const s3Config = {
            accessKeyId: state.edgeCredentials.accessKeyId,
            secretAccessKey: state.edgeCredentials.secretKey,
            endpoint: state.edgeCredentials.endpoint,
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        };

        state.s3Client = new S3(s3Config);
    }

    function setPassphrase(passphrase: string): void {
        state.passphrase = passphrase;
    }

    function setFileComponentBucketName(bucketName: string): void {
        state.fileComponentBucketName = bucketName;
    }

    async function fetchBuckets(): Promise<void> {
        const result = await state.s3Client.listBuckets().promise();

        state.bucketsList = result.Buckets ?? [];
    }

    async function createBucket(name: string): Promise<void> {
        await state.s3Client.createBucket({
            Bucket: name,
        }).promise();
    }

    async function createBucketWithNoPassphrase(name: string): Promise<void> {
        await state.s3ClientForCreate.createBucket({
            Bucket: name,
        }).promise();
    }

    async function deleteBucket(name: string): Promise<void> {
        await state.s3ClientForDelete.deleteBucket({
            Bucket: name,
        }).promise();
    }

    async function getObjectsCount(name: string): Promise<number> {
        const response =  await state.s3Client.listObjectsV2({
            Bucket: name,
        }).promise();

        return response.KeyCount === undefined ? 0 : response.KeyCount;
    }

    function clearObjects(): void {
        state.apiKey = '';
        state.passphrase = '';
        state.promptForPassphrase = true;
        state.edgeCredentials = new EdgeCredentials();
        state.edgeCredentialsForDelete = new EdgeCredentials();
        state.edgeCredentialsForCreate = new EdgeCredentials();
        state.s3Client = new S3({
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        });
        state.s3ClientForDelete = new S3({
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        });
        state.s3ClientForCreate = new S3({
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            httpOptions: { timeout: 0 },
        });
        state.bucketsList = [];
        state.fileComponentBucketName = '';
        state.leaveRoute = '';
    }

    function checkOngoingUploads(uploadingLength: number, leaveRoute: string): boolean {
        if (!uploadingLength) {
            return false;
        }

        state.leaveRoute = leaveRoute;

        const { updateActiveModal } = useAppStore();
        updateActiveModal(MODALS.uploadCancelPopup);

        return true;
    }

    return {
        objectsState: state,
        fetchBuckets,
    };
});
