// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive, UnwrapNestedRefs, h } from 'vue';
import { defineStore } from 'pinia';
import {
    _Object,
    CommonPrefix,
    CopyObjectCommand,
    DeleteObjectCommand,
    GetObjectCommand,
    ListObjectsCommand,
    ListObjectsV2Command,
    ListObjectsV2CommandInput,
    ListObjectsV2CommandOutput,
    ListObjectVersionsCommand,
    ListObjectVersionsCommandInput,
    ListObjectVersionsCommandOutput,
    paginateListObjectsV2,
    PutObjectCommand,
    PutObjectRetentionCommand,
    S3Client,
    S3ClientConfig,
    GetObjectRetentionCommand,
    GetObjectRetentionCommandOutput,
    ObjectVersion,
    DeleteMarkerEntry,
    DeleteObjectsCommand,
    ObjectLockMode,
    PutObjectLegalHoldCommand,
    GetObjectLegalHoldCommand,
    ObjectLockLegalHoldStatus,
} from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';
import { Progress, Upload } from '@aws-sdk/lib-storage';
import { SignatureV4 } from '@smithy/signature-v4';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { ObjectDeleteError } from '@/utils/error';
import { useConfigStore } from '@/store/modules/configStore';
import { LocalData } from '@/utils/localData';
import { ObjectLockStatus, Retention } from '@/types/objectLock';
import { NotifyRenderedUplinkCLIMessage } from '@/types/DelayedNotification';

export type BrowserObject = {
    Key: string;
    VersionId?: string;
    Size: number;
    LastModified: Date;
    type?: 'file' | 'folder';
    isDeleteMarker?: boolean;
    isLatest?: boolean;
    progress?: number;
    upload?: {
        abort: () => void;
    };
    path?: string;
    Versions?: BrowserObject[];
};

export type FullBrowserObject = BrowserObject & {
    retention?: Retention;
    legalHold?: boolean;
};

export enum FailedUploadMessage {
    Failed = 'Upload failed',
    TooBig = 'File is too big',
}

export enum UploadingStatus {
    InProgress,
    Finished,
    Failed,
    Cancelled,
}

export type UploadingBrowserObject = BrowserObject & {
    status: UploadingStatus;
    Bucket: string;
    Body: File;
    failedMessage?: FailedUploadMessage;
};

export type PreviewCache = {
    url: string,
    lastModified: number,
};

export const MAX_KEY_COUNT = 500;

export type ObjectBrowserCursor = {
    page: number,
    limit: number,
};

export type ObjectRange = {
    start: number,
    end: number,
};

export type FileToUpload = {
    path: string,
    file: File,
};

export class FilesState {
    s3: S3Client | null = null;
    accessKey: null | string = null;
    path = '';
    bucket = '';
    browserRoot = '/';
    files: BrowserObject[] = [];
    cursor: ObjectBrowserCursor = { limit: DEFAULT_PAGE_LIMIT, page: 1 };
    continuationTokens: Map<number, string> = new Map<number, string>();
    totalObjectCount = 0;
    activeObjectsRange: ObjectRange = { start: 1, end: 500 };
    uploadChain: Promise<void> = Promise.resolve();
    uploading: UploadingBrowserObject[] = [];
    selectedFiles: BrowserObject[] = [];
    filesToBeDeleted: Set<string> = new Set<string>();
    openedDropdown: null | string = null;
    headingSorted = 'name';
    orderBy: 'asc' | 'desc' = 'asc';
    openModalOnFirstUpload = false;
    objectPathForModal = '';
    cachedObjectPreviewURLs: Map<string, PreviewCache> = new Map<string, PreviewCache>();
    showObjectVersions = { value: false, userModified: false };
    // object keys for which we have expanded versions list.
    versionsExpandedKeys: string[] = [];
    // Local storage data changes are not reactive.
    // So we need to store this info here to make sure components rerender on changes.
    objectCountOfSelectedBucket = LocalData.getObjectCountOfSelectedBucket() ?? 0;

    largeFileNotificationTimeout: ReturnType<typeof setTimeout> | undefined;
}

type InitializedFilesState = FilesState & {
    s3: S3Client;
};

function assertIsInitialized(
    state: UnwrapNestedRefs<FilesState>,
): asserts state is InitializedFilesState {
    if (state.s3 === null) {
        throw new Error(
            'FilesModule: S3 Client is uninitialized. "state.s3" is null.',
        );
    }
}

declare global {
    interface FileSystemEntry {
        // https://developer.mozilla.org/en-US/docs/Web/API/FileSystemFileEntry/file
        file: (
            successCallback: (arg0: File) => void,
            errorCallback?: (arg0: Error) => void
        ) => void;
        createReader: () => FileSystemDirectoryReader;
    }
}

