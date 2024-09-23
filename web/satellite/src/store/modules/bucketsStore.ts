// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';
import {
    S3Client,
    S3ClientConfig,
    CreateBucketCommand,
    DeleteBucketCommand,
    ListObjectsV2Command,
    PutBucketVersioningCommand,
    BucketVersioningStatus,
} from '@aws-sdk/client-s3';
import { SignatureV4 } from '@smithy/signature-v4';

import {
    Bucket,
    BucketCursor,
    BucketPage,
    BucketsApi,
    BucketMetadata,
} from '@/types/buckets';
import { BucketsHttpApi } from '@/api/buckets';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { Duration } from '@/utils/time';

const FIRST_PAGE = 1;

export class BucketsState {
    public allBucketNames: string[] = [];
    public allBucketMetadata: BucketMetadata[] = [];
    public cursor: BucketCursor = { limit: DEFAULT_PAGE_LIMIT, search: '', page: FIRST_PAGE };
    public page: BucketPage = { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: DEFAULT_PAGE_LIMIT, search: '', totalCount: 0 };
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForDelete: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForCreate: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForVersioning: EdgeCredentials = new EdgeCredentials();
    public s3Client: S3Client = new S3Client({
        forcePathStyle: true,
        signerConstructor: SignatureV4,
    });
    public s3ClientForDelete: S3Client = new S3Client({
        forcePathStyle: true,
        signerConstructor: SignatureV4,
    });
    public s3ClientForCreate: S3Client = new S3Client({
        forcePathStyle: true,
        signerConstructor: SignatureV4,
    });
    public s3ClientForVersioning: S3Client = new S3Client({
        forcePathStyle: true,
        signerConstructor: SignatureV4,
    });
    public apiKey = '';
    public passphrase = '';
    public promptForPassphrase = true;
    public fileComponentBucketName = '';
    public fileComponentPath = '';
    public leaveRoute = '';
    public enterPassphraseCallback: (() => void) | null = null;
    public bucketsBeingDeleted: Set<string> = new Set<string>();
}

