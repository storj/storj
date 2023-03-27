// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';
import S3, { CommonPrefix } from 'aws-sdk/clients/s3';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';

const listCache = new Map();

type Promisable<T> = T | PromiseLike<T>;

export type BrowserObject = {
    Key: string;
    Size: number;
    LastModified: number;
    type?: 'file' | 'folder';
    progress?: number;
    upload?: {
      abort: () => void;
    };
    path?: string;
};

export class FilesState {
    s3: S3 | null = null;
    accessKey: null | string = null;
    path = '';
    bucket = '';
    browserRoot = '/';
    files: BrowserObject[] = [];
    uploadChain: Promise<void> = Promise.resolve();
    uploading: BrowserObject[] = [];
    selectedAnchorFile: BrowserObject | null = null;
    unselectedAnchorFile: null | string = null;
    selectedFiles: BrowserObject[] = [];
    shiftSelectedFiles: BrowserObject[] = [];
    filesToBeDeleted: BrowserObject[] = [];
    fetchSharedLink: (arg0: string) => Promisable<string> = () => 'javascript:null';
    fetchPreviewAndMapUrl: (arg0: string) => Promisable<string> = () => 'javascript:null';
    openedDropdown: null | string = null;
    headingSorted = 'name';
    orderBy: 'asc' | 'desc' = 'asc';
    openModalOnFirstUpload = false;
    objectPathForModal = '';
    objectsCount = 0;
}

