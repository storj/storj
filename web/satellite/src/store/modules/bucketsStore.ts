// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';
import {
    S3Client,
    S3ClientConfig,
    BucketLocationConstraint,
    CreateBucketCommand,
    DeleteBucketCommand,
    ListObjectsV2Command,
    PutBucketVersioningCommand,
    BucketVersioningStatus,
    PutObjectLockConfigurationCommand,
    ObjectLockRule,
    ListObjectVersionsCommandInput,
    ListObjectVersionsCommandOutput,
    ListObjectVersionsCommand,
} from '@aws-sdk/client-s3';
import { SignatureV4 } from '@smithy/signature-v4';

import {
    Bucket,
    BucketCursor,
    BucketPage,
    BucketsApi,
    BucketMetadata, PlacementDetails,
} from '@/types/buckets';
import { BucketsHttpApi } from '@/api/buckets';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { Duration } from '@/utils/time';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';

const FIRST_PAGE = 1;

export enum ClientType {
    REGULAR,
    FOR_CREATE,
    FOR_OBJECT_LOCK,
}

export class BucketsState {
    public allBucketNames: string[] = [];
    public allBucketMetadata: BucketMetadata[] = [];
    public cursor: BucketCursor = { limit: DEFAULT_PAGE_LIMIT, search: '', page: FIRST_PAGE };
    public page: BucketPage = { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: DEFAULT_PAGE_LIMIT, search: '', totalCount: 0 };
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForDelete: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForCreate: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForVersioning: EdgeCredentials = new EdgeCredentials();
    public edgeCredentialsForObjectLock: EdgeCredentials = new EdgeCredentials();
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
    public s3ClientForObjectLock: S3Client = new S3Client({
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

    const { setPermissions, generateAccess } = useAccessGrantWorker();

    function setBucketsSearch(search: string): void {
        state.cursor.search = search;
    }

    async function getBuckets(page: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
        const now = new Date();
        const since = new Date(Date.UTC(
            now.getUTCFullYear(),
            now.getUTCMonth(),
            1,
            0, 0, 0, 0,
        ));

        state.cursor.page = page;
        state.cursor.limit = limit;

        state.page = await api.get(projectID, since, now, state.cursor);
    }

    async function getSingleBucket(projectID: string, bucketName: string): Promise<Bucket> {
        const before = new Date();

        return await api.getSingle(projectID, bucketName, before);
    }

    async function getAllBucketsNames(projectID: string): Promise<void> {
        state.allBucketNames = await api.getAllBucketNames(projectID);
    }

    async function getAllBucketsMetadata(projectID: string): Promise<void> {
        state.allBucketMetadata = await api.getAllBucketMetadata(projectID);
    }

    async function getPlacementDetails(projectID: string): Promise<PlacementDetails[]> {
        return await api.getPlacementDetails(projectID);
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

    function setEdgeCredentialsForObjectLock(credentials: EdgeCredentials): void {
        state.edgeCredentialsForObjectLock = credentials;

        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: state.edgeCredentialsForObjectLock.accessKeyId || '',
                secretAccessKey: state.edgeCredentialsForObjectLock.secretKey || '',
            },
            endpoint: state.edgeCredentialsForObjectLock.endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3ClientForObjectLock = new S3Client(s3Config);
    }

    async function setS3Client(projectID: string): Promise<void> {
        if (!state.passphrase) throw new Error('Passphrase can\'t be empty');

        const agStore = useAccessGrantsStore();
        const { objectBrowserKeyNamePrefix, objectBrowserKeyLifetime } = useConfigStore().state.config;
        const now = new Date();

        if (!state.apiKey) {
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(`${objectBrowserKeyNamePrefix}${now.getTime()}`, projectID);
            setApiKey(cleanAPIKey.secret);
        }

        const notAfter = new Date(now.setDate(now.getDate() + new Duration(objectBrowserKeyLifetime).days));

        const macaroon = await setPermissions({
            isDownload: true,
            isUpload: true,
            isList: true,
            isDelete: true,
            isPutObjectRetention: true,
            isGetObjectRetention: true,
            isPutObjectLegalHold: true,
            isGetObjectLegalHold: true,
            isPutObjectLockConfiguration: true,
            isGetObjectLockConfiguration: true,
            notAfter: notAfter.toISOString(),
            buckets: JSON.stringify([]),
            apiKey: state.apiKey,
        });

        const accessGrant = await generateAccess({
            apiKey: macaroon,
            passphrase: state.passphrase,
        }, projectID);

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

    async function createBucket(params: {
        name: string, enableObjectLock: boolean,
        enableVersioning: boolean,
        placementName?: string,
    }): Promise<void> {
        await state.s3Client.send(new CreateBucketCommand({
            Bucket: params.name,
            ObjectLockEnabledForBucket: params.enableObjectLock,
            CreateBucketConfiguration: {
                LocationConstraint: params.placementName as BucketLocationConstraint,
            },
        }));
        // If object lock is enabled, versioning is enabled implicitly.
        if (params.enableVersioning && !params.enableObjectLock) {
            await state.s3Client.send(new PutBucketVersioningCommand({
                Bucket: params.name,
                VersioningConfiguration: {
                    Status: BucketVersioningStatus.Enabled,
                },
            }));
        }
    }

    async function setObjectLockConfig(name: string, clientType: ClientType, rule?: ObjectLockRule): Promise<void> {
        let client: S3Client = state.s3Client;
        if (clientType === ClientType.FOR_CREATE) {
            client = state.s3ClientForCreate;
        } else if (clientType === ClientType.FOR_OBJECT_LOCK) {
            client = state.s3ClientForObjectLock;
        }

        await client.send(new PutObjectLockConfigurationCommand({
            Bucket: name,
            ObjectLockConfiguration: {
                ObjectLockEnabled: 'Enabled',
                Rule: rule,
            },
        }));
    }

    async function createBucketWithNoPassphrase(params: {
        name: string,
        enableObjectLock: boolean,
        enableVersioning: boolean,
        placementName?: string,
    }): Promise<void> {
        await state.s3ClientForCreate.send(new CreateBucketCommand({
            Bucket: params.name,
            ObjectLockEnabledForBucket: params.enableObjectLock,
            CreateBucketConfiguration: {
                LocationConstraint: params.placementName as BucketLocationConstraint,
            },
        }));
        // If object lock is enabled, versioning is enabled implicitly.
        if (params.enableVersioning && !params.enableObjectLock) {
            await state.s3ClientForCreate.send(new PutBucketVersioningCommand({
                Bucket: params.name,
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
     * @param _bucketName - the bucket name.
     * @param _deleteRequest - the promise of the delete bucket request.
     */
    function handleDeleteBucketRequest(_bucketName: string, _deleteRequest: Promise<void>): void {
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

    async function checkBucketEmpty(name: string): Promise<boolean> {
        const input: ListObjectVersionsCommandInput = {
            Bucket: name,
            Delimiter: '/',
            Prefix: '',
            MaxKeys: 10,
        };

        const response: ListObjectVersionsCommandOutput = await state.s3ClientForDelete.send(new ListObjectVersionsCommand(input));
        return !(response.DeleteMarkers?.length || response.Versions?.length || response.CommonPrefixes?.length);
    }

    function clearS3Data(): void {
        state.apiKey = '';
        state.passphrase = '';
        state.promptForPassphrase = true;
        state.edgeCredentials = new EdgeCredentials();
        state.edgeCredentialsForDelete = new EdgeCredentials();
        state.edgeCredentialsForCreate = new EdgeCredentials();
        state.edgeCredentialsForVersioning = new EdgeCredentials();
        state.edgeCredentialsForObjectLock = new EdgeCredentials();
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
        state.s3ClientForObjectLock = new S3Client({
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
        getSingleBucket,
        getAllBucketsNames,
        getAllBucketsMetadata,
        getPlacementDetails,
        setPromptForPassphrase,
        setEdgeCredentials,
        setEdgeCredentialsForDelete,
        setEdgeCredentialsForCreate,
        setEdgeCredentialsForVersioning,
        setEdgeCredentialsForObjectLock,
        setObjectLockConfig,
        setS3Client,
        setPassphrase,
        setApiKey,
        setFileComponentBucketName,
        setFileComponentPath,
        createBucket,
        createBucketWithNoPassphrase,
        setVersioning,
        deleteBucket,
        handleDeleteBucketRequest,
        getObjectsCount,
        checkBucketEmpty,
        clearS3Data,
        clear,
    };
});
