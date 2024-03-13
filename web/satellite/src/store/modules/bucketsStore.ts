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

import { Bucket, BucketCursor, BucketPlacement, BucketPage, BucketsApi } from '@/types/buckets';
import { BucketsHttpApi } from '@/api/buckets';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

const FIRST_PAGE = 1;
export const FILE_BROWSER_AG_NAME = 'Web file browser API key';

export class BucketsState {
    public allBucketNames: string[] = [];
    public allBucketPlacements: BucketPlacement[] = [];
    public cursor: BucketCursor = { limit: DEFAULT_PAGE_LIMIT, search: '', page: FIRST_PAGE };
    public page: BucketPage = { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: DEFAULT_PAGE_LIMIT, search: '', totalCount: 0 };
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForDelete: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForCreate: EdgeCredentials = new EdgeCredentials();
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
    public apiKey = '';
    public passphrase = '';
    public promptForPassphrase = true;
    public fileComponentBucketName = '';
    public fileComponentPath = '';
    public leaveRoute = '';
    public enterPassphraseCallback: (() => void) | null = null;
    public bucketToDelete = '';
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

    async function getAllBucketsPlacements(projectID: string): Promise<void> {
        state.allBucketPlacements = await api.getAllBucketPlacements(projectID);
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

    function setEdgeCredentialsForDelete(credentials: EdgeCredentials): void {
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

        state.s3ClientForDelete.middlewareStack.add(
            (next, _) => (args) => {
                (args.request as { headers: { key: string } }).headers['x-minio-force-delete'] = 'true';
                return next(args);
            },
            { step: 'build' },
        );
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

    async function setS3Client(projectID: string): Promise<void> {
        const agStore = useAccessGrantsStore();

        if (!state.apiKey) {
            await agStore.deleteAccessGrantByNameAndProjectID(FILE_BROWSER_AG_NAME, projectID);
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(FILE_BROWSER_AG_NAME, projectID);
            setApiKey(cleanAPIKey.secret);
        }

        const now = new Date();
        const inThreeDays = new Date(now.setDate(now.getDate() + 3));

        const worker = agStore.state.accessGrantsWebWorker;
        if (!worker) {
            throw new Error('Worker is not defined');
        }

        worker.onerror = (error: ErrorEvent) => {
            throw new Error(error.message);
        };

        worker.postMessage({
            'type': 'SetPermission',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'notAfter': inThreeDays.toISOString(),
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

    async function createBucket(name: string, enableBucketVersioning: boolean): Promise<void> {
        await state.s3Client.send(new CreateBucketCommand({
            Bucket: name,
        }));
        if (enableBucketVersioning) {
            await state.s3Client.send(new PutBucketVersioningCommand({
                Bucket: name,
                VersioningConfiguration: {
                    Status: BucketVersioningStatus.Enabled,
                },
            }));
        }
    }

    async function createBucketWithNoPassphrase(name: string, enableBucketVersioning: boolean): Promise<void> {
        await state.s3ClientForCreate.send(new CreateBucketCommand({
            Bucket: name,
        }));
        if (enableBucketVersioning) {
            await state.s3ClientForCreate.send(new PutBucketVersioningCommand({
                Bucket: name,
                VersioningConfiguration: {
                    Status: BucketVersioningStatus.Enabled,
                },
            }));
        }
    }

    async function deleteBucket(name: string): Promise<void> {
        await state.s3ClientForDelete.send(new DeleteBucketCommand({
            Bucket: name,
        }));
    }

    async function getObjectsCount(name: string): Promise<number> {
        const response = await state.s3Client.send(new ListObjectsV2Command({
            Bucket: name,
            MaxKeys: 1, // We need to know if there is at least 1 decryptable object.
        }));

        return (!response || response.KeyCount === undefined) ? 0 : response.KeyCount;
    }

    function setBucketToDelete(bucket: string): void {
        state.bucketToDelete = bucket;
    }

    function clearS3Data(): void {
        state.apiKey = '';
        state.passphrase = '';
        state.promptForPassphrase = true;
        state.edgeCredentials = new EdgeCredentials();
        state.edgeCredentialsForDelete = new EdgeCredentials();
        state.edgeCredentialsForCreate = new EdgeCredentials();
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
        state.fileComponentBucketName = '';
        state.leaveRoute = '';
        state.bucketToDelete = '';
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
        getAllBucketsPlacements,
        setPromptForPassphrase,
        setEdgeCredentials,
        setEdgeCredentialsForDelete,
        setEdgeCredentialsForCreate,
        setS3Client,
        setPassphrase,
        setApiKey,
        setFileComponentBucketName,
        setFileComponentPath,
        setEnterPassphraseCallback,
        createBucket,
        createBucketWithNoPassphrase,
        deleteBucket,
        getObjectsCount,
        setBucketToDelete,
        clearS3Data,
        clear,
    };
});
