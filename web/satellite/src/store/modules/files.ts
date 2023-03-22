// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import S3, { CommonPrefix } from 'aws-sdk/clients/s3';

import { StoreModule } from '@/types/store';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

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

export type FilesState = {
    s3: S3 | null;
    accessKey: null | string;

    path: string;
    bucket: string;
    browserRoot: string;
    files: BrowserObject[];
    uploadChain: Promise<void>;
    uploading: BrowserObject[];
    selectedAnchorFile: BrowserObject | null;
    unselectedAnchorFile: null | string;
    selectedFiles: BrowserObject[];
    shiftSelectedFiles: BrowserObject[];
    filesToBeDeleted: BrowserObject[];

    fetchSharedLink: (arg0: string) => Promisable<string>;
    fetchPreviewAndMapUrl: (arg0: string) => Promisable<string>;

    openedDropdown: null | string;
    headingSorted: string;
    orderBy: 'asc' | 'desc';
    openModalOnFirstUpload: boolean;
    objectPathForModal: string;
    objectsCount: number;
};

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

interface FilesContext {
    state: FilesState;
    commit: (string, ...unknown) => void;
    dispatch: (string, ...unknown) => Promise<any>; // eslint-disable-line @typescript-eslint/no-explicit-any
    rootState: {
        files: FilesState;
    };
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

type FilesModule = StoreModule<FilesState, FilesContext> & { namespaced: true };

export const makeFilesModule = (): FilesModule => ({
    namespaced: true,
    state: {
        s3: null,
        accessKey: null,

        path: '',
        bucket: '',
        browserRoot: '/',
        files: [],
        uploadChain: Promise.resolve(),
        uploading: [],
        selectedAnchorFile: null,
        unselectedAnchorFile: null,
        selectedFiles: [],
        shiftSelectedFiles: [],
        filesToBeDeleted: [],
        fetchSharedLink: () => 'javascript:null',
        fetchPreviewAndMapUrl: () => 'javascript:null',
        openedDropdown: null,
        headingSorted: 'name',
        orderBy: 'asc',
        openModalOnFirstUpload: false,
        objectPathForModal: '',
        objectsCount: 0,
    },
    getters: {
        sortedFiles: (state: FilesState) => {
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
            const groupedFiles = [
                ...sortedFiles.filter((file) => file.type === 'folder'),
                ...sortedFiles.filter((file) => file.type === 'file'),
            ];

            return groupedFiles;
        },

        isInitialized: (state: FilesState): boolean => state.s3 !== null,
        uploadingLength: (state: FilesState): number => state.uploading.length,
    },
    mutations: {
        init(
            state: FilesState,
            {
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
            },
        ) {
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
        },

        reinit(state: FilesState, {
            accessKey,
            secretKey,
            endpoint,
        }: {
            accessKey: string;
            secretKey: string;
            endpoint: string;
        }) {
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
        },

        updateFiles(state: FilesState, { path, files }) {
            state.path = path;
            state.files = files;
        },

        setSelectedFiles(state: FilesState, files) {
            state.selectedFiles = files;
        },

        setSelectedAnchorFile(state: FilesState, file) {
            state.selectedAnchorFile = file;
        },

        setUnselectedAnchorFile(state: FilesState, file) {
            state.unselectedAnchorFile = file;
        },

        setFilesToBeDeleted(state: FilesState, files) {
            state.filesToBeDeleted = [...state.filesToBeDeleted, ...files];
        },

        removeFileToBeDeleted(state: FilesState, file) {
            state.filesToBeDeleted = state.filesToBeDeleted.filter(
                (singleFile) => singleFile.Key !== file.Key,
            );
        },

        removeAllFilesToBeDeleted(state: FilesState) {
            state.filesToBeDeleted = [];
        },

        removeAllSelectedFiles(state: FilesState) {
            state.selectedAnchorFile = null;
            state.unselectedAnchorFile = null;
            state.shiftSelectedFiles = [];
            state.selectedFiles = [];
        },

        setShiftSelectedFiles(state: FilesState, files) {
            state.shiftSelectedFiles = files;
        },

        pushUpload(state: FilesState, file) {
            state.uploading.push(file);
        },

        setProgress(state: FilesState, { Key, progress }) {
            const file = state.uploading.find((file) => file.Key === Key);

            if (file === undefined) {
                throw new Error(`No file found with key ${JSON.stringify(Key)}`);
            }

            file.progress = progress;
        },

        finishUpload(state: FilesState, Key) {
            state.uploading = state.uploading.filter((file) => file.Key !== Key);
        },

        setOpenedDropdown(state: FilesState, id) {
            state.openedDropdown = id;
        },

        sort(state: FilesState, headingSorted) {
            const flip = (orderBy) => (orderBy === 'asc' ? 'desc' : 'asc');

            state.orderBy = state.headingSorted === headingSorted ? flip(state.orderBy) : 'asc';
            state.headingSorted = headingSorted;
        },

        setObjectPathForModal(state: FilesState, path) {
            state.objectPathForModal = path;
        },

        setObjectsCount(state: FilesState, count: number) {
            state.objectsCount = count;
        },

        addUploadToChain(state: FilesState, fn) {
            state.uploadChain = state.uploadChain.then(fn);
        },

        clear(state: FilesState) {
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
        },
    },
    actions: {
        async list({ commit, state, dispatch }, path = state.path) {
            if (listCache.has(path)) {
                commit('updateFiles', {
                    path,
                    files: listCache.get(path),
                });
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

            const files = [
                ...CommonPrefixes.filter(isPrefixDefined).map(prefixToFolder),
                ...Contents.map(makeFileRelative).filter(isFileVisible),
            ];

            listCache.set(path, files);
            commit('updateFiles', {
                path,
                files,
            });
        },

        async back({ state, dispatch }) {
            const getParentDirectory = (path) => {
                let i = path.length - 2;

                while (path[i - 1] !== '/' && i > 0) {
                    i--;
                }

                return path.slice(0, i);
            };

            dispatch('list', getParentDirectory(state.path));
        },

        async getObjectCount({ commit, state }) {
            assertIsInitialized(state);

            const responseV2 = await state.s3
                .listObjectsV2({
                    Bucket: state.bucket,
                })
                .promise();

            commit('setObjectsCount', responseV2.KeyCount === undefined ? 0 : responseV2.KeyCount);
        },

        async upload({ commit, state, dispatch }, { e }: { e: DragEvent }) {
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

                // If file size exceeds 1 GB, show warning notification
                if (file.size > (1024 * 1024 * 1024)) {
                    const appStore = useAppStore();
                    appStore.setLargeUploadWarningNotification(true);
                }

                const upload = state.s3.upload(
                    { ...params },
                    { partSize: 64 * 1024 * 1024 },
                );

                upload.on('httpUploadProgress', async (progress) => {
                    commit('setProgress', {
                        Key: params.Key,
                        progress: Math.round((progress.loaded / progress.total) * 100),
                    });
                });

                commit('pushUpload', {
                    ...params,
                    upload,
                    progress: 0,
                });

                commit('addUploadToChain', async () => {
                    if (
                        state.uploading.findIndex((file) => file.Key === params.Key) === -1
                    ) {
                        // upload cancelled or removed
                        return -1;
                    }

                    try {
                        await upload.promise();
                    } catch (error) {
                        const limitExceededError = 'storage limit exceeded';
                        if (error.message.includes(limitExceededError)) {
                            dispatch('error', { message: `Error: ${limitExceededError}`, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR }, { root: true });
                        } else {
                            dispatch('error', { message: error.message, source: AnalyticsErrorEventSource.OBJECT_UPLOAD_ERROR }, { root: true });
                        }
                    }

                    await dispatch('list');

                    const uploadedFiles = state.files.filter(
                        (file) => file.type === 'file',
                    );

                    if (uploadedFiles.length === 1 && !path && state.openModalOnFirstUpload) {
                        commit('setObjectPathForModal', params.Key);
                        const appStore = useAppStore();
                        appStore.updateActiveModal(MODALS.objectDetails);
                    }

                    commit('finishUpload', params.Key);
                });
            }
        },

        async createFolder({ state, dispatch }, name) {
            assertIsInitialized(state);

            await state.s3
                .putObject({
                    Bucket: state.bucket,
                    Key: state.path + name + '/.file_placeholder',
                })
                .promise();

            dispatch('list');
        },

        async delete(
            { commit, dispatch, state }: FilesContext,
            { path, file, folder },
        ) {
            assertIsInitialized(state);

            await state.s3
                .deleteObject({
                    Bucket: state.bucket,
                    Key: path + file.Key,
                })
                .promise();

            if (!folder) {
                await dispatch('list');
                commit('removeFileToBeDeleted', file);
            }
        },

        async deleteFolder({ commit, dispatch, state }: FilesContext, { file, path }) {
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

                        await dispatch('delete', {
                            path: '',
                            file,
                            folder: true,
                        });
                    }
                }

                await Promise.all([thread(), thread(), thread()]);

                for (const { Prefix } of CommonPrefixes) {
                    await recurse(Prefix);
                }
            }

            await recurse(path.length > 0 ? path + file.Key : file.Key + '/');

            commit('removeFileToBeDeleted', file);
            await dispatch('list');
        },

