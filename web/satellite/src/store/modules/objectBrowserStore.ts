// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';
import {
    S3Client,
    CommonPrefix,
    S3ClientConfig,
    ListObjectsCommand,
    ListObjectsV2Command,
    DeleteObjectCommand,
    PutObjectCommand,
    _Object,
    GetObjectCommand,
} from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';
import { Progress, Upload } from '@aws-sdk/lib-storage';
import { SignatureV4 } from '@aws-sdk/signature-v4';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useConfigStore } from '@/store/modules/configStore';

const listCache = new Map();

type Promisable<T> = T | PromiseLike<T>;

export type BrowserObject = {
    Key: string;
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
    Body: File;
    failedMessage?: FailedUploadMessage;
}

export type PreviewCache = {
    url: string,
    lastModified: number,
}

export class FilesState {
    s3: S3Client | null = null;
    accessKey: null | string = null;
    path = '';
    bucket = '';
    browserRoot = '/';
    files: BrowserObject[] = [];
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
}

type InitializedFilesState = FilesState & {
  s3: S3Client;
};

function assertIsInitialized(
    state: FilesState,
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

    const isInitialized = computed(() => {
        return state.s3 !== null;
    });

    const uploadingLength = computed(() => {
        const config = useConfigStore();

        if (config.state.config.newUploadModalEnabled) {
            return state.uploading.filter(f => f.status === UploadingStatus.InProgress).length;
        }

        return state.uploading.length;
    });

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

        let { Contents, CommonPrefixes } = response;

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
            LastModified: new Date(),
            Size: 0,
            type: 'folder',
        });

        const makeFileRelative = (file) => ({
            ...file,
            Key: file.Key.slice(path.length),
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
        const getParentDirectory = (path) => {
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

        const responseV2 = await state.s3.send(new ListObjectsV2Command({
            Bucket: state.bucket,
        }));

        state.objectsCount = responseV2.KeyCount === undefined ? 0 : responseV2.KeyCount;
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

        const fileNames = state.files.map((file) => file.Key);

        function getUniqueFileName(fileName: string): string {
            for (let count = 1; fileNames.includes(fileName); count++) {
                if (count > 1) {
                    fileName = fileName.replace(/\((\d+)\)(.*)/, `(${count})$2`);
                } else {
                    fileName = fileName.replace(/([^.]*)(.*)/, `$1 (${count})$2`);
                }
            }

            return fileName;
        }

        for await (const { path, file } of traverse(iterator)) {
            const directories = path.split('/');
            directories[0] = getUniqueFileName(directories[0]);

            const fileName = getUniqueFileName(directories.join('/') + file.name);
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
        const config = useConfigStore();
        const { notifyError } = useNotificationsStore();

        const params = {
            Bucket: state.bucket,
            Key: key,
            Body: body,
        };

        if (config.state.config.newUploadModalEnabled) {
            if (state.uploading.some(f => f.Key === key && f.status === UploadingStatus.InProgress)) {
                notifyError({ message: `${key} is already uploading`, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR });
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
                });

                return;
            }
        }

        // If file size exceeds 1 GB, show warning notification
        if (body.size > (1024 * 1024 * 1024)) {
            appStore.setLargeUploadWarningNotification(true);
        }

        // Upload 4 parts at a time.
        const queueSize = 4;
        // Part size must be 64MB or higher, depending on file size.
        const partSize = Math.max(64 * 1024 * 1024, Math.floor(body.size / queueSize));

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
                notifyError({
                    message: `Error updating progress. No file found with key '${key}'`,
                    source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR,
                });
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

            await list();

            const uploadedFiles = state.files.filter(f => f.type === 'file');
            if (uploadedFiles.length === 1 && !key.includes('/') && state.openModalOnFirstUpload) {
                state.objectPathForModal = key;

                if (config.state.config.galleryViewEnabled) {
                    appStore.setGalleryView(true);
                } else {
                    appStore.updateActiveModal(MODALS.objectDetails);
                }
            }

            if (!config.state.config.newUploadModalEnabled) {
                state.uploading = state.uploading.filter(file => file.Key !== key);
            }
        });
    }

    function handleUploadError(item: UploadingBrowserObject, error: Error): void {
        if (error.name === 'AbortError' && item.status === UploadingStatus.Cancelled) return;

        const config = useConfigStore();

        if (config.state.config.newUploadModalEnabled) {
            item.status = UploadingStatus.Failed;
            item.failedMessage = FailedUploadMessage.Failed;
        }

        const { notifyError } = useNotificationsStore();

        const limitExceededError = 'storage limit exceeded';
        if (error.message.includes(limitExceededError)) {
            notifyError({ message: `Error: ${limitExceededError}`, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR });
        } else {
            notifyError({ message: error.message, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR });
        }
    }

    async function createFolder(name): Promise<void> {
        assertIsInitialized(state);

        await state.s3.send(new PutObjectCommand({
            Bucket: state.bucket,
            Key: state.path + name + '/.file_placeholder',
            Body: '',
        }));

        list();
    }

    async function deleteObject(path: string, file?: _Object | BrowserObject, isFolder = false): Promise<void> {
        if (!file) {
            return;
        }

        assertIsInitialized(state);

        await state.s3.send(new DeleteObjectCommand({
            Bucket: state.bucket,
            Key: path + file.Key,
        }));

        const config = useConfigStore();
        if (config.state.config.newUploadModalEnabled) {
            state.uploading = state.uploading.filter(f => f.Key !== file.Key);
        }

        if (!isFolder) {
            await list();
            removeFileFromToBeDeleted(file);
        }
    }

    async function deleteFolder(file: BrowserObject, path: string): Promise<void> {
        assertIsInitialized(state);

        async function recurse(filePath) {
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
                await recurse(Prefix);
            }
        }

        await recurse(path.length > 0 ? path + file.Key : file.Key + '/');

        removeFileFromToBeDeleted(file);
        await list();
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

    async function download(file): Promise<void> {
        assertIsInitialized(state);

        const url = await getSignedUrl(state.s3, new GetObjectCommand({
            Bucket: state.bucket,
            Key: state.path + file.Key,
        }));
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
            singleFile => singleFile.Key !== file.Key,
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

    function clear(): void {
        state.s3 = null;
        state.accessKey = null;
        state.path = '';
        state.bucket = '';
        state.browserRoot = '/';
        state.files = [];
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
    }

    return {
        state,
        sortedFiles,
        isInitialized,
        uploadingLength,
        init,
        reinit,
        list,
        back,
        sort,
        getObjectCount,
        upload,
        retryUpload,
        createFolder,
        deleteObject,
        deleteFolder,
        deleteSelected,
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
        clear,
    };
});
