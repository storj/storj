// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive, UnwrapNestedRefs } from 'vue';
import { defineStore } from 'pinia';
import {
    _Object,
    CommonPrefix,
    DeleteObjectCommand,
    GetObjectCommand,
    ListObjectsCommand,
    ListObjectsV2Command,
    ListObjectsV2CommandInput,
    ListObjectVersionsCommand,
    paginateListObjectsV2,
    PutObjectCommand,
    S3Client,
    S3ClientConfig,
} from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';
import { Progress, Upload } from '@aws-sdk/lib-storage';
import { SignatureV4 } from '@smithy/signature-v4';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

const listCache = new Map();

export type BrowserObject = {
    Key: string;
    VersionId?: string;
    Size: number;
    LastModified: Date;
    type?: 'file' | 'folder';
    progress?: number;
    upload?: {
        abort: () => void;
    };
    path?: string;
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
}

export type PreviewCache = {
    url: string,
    lastModified: number,
}

export const MAX_KEY_COUNT = 500;

export type ObjectBrowserCursor = {
    page: number,
    limit: number,
}

export type ObjectRange = {
    start: number,
    end: number,
}

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
    selectedAnchorFile: BrowserObject | null = null;
    unselectedAnchorFile: BrowserObject | null = null;
    selectedFiles: BrowserObject[] = [];
    shiftSelectedFiles: BrowserObject[] = [];
    filesToBeDeleted: BrowserObject[] = [];
    openedDropdown: null | string = null;
    headingSorted = 'name';
    orderBy: 'asc' | 'desc' = 'asc';
    openModalOnFirstUpload = false;
    objectPathForModal = '';
    objectsCount = 0;
    cachedObjectPreviewURLs: Map<string, PreviewCache> = new Map<string, PreviewCache>();
    showObjectVersions: boolean = true;
    objectVersions: Map<string, BrowserObject[]> = new Map<string, BrowserObject[]>();
    // object keys for which we have expanded versions list.
    versionsExpandedKeys: string[] = [];
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

    const config = useConfigStore();

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

    async function list(path = state.path): Promise<void> {
        if (listCache.has(path)) {
            updateFiles(path, listCache.get(path));
        }

        assertIsInitialized(state);

        const response = await state.s3.send(new ListObjectsCommand({
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: path,
        }));

        const { Contents, CommonPrefixes } = response;

        processFetchedObjects(path, Contents, CommonPrefixes);
    }

    async function listVersions(objectKey: string): Promise<void> {
        assertIsInitialized(state);
        const response = await state.s3.send(new ListObjectVersionsCommand({
            Bucket: state.bucket,
            Delimiter: '/',
            Prefix: objectKey,
        }));
        const Key = objectKey.substring(objectKey.lastIndexOf('/') + 1);
        const path = objectKey.substring(0, objectKey.lastIndexOf('/') + 1);

        const { Versions } = response;
        const versions = Versions ?? [];

        const makeFileRelative = (file) => ({
            ...file,
            Key,
            path,
            type: 'file',
        });

        const files = versions.map(file => ({
            ...file,
            Key,
            path,
            type: 'file',
        })) as BrowserObject[];
        // remove the first element which is the current version of the object
        files.shift();

        state.objectVersions.set(objectKey, files);
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

        type DefinedCommonPrefix = CommonPrefix & {
            Prefix: string;
        };

        const isPrefixDefined = (
            value: CommonPrefix,
        ): value is DefinedCommonPrefix => value.Prefix !== undefined;

        const prefixToFolder = ({
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

        const makeFileRelative = (file) => ({
            ...file,
            Key: file.Key.slice(path.length),
            path: path,
            type: 'file',
        });

        const isFileVisible = (file) =>
            file.Key.length > 0 && file.Key !== '.file_placeholder';

        const files: BrowserObject[] = [
            ...CommonPrefixes.filter(isPrefixDefined).map(prefixToFolder),
            ...Contents.map(makeFileRelative).filter(isFileVisible),
        ];

        listCache.set(path, files);
        updateFiles(path, files);
    }

    async function back(): Promise<void> {
        const getParentDirectory = (path: string) => {
            let i = path.length - 2;

            while (path[i - 1] !== '/' && i > 0) {
                i--;
            }

            return path.slice(0, i);
        };

        list(getParentDirectory(state.path));
    }

    async function getObjectCount(): Promise<void> {
        assertIsInitialized(state);

        const response = await state.s3.send(new ListObjectsV2Command({
            Bucket: state.bucket,
            MaxKeys: 1, // We need to know if there is at least 1 decryptable object.
        }));

        state.objectsCount = (!response || response.KeyCount === undefined) ? 0 : response.KeyCount;
    }

    async function upload({ e }: { e: DragEvent | Event }): Promise<void> {
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

        for await (const { path, file } of traverse(iterator)) {
            const directories = path.split('/');
            const fileName = directories.join('/') + file.name;
            const key = state.path + fileName;

            await enqueueUpload(key, file);
        }
    }

    async function retryUpload(key: string): Promise<void> {
        assertIsInitialized(state);

        const item = state.uploading.find(file => file.Key === key);
        if (!item) {
            throw new Error(`No uploads found with key '${key}'`);
        }

        return await enqueueUpload(item.Key, item.Body);
    }

    async function enqueueUpload(key: string, body: File): Promise<void> {
        assertIsInitialized(state);

        const appStore = useAppStore();
        const { notifyError } = useNotificationsStore();

        const params = {
            Bucket: state.bucket,
            Key: key,
            Body: body,
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
                Size: 0,
                LastModified: new Date(),
                Body: body,
                status: UploadingStatus.Failed,
                failedMessage: FailedUploadMessage.TooBig,
                type: 'file',
            });

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
            Size: 0,
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

            if (config.state.config.objectBrowserPaginationEnabled) {
                await initList();
            } else {
                await list();
            }
            if (state.versionsExpandedKeys.includes(item.Key)) {
                listVersions(item.Key);
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

        const { notifyError } = useNotificationsStore();

        const limitExceededError = 'storage limit exceeded';
        if (error.message.includes(limitExceededError)) {
            notifyError(`Error: ${limitExceededError}`, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);
        } else {
            notifyError(error.message, AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR);
        }
    }

    async function createFolder(name: string): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new PutObjectCommand({
            Bucket: state.bucket,
            Key: state.path + name + '/.file_placeholder',
            Body: '',
        }));

        if (config.state.config.objectBrowserPaginationEnabled) {
            initList();
        } else {
            list();
        }
    }

    async function deleteObject(path: string, file?: _Object | BrowserObject, isFolder = false): Promise<void> {
        if (!file) {
            return;
        }

        assertIsInitialized(state);

        await state.s3.send(new DeleteObjectCommand({
            Bucket: state.bucket,
            Key: path + file.Key,
            VersionId: file['VersionId'] ?? undefined,
        }));

        state.uploading = state.uploading.filter(f => f.Key !== path + file.Key);

        if (!isFolder) {
            if (config.state.config.objectBrowserPaginationEnabled) {
                await initList();
            } else {
                await list();
            }

            if (file['VersionId']) {
                // versioned object
                await listVersions(state.path + file.Key);
            }

            removeFileFromToBeDeleted(file);
        }
    }

    async function deleteFolder(file: BrowserObject, path: string): Promise<void> {
        assertIsInitialized(state);

        async function recurse(filePath: string) {
            assertIsInitialized(state);

            let { Contents, CommonPrefixes } = await state.s3.send(new ListObjectsCommand({
                Bucket: state.bucket,
                Delimiter: '/',
                Prefix: filePath,
            }));

            if (Contents === undefined) {
                Contents = [];
            }

            if (CommonPrefixes === undefined) {
                CommonPrefixes = [];
            }

            async function thread() {
                if (Contents === undefined) {
                    Contents = [];
                }

                while (Contents.length) {
                    const file = Contents.pop();

                    await deleteObject('', file, true);
                }
            }

            await Promise.all([thread(), thread(), thread()]);

            for (const { Prefix } of CommonPrefixes) {
                await recurse(Prefix ?? '');
            }
        }

        await recurse(path.length > 0 ? path + file.Key : file.Key + '/');

        removeFileFromToBeDeleted(file);
        if (config.state.config.objectBrowserPaginationEnabled) {
            await initList();
        } else {
            await list();
        }
    }

    async function deleteSelected(): Promise<void> {
        const filesToDelete = [
            ...state.selectedFiles,
            ...state.shiftSelectedFiles,
        ];

        if (state.selectedAnchorFile) {
            filesToDelete.push(state.selectedAnchorFile);
        }

        addFileToBeDeleted(filesToDelete);

        await Promise.all(
            filesToDelete.map(async (file) => {
                if (file.type === 'file') {
                    await deleteObject(state.path, file);
                } else {
                    await deleteFolder(file, state.path);
                }
            }),
        );

        clearAllSelectedFiles();
    }

    async function getDownloadLink(file: BrowserObject): Promise<string> {
        assertIsInitialized(state);

        return await getSignedUrl(state.s3, new GetObjectCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
            VersionId: file.VersionId,
        }));
    }

    async function download(file: BrowserObject): Promise<void> {
        const url = await getDownloadLink(file);
        const downloadURL = function(data: string, fileName: string) {
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

    function updateShiftSelectedFiles(files): void {
        state.shiftSelectedFiles = files;
    }

    function addFileToBeDeleted(file): void {
        state.filesToBeDeleted = [...state.filesToBeDeleted, file];
    }

    function removeFileFromToBeDeleted(file): void {
        state.filesToBeDeleted = state.filesToBeDeleted.filter(
            singleFile => !(singleFile.Key === file.Key && singleFile.path === file.path),
        );
    }

    function clearAllSelectedFiles(): void {
        if (state.selectedAnchorFile || state.unselectedAnchorFile) {
            state.selectedAnchorFile = null;
            state.unselectedAnchorFile = null;
            state.shiftSelectedFiles = [];
            state.selectedFiles = [];
        }
    }

    function openDropdown(id): void {
        clearAllSelectedFiles();
        state.openedDropdown = id;
    }

    function closeDropdown(): void {
        state.openedDropdown = null;
    }

    function openFileBrowserDropdown(): void {
        state.openedDropdown = 'FileBrowser';
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

    function setSelectedAnchorFile(file: BrowserObject | null): void {
        state.selectedAnchorFile = file;
    }

    function setUnselectedAnchorFile(file: BrowserObject | null): void {
        state.unselectedAnchorFile = file;
    }

    function clearUploading(): void {
        state.uploading = [];
    }

    function toggleShowObjectVersions(): void {
        state.showObjectVersions = !state.showObjectVersions;
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
        state.selectedAnchorFile = null;
        state.unselectedAnchorFile = null;
        state.selectedFiles = [];
        state.shiftSelectedFiles = [];
        state.filesToBeDeleted = [];
        state.openedDropdown = null;
        state.headingSorted = 'name';
        state.orderBy = 'asc';
        state.openModalOnFirstUpload = false;
        state.objectPathForModal = '';
        state.cachedObjectPreviewURLs = new Map<string, PreviewCache>();
        state.showObjectVersions = true;
        state.objectVersions = new Map<string, BrowserObject[]>();
        state.versionsExpandedKeys = [];
    }

    return {
        state,
        sortedFiles,
        displayedObjects,
        isInitialized,
        uploadingLength,
        init,
        reinit,
        list,
        initList,
        listByToken,
        listVersions,
        back,
        setCursor,
        updateVersionsExpandedKeys,
        sort,
        getObjectCount,
        upload,
        retryUpload,
        createFolder,
        deleteObject,
        deleteFolder,
        deleteSelected,
        getDownloadLink,
        download,
        updateSelectedFiles,
        updateShiftSelectedFiles,
        addFileToBeDeleted,
        removeFileFromToBeDeleted,
        clearAllSelectedFiles,
        setObjectPathForModal,
        openDropdown,
        closeDropdown,
        openFileBrowserDropdown,
        setSelectedAnchorFile,
        setUnselectedAnchorFile,
        cancelUpload,
        cacheObjectPreviewURL,
        removeFromObjectPreviewCache,
        clearUploading,
        toggleShowObjectVersions,
        clear,
    };
});