export const useBucketsStore = defineStore('buckets', () => {
    const state = reactive<BucketsState>(new BucketsState());

    const api: BucketsApi = new BucketsHttpApi();

    function setBucketsSearch(search: string): void {
        state.cursor.search = search;
    }

    async function getBuckets(page: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
        const before = new Date();
        state.cursor.page = page;
        state.cursor.limit = limit;

        state.page = await api.get(projectID, before, state.cursor);
    }

    async function getAllBucketsNames(projectID: string): Promise<void> {
        state.allBucketNames = await api.getAllBucketNames(projectID);
    }

    async function getAllBucketsMetadata(projectID: string): Promise<void> {
        state.allBucketMetadata = await api.getAllBucketMetadata(projectID);
    }

    function setPromptForPassphrase(value: boolean): void {
        state.promptForPassphrase = value;
    }

    function setApiKey(apiKey: string): void {
        state.apiKey = apiKey;
    }

    function setEdgeCredentials(credentials: EdgeCredentials): void {
        state.edgeCredentials = credentials;
    }

    function setEdgeCredentialsForDelete(credentials: EdgeCredentials, forceDeleteDisabled = false): void {
        state.edgeCredentialsForDelete = credentials;

        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: state.edgeCredentialsForDelete.accessKeyId || '',
                secretAccessKey: state.edgeCredentialsForDelete.secretKey || '',
            },
            endpoint: state.edgeCredentialsForDelete.endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3ClientForDelete = new S3Client(s3Config);

        if (!forceDeleteDisabled) {
            state.s3ClientForDelete.middlewareStack.add(
                (next, _) => (args) => {
                    (args.request as { headers: { key: string } }).headers['x-minio-force-delete'] = 'true';
                    return next(args);
                },
                { step: 'build' },
            );
        }
    }

    function setEdgeCredentialsForCreate(credentials: EdgeCredentials): void {
        state.edgeCredentialsForCreate = credentials;

        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: state.edgeCredentialsForCreate.accessKeyId || '',
                secretAccessKey: state.edgeCredentialsForCreate.secretKey || '',
            },
            endpoint: state.edgeCredentialsForCreate.endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3ClientForCreate = new S3Client(s3Config);
    }

    function setEdgeCredentialsForVersioning(credentials: EdgeCredentials): void {
        state.edgeCredentialsForVersioning = credentials;

        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: state.edgeCredentialsForVersioning.accessKeyId || '',
                secretAccessKey: state.edgeCredentialsForVersioning.secretKey || '',
            },
            endpoint: state.edgeCredentialsForVersioning.endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3ClientForVersioning = new S3Client(s3Config);
    }

    async function setS3Client(projectID: string): Promise<void> {
        const agStore = useAccessGrantsStore();
        const { objectBrowserKeyNamePrefix, objectBrowserKeyLifetime } = useConfigStore().state.config;
        const now = new Date();

        if (!state.apiKey) {
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(`${objectBrowserKeyNamePrefix}${now.getTime()}`, projectID);
            setApiKey(cleanAPIKey.secret);
        }

        const worker = agStore.state.accessGrantsWebWorker;
        if (!worker) {
            throw new Error('Worker is not defined');
        }

        worker.onerror = (error: ErrorEvent) => {
            throw new Error(error.message);
        };

        const notAfter = new Date(now.setDate(now.getDate() + new Duration(objectBrowserKeyLifetime).days));
        worker.postMessage({
            'type': 'SetPermission',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'notAfter': notAfter.toISOString(),
            'buckets': JSON.stringify([]),
            'apiKey': state.apiKey,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        const projectsStore = useProjectsStore();
        const configStore = useConfigStore();

        const salt = await projectsStore.getProjectSalt(projectID);
        const satelliteNodeURL: string = configStore.state.config.satelliteNodeURL;

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
        state.edgeCredentials = await agStore.getEdgeCredentials(accessGrant);

        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: state.edgeCredentials.accessKeyId || '',
                secretAccessKey: state.edgeCredentials.secretKey || '',
            },
            endpoint: state.edgeCredentials.endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3Client = new S3Client(s3Config);
    }

    function setPassphrase(passphrase: string): void {
        state.passphrase = passphrase;
    }

    function setFileComponentBucketName(bucketName: string): void {
        state.fileComponentBucketName = bucketName;
    }

    function setFileComponentPath(path: string): void {
        state.fileComponentPath = path;
    }

    function setEnterPassphraseCallback(fn: (() => void) | null): void {
        state.enterPassphraseCallback = fn;
    }

    async function createBucket(name: string, enableObjectLock: boolean, enableBucketVersioning: boolean): Promise<void> {
        await state.s3Client.send(new CreateBucketCommand({
            Bucket: name,
            ObjectLockEnabledForBucket: enableObjectLock,
        }));
        // If object lock is enabled, versioning is enabled implicitly.
        if (enableBucketVersioning && !enableObjectLock) {
            await state.s3Client.send(new PutBucketVersioningCommand({
                Bucket: name,
                VersioningConfiguration: {
                    Status: BucketVersioningStatus.Enabled,
                },
            }));
        }
    }

    async function createBucketWithNoPassphrase(name: string, enableObjectLock: boolean, enableBucketVersioning: boolean): Promise<void> {
        await state.s3ClientForCreate.send(new CreateBucketCommand({
            Bucket: name,
            ObjectLockEnabledForBucket: enableObjectLock,
        }));
        // If object lock is enabled, versioning is enabled implicitly.
        if (enableBucketVersioning && !enableObjectLock) {
            await state.s3ClientForCreate.send(new PutBucketVersioningCommand({
                Bucket: name,
                VersioningConfiguration: {
                    Status: BucketVersioningStatus.Enabled,
                },
            }));
        }
    }

    async function setVersioning(bucket: string, enable: boolean): Promise<void> {
        await state.s3ClientForVersioning.send(new PutBucketVersioningCommand({
            Bucket: bucket,
            VersioningConfiguration: {
                Status: enable ? BucketVersioningStatus.Enabled : BucketVersioningStatus.Suspended,
            },
        }));
    }

    /**
     * This is an empty action for App.vue to subscribe to know the status of the delete bucket request.
     *
     * @param bucketName - the bucket name.
     * @param deleteRequest - the promise of the delete bucket request.
     */
    function handleDeleteBucketRequest(bucketName: string, deleteRequest: Promise<void>): void {
        /* empty */
    }

    async function deleteBucket(name: string): Promise<void> {
        state.bucketsBeingDeleted.add(name);
        try {
            await state.s3ClientForDelete.send(new DeleteBucketCommand({
                Bucket: name,
            }));
        } finally {
            state.bucketsBeingDeleted.delete(name);
        }
    }

    async function getObjectsCount(name: string): Promise<number> {
        const response = await state.s3Client.send(new ListObjectsV2Command({
            Bucket: name,
            MaxKeys: 1, // We need to know if there is at least 1 decryptable object.
        }));

        return (!response || response.KeyCount === undefined) ? 0 : response.KeyCount;
    }

    function clearS3Data(): void {
        state.apiKey = '';
        state.passphrase = '';
        state.promptForPassphrase = true;
        state.edgeCredentials = new EdgeCredentials();
        state.edgeCredentialsForDelete = new EdgeCredentials();
        state.edgeCredentialsForCreate = new EdgeCredentials();
        state.edgeCredentialsForVersioning = new EdgeCredentials();
        state.s3Client = new S3Client({
            forcePathStyle: true,
            signerConstructor: SignatureV4,
        });
        state.s3ClientForDelete = new S3Client({
            forcePathStyle: true,
            signerConstructor: SignatureV4,
        });
        state.s3ClientForCreate = new S3Client({
            forcePathStyle: true,
            signerConstructor: SignatureV4,
        });
        state.s3ClientForVersioning = new S3Client({
            forcePathStyle: true,
            signerConstructor: SignatureV4,
        });
        state.fileComponentBucketName = '';
        state.leaveRoute = '';
        state.bucketsBeingDeleted.clear();
    }

    function clear(): void {
        state.allBucketNames = [];
        state.cursor = new BucketCursor('', DEFAULT_PAGE_LIMIT, FIRST_PAGE);
        state.page = new BucketPage([], '', DEFAULT_PAGE_LIMIT, 0, 1, 1, 0);
        state.enterPassphraseCallback = null;
        clearS3Data();
    }

    return {
        state,
        setBucketsSearch,
        getBuckets,
        getAllBucketsNames,
        getAllBucketsMetadata,
        setPromptForPassphrase,
        setEdgeCredentials,
        setEdgeCredentialsForDelete,
        setEdgeCredentialsForCreate,
        setEdgeCredentialsForVersioning,
        setS3Client,
        setPassphrase,
        setApiKey,
        setFileComponentBucketName,
        setFileComponentPath,
        setEnterPassphraseCallback,
        createBucket,
        createBucketWithNoPassphrase,
        setVersioning,
        deleteBucket,
        handleDeleteBucketRequest,
        getObjectsCount,
        clearS3Data,
        clear,
    };
});