type InitializedFilesState = FilesState & {
  s3: S3;
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

export const useFilesStore = defineStore('files', () => {
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
        return state.uploading.length;
    });

    function init({
        accessKey,
        secretKey,
        bucket,
        endpoint,
        browserRoot,
        openModalOnFirstUpload = true,
        fetchSharedLink = () => 'javascript:null',
        fetchPreviewAndMapUrl = () => 'javascript:null',
    }: {
        accessKey: string;
        secretKey: string;
        bucket: string;
        endpoint: string;
        browserRoot: string;
        openModalOnFirstUpload: boolean;
        fetchSharedLink: (arg0: string) => Promisable<string>;
        fetchPreviewAndMapUrl: (arg0: string) => Promisable<string>;
    }): void {
        const s3Config = {
            accessKeyId: accessKey,
            secretAccessKey: secretKey,
            endpoint,
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            connectTimeout: 0,
            httpOptions: { timeout: 0 },
        };

        state.s3 = new S3(s3Config);
        state.accessKey = accessKey;
        state.bucket = bucket;
        state.browserRoot = browserRoot;
        state.openModalOnFirstUpload = openModalOnFirstUpload;
        state.fetchSharedLink = fetchSharedLink;
        state.fetchPreviewAndMapUrl = fetchPreviewAndMapUrl;
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
        const s3Config = {
            accessKeyId: accessKey,
            secretAccessKey: secretKey,
            endpoint,
            s3ForcePathStyle: true,
            signatureVersion: 'v4',
            connectTimeout: 0,
            httpOptions: { timeout: 0 },
        };

        state.files = [];
        state.s3 = new S3(s3Config);
        state.accessKey = accessKey;
    }

    function updateFiles(path: string, files: BrowserObject[]): void {
        state.path = path;
        state.files = files;
    }

    async function list(path = state.path) {
        if (listCache.has(path)) {
            updateFiles(path, listCache.get(path));
        }

        assertIsInitialized(state);

        const response = await state.s3
            .listObjects({
                Bucket: state.bucket,
                Delimiter: '/',
                Prefix: path,
            })
            .promise();

        const { Contents, CommonPrefixes } = response;

        if (Contents === undefined) {
            throw new Error('Bad S3 listObjects() response: "Contents" undefined');
        }

        if (CommonPrefixes === undefined) {
            throw new Error(
                'Bad S3 listObjects() response: "CommonPrefixes" undefined',
            );
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
            LastModified: 0,
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

    async function getObjectCount() {
        assertIsInitialized(state);

        const responseV2 = await state.s3
            .listObjectsV2({
                Bucket: state.bucket,
            })
            .promise();

        state.objectsCount = responseV2.KeyCount === undefined ? 0 : responseV2.KeyCount;
    }

    async function upload({ e }: { e: DragEvent }) {
        assertIsInitialized(state);

        type Item = DataTransferItem | FileSystemEntry;

        const items: Item[] = e.dataTransfer
            ? [...e.dataTransfer.items]
            : e.target !== null
                ? ((e.target as unknown) as { files: FileSystemEntry[] }).files
                : [];

        async function* traverse(item: Item | Item[], path = '') {
            if ('isFile' in item && item.isFile === true) {
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
            } else if ('length' in item && typeof item.length === 'number') {
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

        function getUniqueFileName(fileName) {
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

            const params = {
                Bucket: state.bucket,
                Key: state.path + fileName,
                Body: file,
            };

            const upload = state.s3.upload(
                { ...params },
                { partSize: 64 * 1024 * 1024 },
            );

            upload.on('httpUploadProgress', async (progress) => {
                const file = state.uploading.find((file) => file.Key === params.Key);

                if (file === undefined) {
                    throw new Error(`No file found with key ${JSON.stringify(params.Key)}`);
                }

                file.progress = Math.round((progress.loaded / progress.total) * 100);
            });

            state.uploading.push({
                ...params,
                upload,
                progress: 0,
                Size: 0,
                LastModified: 0,
            });

            state.uploadChain = state.uploadChain.then(async () => {
                if (
                    state.uploading.findIndex((file) => file.Key === params.Key) === -1
                ) {
                    // upload cancelled or removed
                    return;
                }

                try {
                    await upload.promise();
                } catch (error) {
                    const { notifyError } = useNotificationsStore();
                    const limitExceededError = 'storage limit exceeded';
                    if (error.message.includes(limitExceededError)) {
                        notifyError({ message: `Error: ${limitExceededError}`, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR });
                    } else {
                        notifyError({ message: error.message, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR });
                    }
                }

                await list();

                const uploadedFiles = state.files.filter(
                    (file) => file.type === 'file',
                );

                if (uploadedFiles.length === 1) {
                    if (state.openModalOnFirstUpload) {
                        state.objectPathForModal = params.Key;

                        const { updateActiveModal } = useAppStore();
                        updateActiveModal(MODALS.objectDetails);
                    }
                }

                state.uploading = state.uploading.filter((file) => file.Key !== params.Key);
            });
        }
    }

    async function createFolder(name) {
        assertIsInitialized(state);

        await state.s3
            .putObject({
                Bucket: state.bucket,
                Key: state.path + name + '/.file_placeholder',
            })
            .promise();

        list();
    }

    async function deleteObject(path: string, file?: S3.Object | BrowserObject, isFolder = false) {
        if (!file) {
            return;
        }

        assertIsInitialized(state);

        await state.s3
            .deleteObject({
                Bucket: state.bucket,
                Key: path + file.Key,
            })
            .promise();

        if (!isFolder) {
            await list();
            removeFileFromToBeDeleted(file);
        }
    }

    async function deleteFolder(file: BrowserObject, path: string) {
        assertIsInitialized(state);

        async function recurse(filePath) {
            assertIsInitialized(state);

            const { Contents, CommonPrefixes } = await state.s3
                .listObjects({
                    Bucket: state.bucket,
                    Delimiter: '/',
                    Prefix: filePath,
                })
                .promise();

            if (Contents === undefined) {
                throw new Error(
                    'Bad S3 listObjects() response: "Contents" undefined',
                );
            }

            if (CommonPrefixes === undefined) {
                throw new Error(
                    'Bad S3 listObjects() response: "CommonPrefixes" undefined',
                );
            }

            async function thread() {
                if (Contents === undefined) {
                    throw new Error(
                        'Bad S3 listObjects() response: "Contents" undefined',
                    );
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

    async function deleteSelected() {
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

    async function download(file) {
        assertIsInitialized(state);

        const url = state.s3.getSignedUrl('getObject', {
            Bucket: state.bucket,
            Key: state.path + file.Key,
        });
        const downloadURL = function(data, fileName) {
            const a = document.createElement('a');
            a.href = data;
            a.download = fileName;
            a.click();
        };

        downloadURL(url, file.Key);
    }

    function updateSelectedFiles(files) {
        state.selectedFiles = [...files];
    }

    function updateShiftSelectedFiles(files) {
        state.shiftSelectedFiles = files;
    }

    function addFileToBeDeleted(file) {
        state.filesToBeDeleted = [...state.filesToBeDeleted, file];
    }

    function removeFileFromToBeDeleted(file) {
        state.filesToBeDeleted = state.filesToBeDeleted.filter(
            (singleFile) => singleFile.Key !== file.Key,
        );
    }

    function clearAllSelectedFiles() {
        if (state.selectedAnchorFile || state.unselectedAnchorFile) {
            state.selectedAnchorFile = null;
            state.unselectedAnchorFile = null;
            state.shiftSelectedFiles = [];
            state.selectedFiles = [];
        }
    }

    function openDropdown(id) {
        clearAllSelectedFiles();
        state.openedDropdown = id;
    }

    function closeDropdown() {
        state.openedDropdown = null;
    }

    function openFileBrowserDropdown() {
        state.openedDropdown = 'FileBrowser';
    }

    function cancelUpload(key) {
        const file = state.uploading.find((file) => file.Key === key);

        if (typeof file === 'object') {
            if (file.progress !== undefined && file.upload && file.progress > 0) {
                file.upload.abort();
            }

            state.uploading = state.uploading.filter((file) => file.Key !== key);
        } else {
            throw new Error(`File ${JSON.stringify(key)} not found`);
        }
    }

    function closeAllInteractions() {
        if (state.openedDropdown) {
            closeDropdown();
        }

        if (state.selectedAnchorFile) {
            clearAllSelectedFiles();
        }
    }

    function clear() {
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
        state.fetchSharedLink = () => 'javascript:null';
        state.fetchPreviewAndMapUrl = () => 'javascript:null';
        state.openedDropdown = null;
        state.headingSorted = 'name';
        state.orderBy = 'asc';
        state.openModalOnFirstUpload = false;
        state.objectPathForModal = '';
    }

    return {
        filesState: state,
    };
});