export const useObjectBrowserStore = defineStore('objectBrowser', () => {
    const state = reactive<FilesState>(new FilesState());

    const configStore = useConfigStore();
    const appStore = useAppStore();
    const { notifyError, notifyWarning } = useNotificationsStore();

    // TODO: replace a hard-coded value with a config value?
    const isAltPagination = computed<boolean>(() => {
        return configStore.state.config.altObjBrowserPagingEnabled &&
            state.objectCountOfSelectedBucket > configStore.state.config.altObjBrowserPagingThreshold;
    });

    const sortedFiles = computed(() => {
        // key-specific sort cases
        const fns = {
            date: (a: BrowserObject, b: BrowserObject): number =>
                new Date(a.LastModified).getTime() - new Date(b.LastModified).getTime(),
            name: (a: BrowserObject, b: BrowserObject): number =>
                a.Key.localeCompare(b.Key),
            size: (a: BrowserObject, b: BrowserObject): number => a.Size - b.Size,
        };

        // TODO(performance): avoid several passes over the slice.

        // sort by appropriate function
        const sortedFiles = state.files.slice();
        sortedFiles.sort(fns[state.headingSorted]);
        // reverse if descending order
        if (state.orderBy !== 'asc') {
            sortedFiles.reverse();
        }

        // display folders and then files
        return [
            ...sortedFiles.filter((file) => file.type === 'folder'),
            ...sortedFiles.filter((file) => file.type === 'file'),
        ];
    });

    const displayedObjects = computed(() => {
        let end = state.cursor.limit * state.cursor.page;
        let start = end - state.cursor.limit;

        // We check if current active range is not initial and recalculate slice indexes.
        if (state.activeObjectsRange.end !== MAX_KEY_COUNT) {
            end -= state.activeObjectsRange.start;
            start = end - state.cursor.limit;
        }

        return sortedFiles.value.slice(start, end);
    });

    const isInitialized = computed(() => {
        return state.s3 !== null;
    });

    const uploadingLength = computed(() => {
        return state.uploading.filter(f => f.status === UploadingStatus.InProgress).length;
    });

    function setCursor(cursor: ObjectBrowserCursor): void {
        state.cursor = cursor;
    }

    function init({
        accessKey,
        secretKey,
        bucket,
        endpoint,
        browserRoot,
        openModalOnFirstUpload = true,
    }: {
        accessKey: string;
        secretKey: string;
        bucket: string;
        endpoint: string;
        browserRoot: string;
        openModalOnFirstUpload?: boolean;
    }): void {
        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: accessKey,
                secretAccessKey: secretKey,
            },
            endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.s3 = new S3Client(s3Config);
        state.accessKey = accessKey;
        state.bucket = bucket;
        state.browserRoot = browserRoot;
        state.openModalOnFirstUpload = openModalOnFirstUpload;
        state.path = '';
        state.files = [];
    }

    function reinit({
        accessKey,
        secretKey,
        endpoint,
    }: {
        accessKey: string;
        secretKey: string;
        endpoint: string;
    }): void {
        const s3Config: S3ClientConfig = {
            credentials: {
                accessKeyId: accessKey,
                secretAccessKey: secretKey,
            },
            endpoint,
            forcePathStyle: true,
            signerConstructor: SignatureV4,
            region: 'us-east-1',
        };

        state.files = [];
        state.s3 = new S3Client(s3Config);
        state.accessKey = accessKey;
    }

    function updateFiles(path: string, files: BrowserObject[]): void {
        state.path = path;
        state.files = files;
    }

    function updateVersionsExpandedKeys(keys: string[]): void {
        state.versionsExpandedKeys = keys;
    }

    const isFileVisible = (file) =>
        file.Key.length > 0 && !file.Key?.includes('.file_placeholder');

    type DefinedCommonPrefix = CommonPrefix & {
        Prefix: string;
    };
    const isPrefixDefined = (
        value: CommonPrefix,
    ): value is DefinedCommonPrefix => value.Prefix !== undefined;

    function prefixToFolder(path: string) {
        return ({
            Prefix,
        }: {
            Prefix: string;
        }): BrowserObject => ({
            Key: Prefix.slice(path.length, -1),
            path: path,
            LastModified: new Date(),
            Size: 0,
            type: 'folder',
        });
    }

    function makeFileRelative(path: string) {
        return (file) => ({
            ...file,
            Key: file.Key.slice(path.length),
            path: path,
            type: 'file',
        });
    }

    async function countVersions(objectKey: string, maxKeys = 500): Promise<string> {
        assertIsInitialized(state);
        const response = await state.s3.send(new ListObjectVersionsCommand({
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: objectKey,
            MaxKeys: maxKeys,
        }));
        const { Versions, DeleteMarkers, CommonPrefixes } = response;
        const allVersions = [...Versions ?? [], ...DeleteMarkers ?? []].filter(isFileVisible);

        const listedCount = `${allVersions.length}`;
        if (response.IsTruncated || (CommonPrefixes?.length ?? 0) > 0) {
            return `${listedCount}+`;
        }
        return listedCount;
    }

    async function listAllVersions(path = state.path, page = state.cursor.page, saveNextToken = false) {
        assertIsInitialized(state);

        const continuationToken = state.continuationTokens.get(page);
        let nextKey: string = '';
        let nextVersion: string = '';
        if (continuationToken) {
            [nextKey, nextVersion] = continuationToken.split('::');
        }

        state.cursor.page = page;
        const input: ListObjectVersionsCommandInput = {
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: path,
            KeyMarker: nextKey,
            VersionIdMarker: nextVersion,
            MaxKeys: state.cursor.limit,
        };

        const response: ListObjectVersionsCommandOutput = await state.s3.send(new ListObjectVersionsCommand(input));

        const versions = response.Versions ?? [];
        const deleteMarkers = response.DeleteMarkers ?? [];
        const allItems = [...versions, ...deleteMarkers];
        const groupedItems = new Map<string, BrowserObject[]>();

        for (let item of allItems) {
            item = makeFileRelative(path)(item);
            if (!isFileVisible(item)) {
                continue;
            }

            if (!groupedItems.has(item.Key ?? '')) {
                groupedItems.set(item.Key ?? '', []);
            }

            let size = 0;
            let isDeleteMarker = true;
            let isLatest = false;
            if ('Size' in item) {
                size = (item.Size as number) ?? 0;
                isDeleteMarker = false;
                isLatest = item.IsLatest ?? false;
            }
            const browserObject: BrowserObject = {
                Key: item.Key ?? '',
                path: path,
                VersionId: item.VersionId,
                Size: size,
                LastModified: item.LastModified ?? new Date(),
                isLatest: isLatest,
                type: 'file',
                isDeleteMarker,
            };

            groupedItems.get(item.Key ?? '')?.push(browserObject);
        }

        const latestObjects: BrowserObject[] = [];
        const keys: string[] = [];
        for (const [key, items] of groupedItems.entries()) {
            items.sort((a, b) => new Date(b.LastModified ?? 0).getTime() - new Date(a.LastModified ?? 0).getTime());
            const item = items[0];
            keys.push(item.path + item.Key);
            latestObjects.push({
                Key: key,
                Size: item.Size,
                path: item.path,
                type: item.type,
                Versions: items,
                LastModified: item.LastModified,
                isDeleteMarker: item.isDeleteMarker,
            });
        }
        updateVersionsExpandedKeys(keys);

        if (saveNextToken) {
            const nextToken = `${response.NextKeyMarker ?? ''}::${response.NextVersionIdMarker ?? ''}`;
            if (nextToken !== '::') {
                state.continuationTokens.set(page + 1, nextToken);
            }
        }

        state.path = path;
        const folders = response.CommonPrefixes ?? [];
        updateFiles(path, [
            ...folders.filter(isPrefixDefined).map(prefixToFolder(path)),
            ...latestObjects,
        ]);
    }

    async function initList(path = state.path): Promise<void> {
        assertIsInitialized(state);

        const input: ListObjectsV2CommandInput = {
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: path,
        };

        const paginator = paginateListObjectsV2({ client: state.s3, pageSize: MAX_KEY_COUNT }, input);

        let iteration = 1;
        let keyCount = 0;

        for await (const response of paginator) {
            if (iteration === 1) {
                const { Contents, CommonPrefixes } = response;

                processFetchedObjects(path, Contents, CommonPrefixes);

                state.activeObjectsRange = { start: 1, end: MAX_KEY_COUNT };
            }

            keyCount += response.KeyCount ?? 0;

            if (!response.NextContinuationToken) break;

            state.continuationTokens.set(MAX_KEY_COUNT * (iteration + 1), response.NextContinuationToken);
            iteration++;
        }

        // We decrement key count if we're inside a folder to exclude .file_placeholder object
        // which was auto created for this folder because it's not visible by the user
        // and it shouldn't be included in pagination process.
        if (path) {
            keyCount -= 1;
        }

        state.totalObjectCount = keyCount;
    }

    async function listByToken(path: string, key: number, continuationToken: string): Promise<void> {
        assertIsInitialized(state);

        const input: ListObjectsV2CommandInput = {
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: path,
            ContinuationToken: continuationToken,
        };

        const response = await state.s3.send(new ListObjectsV2Command(input));

        const { Contents, CommonPrefixes } = response;

        processFetchedObjects(path, Contents, CommonPrefixes);

        state.activeObjectsRange = { start: key - MAX_KEY_COUNT, end: key };
    }

    async function listCustom(path = state.path, page: number, saveNextToken = false): Promise<void> {
        assertIsInitialized(state);

        const continuationToken = state.continuationTokens.get(page);

        const input: ListObjectsV2CommandInput = {
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: path,
            ContinuationToken: continuationToken,
            MaxKeys: state.cursor.limit,
        };

        const response: ListObjectsV2CommandOutput = await state.s3.send(new ListObjectsV2Command(input));

        const { Contents, CommonPrefixes } = response;

        processFetchedObjects(path, Contents, CommonPrefixes);

        if (saveNextToken && response.NextContinuationToken) {
            state.continuationTokens.set(page + 1, response.NextContinuationToken);
        }

        state.cursor.page = page;
    }

    function processFetchedObjects(path: string, Contents: _Object[] | undefined, CommonPrefixes: CommonPrefix[] | undefined): void {
        if (Contents === undefined) {
            Contents = [];
        }

        if (CommonPrefixes === undefined) {
            CommonPrefixes = [];
        }

        Contents.sort((a, b) => {
            if (
                a === undefined ||
                a.LastModified === undefined ||
                b === undefined ||
                b.LastModified === undefined ||
                a.LastModified === b.LastModified
            ) {
                return 0;
            }

            return a.LastModified < b.LastModified ? -1 : 1;
        });

        const files: BrowserObject[] = [
            ...CommonPrefixes.filter(isPrefixDefined).map(prefixToFolder(path)),
            ...Contents.map(makeFileRelative(path)).filter(isFileVisible),
        ];

        updateFiles(path, files);
    }

    async function restoreObject(obj: BrowserObject): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new CopyObjectCommand({
            CopySource: `${state.bucket}/${obj.Key}?versionId=${obj.VersionId}`,
            Bucket: state.bucket,
            Key: obj.Key,
            MetadataDirective: 'REPLACE',
        }));
    }

    // Low effort check for duplicates in files to upload. If files checked reaches 100, or duplicates found reaches 5, return.
    function lazyDuplicateCheck(filesToUpload: FileToUpload[]): string[] {
        assertIsInitialized(state);

        const fileNames = state.files.map((file) => file.Key);
        const duplicateFiles: string[] = [];
        let traversedCount = 0;
        for (const { path, file } of filesToUpload) {
            const directories = path.split('/');
            const fileName = path + file.name;
            const hasDuplicate = fileNames.includes(directories[0]) || fileNames.includes(fileName);
            if (duplicateFiles.length < 5 && hasDuplicate) {
                duplicateFiles.push(fileName);
                if (duplicateFiles.length === 5) {
                    break;
                }
            }

            traversedCount++;
            if (traversedCount === 100) {
                break;
            }
        }
        return duplicateFiles;
    }

    async function getFilesToUpload({ e }: { e: DragEvent | Event }): Promise<FileToUpload[]> {
        assertIsInitialized(state);

        type Item = DataTransferItem | FileSystemEntry;
        type TraverseResult = { path: string, file: File };

        const items: Item[] = 'dataTransfer' in e && e.dataTransfer
            ? [...e.dataTransfer.items]
            : e.target !== null
                ? ((e.target as unknown) as { files: FileSystemEntry[] }).files
                : [];

        async function* traverse(item: Item | Item[], path = ''): AsyncGenerator<TraverseResult, void, void> {
            if ('isFile' in item && item.isFile) {
                const file = await new Promise(item.file.bind(item));
                yield { path, file };
            } else if (item instanceof File) {
                let relativePath = '';
                // on Firefox mobile, item.webkitRelativePath might be `undefined`
                if (item.webkitRelativePath) {
                    relativePath = item.webkitRelativePath
                        .split('/')
                        .slice(0, -1)
                        .join('/');
                }

                if (relativePath.length) {
                    relativePath += '/';
                }

                yield { path: relativePath, file: item };
            } else if ('isFile' in item && item.isDirectory) {
                const dirReader = item.createReader();

                const entries = await new Promise(
                    dirReader.readEntries.bind(dirReader),
                );

                for (const entry of entries) {
                    yield* traverse(
                        (entry as FileSystemEntry) as Item,
                        path + item.name + '/',
                    );
                }
            } else if ('length' in item) {
                for (const i of item) {
                    yield* traverse(i);
                }
            } else {
                throw new Error('Item is not directory or file');
            }
        }

        const isFileSystemEntry = (
            a: FileSystemEntry | null,
        ): a is FileSystemEntry => a !== null;

        const iterator = [...items]
            .map((item) =>
                'webkitGetAsEntry' in item ? item.webkitGetAsEntry() : item,
            )
            .filter(isFileSystemEntry) as FileSystemEntry[];

        const files: FileToUpload[] = [];
        for await (const { path, file } of traverse(iterator)) {
            files.push({ path, file });
        }
        return files;
    }

    async function upload(files: FileToUpload[]): Promise<void> {
        for await (const { path, file } of files) {
            const directories = path.split('/');
            const fileName = directories.join('/') + file.name;
            const key = state.path + fileName;

            await enqueueUpload(key, file);
        }
    }

    async function enqueueUpload(key: string, body: File): Promise<void> {
        assertIsInitialized(state);

        const params = {
            Bucket: state.bucket,
            Key: key,
            Body: body,
            ContentType: body.type,
        };

        if (state.uploading.some(f => f.Key === key && f.status === UploadingStatus.InProgress)) {
            notifyError(`${key} is already uploading`, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);
            return;
        }

        appStore.setUploadingModal(true);

        const index = state.uploading.findIndex(file => file.Key === key);
        if (index !== -1) {
            state.uploading.splice(index, 1);
        }

        // If file size exceeds 30 GB, abort the upload attempt
        if (body.size > (30 * 1024 * 1024 * 1024)) {
            state.uploading.push({
                ...params,
                progress: 0,
                Size: body.size,
                LastModified: new Date(),
                Body: body,
                status: UploadingStatus.Failed,
                failedMessage: FailedUploadMessage.TooBig,
                type: 'file',
            });

            notifyError(() => {
                return [
                    h('span', {}, `${key}: To upload files above 30GB, please use the `),
                    ...NotifyRenderedUplinkCLIMessage,
                ];
            }, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);

            return;
        }

        // Upload 4 parts at a time.
        const queueSize = 4;
        // For now use a 64mb part size. This may be configurable in the future to enhance performance.
        const partSize = 64 * 1024 * 1024;

        const upload = new Upload({
            client: state.s3,
            queueSize,
            partSize,
            params,
        });

        const progressListener = async (progress: Progress) => {
            const item = state.uploading.find(f => f.Key === key);
            if (!item) {
                upload.off('httpUploadProgress', progressListener);
                notifyError(
                    `Error updating progress. No file found with key '${key}'`,
                    AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR,
                );
                return;
            }

            let p = 0;
            if (progress.loaded && progress.total) {
                p = Math.round((progress.loaded / progress.total) * 100);
            }
            item.progress = p;
        };
        upload.on('httpUploadProgress', progressListener);

        state.uploading.push({
            ...params,
            upload,
            progress: 0,
            Size: body.size,
            LastModified: new Date(),
            status: UploadingStatus.InProgress,
            type: 'file',
        });

        state.uploadChain = state.uploadChain.then(async () => {
            const item = state.uploading.find(f => f.Key === key && f.status !== UploadingStatus.Cancelled);
            if (!item) return;

            try {
                await upload.done();
                item.status = UploadingStatus.Finished;
            } catch (error) {
                handleUploadError(item, error);
                return;
            } finally {
                upload.off('httpUploadProgress', progressListener);
            }

            if (state.showObjectVersions.value) {
                clearTokens();
                await listAllVersions(state.path, 1, true);
            } else if (isAltPagination.value) {
                clearTokens();
                await listCustom(state.path, 1, true);
            } else {
                await initList();
            }

            const uploadedFiles = state.files.filter(f => f.type === 'file');
            if (uploadedFiles.length === 1 && !key.includes('/') && state.openModalOnFirstUpload) {
                state.objectPathForModal = key;
            }
        });
    }

    function handleUploadError(item: UploadingBrowserObject, error: Error): void {
        if (error.name === 'AbortError' && item.status === UploadingStatus.Cancelled) return;

        item.status = UploadingStatus.Failed;
        item.failedMessage = FailedUploadMessage.Failed;

        const limitExceededError = 'storage limit exceeded';
        if (error.message.includes(limitExceededError)) {
            notifyError(`Error: ${limitExceededError}`, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);
        } else {
            notifyError(error.message, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);
        }

        if (item.Body.size > (1024 * 1024 * 1024)) {
            notifyWarning(() => {
                return [
                    h('span', {}, `To upload large files, please consider using the `),
                    ...NotifyRenderedUplinkCLIMessage,
                ];
            }, undefined, 10000);
        }
    }

    async function createFolder(name: string): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new PutObjectCommand({
            Bucket: state.bucket,
            Key: state.path + name + '/.file_placeholder',
            Body: '',
        }));

        if (state.showObjectVersions.value) {
            clearTokens();
            await listAllVersions(state.path, 1, true);
        } else if (isAltPagination.value) {
            clearTokens();
            listCustom(state.path, 1, true);
        } else {
            initList();
        }
    }

    async function lockObject(file: BrowserObject, mode: ObjectLockMode, until: Date): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new PutObjectRetentionCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file['VersionId'] ?? undefined,
            Retention: {
                Mode: mode,
                RetainUntilDate: until,
            },
        }));
    }

    async function legalHoldObject(file: BrowserObject, status: ObjectLockLegalHoldStatus): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new PutObjectLegalHoldCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file['VersionId'] ?? undefined,
            LegalHold: {
                Status: status,
            },
        }));
    }

    async function getObjectLegalHold(file: BrowserObject): Promise<boolean> {
        assertIsInitialized(state);

        const res = await state.s3.send(new GetObjectLegalHoldCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file['VersionId'] ?? undefined,
        }));

        return res?.LegalHold?.Status === ObjectLockLegalHoldStatus.ON;
    }

    async function getObjectRetention(file: BrowserObject): Promise<Retention> {
        assertIsInitialized(state);

        const response = await state.s3.send(new GetObjectRetentionCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file.VersionId,
        })).catch((error) => {
            if (
                error.message.includes('object retention not found')
                || error.message.includes('missing retention configuration')
                || error.message.includes('object does not have a retention configuration')
            ) {
                return {} as GetObjectRetentionCommandOutput;
            }
            return Promise.reject(error);
        });

        if (response.Retention && response.Retention.Mode && response.Retention.RetainUntilDate) {
            return new Retention(response.Retention.Mode, response.Retention.RetainUntilDate);
        }

        return Retention.empty();
    }

    async function getObjectLockStatus(file: BrowserObject): Promise<ObjectLockStatus> {
        assertIsInitialized(state);

        const results = await Promise.all([
            getObjectRetention(file),
            getObjectLegalHold(file),
        ]);

        return {
            retention: results[0],
            legalHold: results[1],
        };
    }

    async function bulkDeleteObjects(files: _Object[] | BrowserObject[]): Promise<number> {
        assertIsInitialized(state);

        addFileToBeDeleted(...files);

        try {
            const res = await state.s3.send(new DeleteObjectsCommand({
                Bucket: state.bucket,
                Delete: {
                    Objects: files.map((file: _Object | BrowserObject) => ({
                        // the file key may already be a full path
                        // in that case we don't need to prepend the path.
                        Key: file.Key?.includes('/') ? file.Key : state.path + file.Key,
                        VersionId: file['VersionId'] || undefined,
                    })),
                },
            }));

            let deletedCount = 0;
            for (const deleted of res.Deleted ?? []) {
                if (deleted.Key) deletedCount++;
            }
            if (res.Errors?.length && !!res.Errors[0].Message) {
                return Promise.reject(new ObjectDeleteError(deletedCount, res.Errors[0].Message));
            }
            return deletedCount;
        } catch (e) {
            return Promise.reject(new ObjectDeleteError(0, e.message));
        } finally {
            removeFileToBeDeleted(...files);
        }
    }

    /**
     * Delete all objects under a given path or prefix. If the prefix
     * is a folder, it will delete all objects under that folder.
     * If withVersions is true, it will delete all versions of the objects.
     */
    async function recursivelyDeleteObjects(pathWithKey: string, withVersions = false): Promise<number> {
        let deletedCount = 0;

        async function performDelete(prefix: string) {
            assertIsInitialized(state);

            let commonPrefixes: string[] = [];
            let isTruncated = true;
            let nextKey: string = '';
            let nextVersion: string = '';
            while (isTruncated) {
                let objects: ObjectVersion[] | _Object[] = [];
                let deleteMarkers: DeleteMarkerEntry[] = [];

                try {
                    if (withVersions) {
                        const response = await state.s3.send(new ListObjectVersionsCommand({
                            Bucket: state.bucket,
                            Delimiter: '/',
                            Prefix: prefix,
                            KeyMarker: nextKey || undefined,
                            VersionIdMarker: nextVersion || undefined,
                        }));
                        objects = response.Versions ?? [];
                        deleteMarkers = response.DeleteMarkers ?? [];
                        commonPrefixes.push(...response.CommonPrefixes?.map(prefix => prefix.Prefix ?? '') ?? []);

                        isTruncated = response.IsTruncated ?? false;
                        nextKey = response.NextKeyMarker ?? '';
                        nextVersion = response.NextVersionIdMarker ?? '';
                    } else {
                        const response = await state.s3.send(new ListObjectsCommand({
                            Bucket: state.bucket,
                            Delimiter: '/',
                            Prefix: prefix,
                            Marker: nextKey || undefined,
                        }));
                        objects = response.Contents ?? [];
                        commonPrefixes = response.CommonPrefixes?.map(prefix => prefix.Prefix ?? '') ?? [];

                        isTruncated = response.IsTruncated ?? false;
                        nextKey = response.NextMarker ?? '';
                    }
                } catch (e) {
                    return Promise.reject(new ObjectDeleteError(deletedCount, e.message));
                }

                const allObjects = [...objects, ...deleteMarkers];
                if (allObjects.length === 0) {
                    break;
                }
                deletedCount += await bulkDeleteObjects(allObjects);
            }

            for await (const prefix of commonPrefixes) {
                await performDelete(prefix);
            }
        }

        await performDelete(pathWithKey);

        return deletedCount;
    }

    async function deleteObjectWithVersions(path: string, file: BrowserObject): Promise<number> {
        assertIsInitialized(state);

        addFileToBeDeleted(file);

        try {
            const deletedCount = await recursivelyDeleteObjects(path + file.Key, true);

            state.uploading = state.uploading.filter(f => f.Key !== path + file.Key);
            return deletedCount;
        } catch (e) {
            return Promise.reject(e);
        } finally {
            removeFileToBeDeleted(file);
        }
    }

    async function deleteObject(path: string, file: BrowserObject): Promise<number> {
        assertIsInitialized(state);

        addFileToBeDeleted(file);

        try {
            await state.s3.send(new DeleteObjectCommand({
                Bucket: state.bucket,
                Key: path + file.Key,
                VersionId: file['VersionId'] ?? undefined,
            }));

            state.uploading = state.uploading.filter(f => f.Key !== path + file.Key);
            return 1;
        } catch (e) {
            return Promise.reject(new ObjectDeleteError(0, e.message));
        } finally {
            removeFileToBeDeleted(file);
        }
    }

    async function deleteFolderWithVersions(path: string, file: BrowserObject): Promise<number> {
        assertIsInitialized(state);

        addFileToBeDeleted(file);

        try {
            return recursivelyDeleteObjects(path.length > 0 ? path + file.Key : file.Key + '/', true);
        } catch (e) {
            return Promise.reject(e);
        } finally {
            removeFileToBeDeleted(file);
        }
    }

    async function deleteFolder(path: string, file: BrowserObject): Promise<number> {
        assertIsInitialized(state);

        addFileToBeDeleted(file);

        try {
            return await recursivelyDeleteObjects(path.length > 0 ? path + file.Key : file.Key + '/');
        } catch (e) {
            return Promise.reject(e);
        } finally {
            removeFileToBeDeleted(file);
        }
    }

    async function deleteSelected(withVersions = false): Promise<number> {
        addFileToBeDeleted(...state.selectedFiles);

        const promises: Promise<number>[] = [];
        if (!withVersions) {
            const files = state.selectedFiles.filter(file => file.type === 'file');
            const folders = state.selectedFiles.filter(file => file.type === 'folder');

            if (files.length) {
                promises.push(bulkDeleteObjects(files));
            }
            if (folders.length) {
                promises.push(...folders.map(selectedFile => deleteFolder(state.path, selectedFile)));
            }
        } else {
            promises.push(...state.selectedFiles.map(selectedFile => {
                if (selectedFile.type === 'file') {
                    return deleteObjectWithVersions(state.path, selectedFile);
                } else {
                    return deleteFolderWithVersions(state.path, selectedFile);
                }
            }));
        }

        try {
            return await finishDeleteSelected(promises);
        } catch (e) {
            return Promise.reject(e);
        } finally {
            removeFileToBeDeleted(...state.selectedFiles);
        }
    }

    async function deleteSelectedVersions(): Promise<number> {
        addFileToBeDeleted(...state.selectedFiles);
        const promises: Promise<number>[] = [];
        const files = state.selectedFiles.filter(file => file.type === 'file');
        const folders = state.selectedFiles.filter(file => file.type === 'folder');

        if (files.length) {
            promises.push(bulkDeleteObjects(files));
        }
        if (folders.length) {
            promises.push(...folders.map(selectedFile => deleteFolderWithVersions(state.path, selectedFile)));
        }
        try {
            return await finishDeleteSelected(promises);
        } catch (e) {
            return Promise.reject(e);
        } finally {
            removeFileToBeDeleted(...state.selectedFiles);
        }
    }

    async function finishDeleteSelected(promises: Promise<number>[]) {
        const results = await Promise.allSettled(promises);

        removeFileToBeDeleted(...state.selectedFiles);

        const deletedCount = results.reduce((count, result) => {
            if (result.status === 'fulfilled') {
                return count + result.value;
            }
            if (result.status === 'rejected') {
                const deleteError = (result as PromiseRejectedResult).reason as ObjectDeleteError;
                return count + deleteError.deletedCount || 0;
            }
            return count;
        }, 0);
        const rejection = results.find(result => result.status === 'rejected');
        if (rejection) {
            return Promise.reject(new ObjectDeleteError(deletedCount, (rejection as PromiseRejectedResult).reason.message));
        }
        return deletedCount;
    }

    /**
     * This is an empty action for App.vue to subscribe to know the status of the delete object/folder requests.
     *
     * @param _deleteRequest - the promise of the delete request.
     * @param _filesLabel - descriptive label of files deleted (versions/files).
     */
    function handleDeleteObjectRequest(_deleteRequest: Promise<number>, _filesLabel = 'object'): void {
        /* empty */
    }

    /**
     * Action for the file browser to refresh on files deleted.
     * It also resets the count for number of files deleted.
     */
    function filesDeleted(): void {
        /* empty */
    }

    async function getDownloadLink(file: BrowserObject, options: {
        contentDisposition?: string,
        contentType?: string,
    } | null = null): Promise<string> {
        assertIsInitialized(state);

        // backwards compatibility for previewing pdfs
        // with application/octet-stream content type
        if (!options && file.Key.endsWith('.pdf')) {
            options = {};
            options.contentType = 'application/pdf';
        }

        return await getSignedUrl(state.s3, new GetObjectCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file.VersionId,
            ResponseContentDisposition: options?.contentDisposition,
            ResponseContentType: options?.contentType,
        }));
    }

    async function download(file: BrowserObject): Promise<void> {
        const url = await getDownloadLink(file, {
            contentDisposition: 'attachment',
        });
        const downloadURL = function (data: string, fileName: string) {
            const a = document.createElement('a');
            a.href = data;
            a.download = fileName;
            a.click();
        };

        downloadURL(url, file.Key);
    }

    function updateSelectedFiles(files): void {
        state.selectedFiles = [...files];
    }

    function addFileToBeDeleted(...files: (_Object & { path?: string, VersionId?: string })[] | BrowserObject[]): void {
        for (const file of files) {
            state.filesToBeDeleted.add((file.path ?? '') + file.Key + (file.VersionId ?? ''));
        }
    }

    function removeFileToBeDeleted(...files: (_Object & { path?: string, VersionId?: string })[] | BrowserObject[]): void {
        for (const file of files) {
            state.filesToBeDeleted.delete((file.path ?? '') + file.Key + (file.VersionId ?? ''));
        }
    }

    function cancelUpload(key: string): void {
        const file = state.uploading.find(f => f.Key === key);
        if (!file) {
            throw new Error(`File '${key}' not found`);
        }
        file.upload?.abort();
        file.status = UploadingStatus.Cancelled;
    }

    function sort(headingSorted: string): void {
        const flip = (orderBy) => (orderBy === 'asc' ? 'desc' : 'asc');

        state.orderBy = state.headingSorted === headingSorted ? flip(state.orderBy) : 'asc';
        state.headingSorted = headingSorted;
    }

    function setObjectPathForModal(path: string): void {
        state.objectPathForModal = path;
    }

    function cacheObjectPreviewURL(path: string, cacheValue: PreviewCache): void {
        state.cachedObjectPreviewURLs.set(path, cacheValue);
    }

    function removeFromObjectPreviewCache(path: string): void {
        state.cachedObjectPreviewURLs.delete(path);
    }

    function clearUploading(): void {
        state.uploading = [];
    }

    function clearTokens(): void {
        state.continuationTokens = new Map<number, string>();
    }

    function toggleShowObjectVersions(toggle?: boolean, userModified = true): void {
        clearTokens();
        updateVersionsExpandedKeys([]);
        updateSelectedFiles([]);
        updateFiles(state.path, []);
        state.showObjectVersions = {
            value: toggle ?? !state.showObjectVersions.value,
            userModified: !userModified ? state.showObjectVersions.userModified : userModified,
        };
    }

    function setObjectCountOfSelectedBucket(count: number): void {
        state.objectCountOfSelectedBucket = count;
        LocalData.setObjectCountOfSelectedBucket(count);
    }

    function clear(): void {
        state.s3 = null;
        state.accessKey = null;
        state.path = '';
        state.bucket = '';
        state.browserRoot = '/';
        state.files = [];
        state.cursor = { limit: DEFAULT_PAGE_LIMIT, page: 1 };
        state.continuationTokens = new Map<number, string>();
        state.totalObjectCount = 0;
        state.activeObjectsRange = { start: 1, end: 500 };
        state.uploadChain = Promise.resolve();
        state.uploading = [];
        state.selectedFiles = [];
        state.filesToBeDeleted.clear();
        state.openedDropdown = null;
        state.headingSorted = 'name';
        state.orderBy = 'asc';
        state.openModalOnFirstUpload = false;
        state.objectPathForModal = '';
        state.cachedObjectPreviewURLs = new Map<string, PreviewCache>();
        state.showObjectVersions = { value: false, userModified: false };
        state.versionsExpandedKeys = [];
        state.objectCountOfSelectedBucket = 0;
        clearTimeout(state.largeFileNotificationTimeout);
        state.largeFileNotificationTimeout = undefined;
    }

    return {
        state,
        sortedFiles,
        displayedObjects,
        isInitialized,
        uploadingLength,
        isAltPagination,
        init,
        reinit,
        initList,
        listByToken,
        countVersions,
        listAllVersions,
        listCustom,
        setCursor,
        updateVersionsExpandedKeys,
        sort,
        upload,
        getFilesToUpload,
        lazyDuplicateCheck,
        restoreObject,
        createFolder,
        getObjectRetention,
        getObjectLockStatus,
        lockObject,
        legalHoldObject,
        getObjectLegalHold,
        deleteObject,
        deleteObjectWithVersions,
        deleteFolder,
        deleteFolderWithVersions,
        deleteSelected,
        deleteSelectedVersions,
        handleDeleteObjectRequest,
        filesDeleted,
        getDownloadLink,
        download,
        updateSelectedFiles,
        setObjectPathForModal,
        cancelUpload,
        cacheObjectPreviewURL,
        removeFromObjectPreviewCache,
        clearUploading,
        toggleShowObjectVersions,
        setObjectCountOfSelectedBucket,
        clear,
        clearTokens,
    };
});