        async deleteSelected({ state, dispatch, commit }) {
            const filesToDelete = [
                ...state.selectedFiles,
                ...state.shiftSelectedFiles,
            ];

            if (state.selectedAnchorFile) {
                filesToDelete.push(state.selectedAnchorFile);
            }

            commit('setFilesToBeDeleted', filesToDelete);

            await Promise.all(
                filesToDelete.map(async (file) => {
                    if (file.type === 'file')
                        await dispatch('delete', {
                            file,
                            path: state.path,
                        });
                    else
                        await dispatch('deleteFolder', {
                            file,
                            path: state.path,
                        });
                }),
            );

            dispatch('clearAllSelectedFiles');
        },

        async download({ state }, file) {
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
        },

        updateSelectedFiles({ commit }, files) {
            commit('setSelectedFiles', [...files]);
        },

        updateShiftSelectedFiles({ commit }, files) {
            commit('setShiftSelectedFiles', files);
        },

        addFileToBeDeleted({ commit }, file) {
            commit('setFilesToBeDeleted', [file]);
        },

        removeFileFromToBeDeleted({ commit }, file) {
            commit('removeFileToBeDeleted', file);
        },

        clearAllSelectedFiles({ commit, state }) {
            if (state.selectedAnchorFile || state.unselectedAnchorFile) {
                commit('removeAllSelectedFiles');
            }
        },

        openDropdown({ commit, dispatch }, id) {
            dispatch('clearAllSelectedFiles');
            commit('setOpenedDropdown', id);
        },

        closeDropdown({ commit }) {
            commit('setOpenedDropdown', null);
        },

        openFileBrowserDropdown({ commit }) {
            commit('setOpenedDropdown', 'FileBrowser');
        },

        cancelUpload({ commit, state }, key) {
            const file = state.uploading.find((file) => file.Key === key);

            if (typeof file === 'object') {
                if (file.progress !== undefined && file.upload && file.progress > 0) {
                    file.upload.abort();
                }

                commit('finishUpload', key);
            } else {
                throw new Error(`File ${JSON.stringify(key)} not found`);
            }
        },

        closeAllInteractions({ state, dispatch }) {
            if (state.openedDropdown) {
                dispatch('closeDropdown');
            }

            if (state.selectedAnchorFile) {
                dispatch('clearAllSelectedFiles');
            }
        },

        clear({ commit }) {
            commit('clear');
        },
    },
});
