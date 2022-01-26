// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/* eslint-disable */

import S3 from "aws-sdk/clients/s3";

const listCache = new Map();

interface BrowserObject {
	Key: string;
	Size: number;
	LastModified: number;
}

export default {
    namespaced: true,
    state: {
        s3: null,
        accessKey: null,

        path: "",
        bucket: "",
        browserRoot: "/",
        files: [],
        uploadChain: Promise.resolve(),
        uploading: [],
        selectedAnchorFile: null,
        unselectedAnchorFile: null,
        selectedFiles: [],
        shiftSelectedFiles: [],
        filesToBeDeleted: [],
        fetchSharedLink: null,
        fetchObjectMap: null,
        fetchObjectPreview: null,
        openedDropdown: null,
        headingSorted: "name",
        orderBy: "asc",
        createFolderInputShow: false,
        openModalOnFirstUpload: false,

        modalPath: null,
        fileShareModal: null
    },
    getters: {
        sortedFiles: (state) => {
            // key-specific sort cases
            const fns = {
                date: (a: BrowserObject, b: BrowserObject): number =>
                    (new Date(a.LastModified)).getTime() - (new Date(b.LastModified)).getTime(),
                name: (a: BrowserObject, b: BrowserObject): number => a.Key.localeCompare(b.Key),
                size: (a: BrowserObject, b: BrowserObject): number => a.Size - b.Size
            };

            // TODO(performance): avoid several passes over the slice.

            // sort by appropriate function
            const sortedFiles = state.files.slice();
            sortedFiles.sort(fns[state.headingSorted]);
            // reverse if descending order
            if(state.orderBy !== "asc") {
                sortedFiles.reverse();
            }

            // display folders and then files
            const groupedFiles = [
                ...sortedFiles.filter((file) => file.type === "folder"),
                ...sortedFiles.filter((file) => file.type === "file")
            ];

            return groupedFiles;
        },

        isInitialized: (state) => state.s3 !== null
    },
    mutations: {
        init(
            state,
            {
                accessKey,
                secretKey,
                bucket,
                endpoint = "https://gateway.tardigradeshare.io",
                browserRoot,
                openModalOnFirstUpload = true,
                fetchSharedLink = () => "javascript:null",
                fetchObjectPreview = () => "javascript:null",
                fetchObjectMap = () =>
                    new Promise((resolve) =>
                        setTimeout(
                            () =>
                                resolve(
                                    "https://link.us1.storjshare.io/s/jx7t2i4lky36b3pomls6upakdzba/filebrowser%2Fsto-1.jpeg?map=1"
                                ),
                            1000
                        )
                    )
            }
        ) {
            const s3Config = {
                accessKeyId: accessKey,
                secretAccessKey: secretKey,
                endpoint,
                s3ForcePathStyle: true,
                signatureVersion: "v4",
                connectTimeout: 0,
                httpOptions: { timeout: 0 }
            };

            state.s3 = new S3(s3Config);
            state.accessKey = accessKey;
            state.bucket = bucket;
            state.browserRoot = browserRoot;
            state.openModalOnFirstUpload = openModalOnFirstUpload;
            state.fetchSharedLink = fetchSharedLink;
            state.fetchObjectMap = fetchObjectMap;
            state.fetchObjectPreview = fetchObjectPreview;
            state.path = "";
        },

        updateFiles(state, { path, files }) {
            state.path = path;
            state.files = files;
        },

        setSelectedFiles(state, files) {
            state.selectedFiles = files;
        },

        setSelectedAnchorFile(state, file) {
            state.selectedAnchorFile = file;
        },

        setUnselectedAnchorFile(state, file) {
            state.unselectedAnchorFile = file;
        },

        setFilesToBeDeleted(state, files) {
            state.filesToBeDeleted = [...state.filesToBeDeleted, ...files];
        },

        removeFileToBeDeleted(state, file) {
            state.filesToBeDeleted = state.filesToBeDeleted.filter(
                (singleFile) => singleFile.Key !== file.Key
            );
        },

        removeAllFilesToBeDeleted(state) {
            state.filesToBeDeleted = [];
        },

        removeAllSelectedFiles(state) {
            state.selectedAnchorFile = null;
            state.unselectedAnchorFile = null;
            state.shiftSelectedFiles = [];
            state.selectedFiles = [];
        },

        setShiftSelectedFiles(state, files) {
            state.shiftSelectedFiles = files;
        },

        pushUpload(state, file) {
            state.uploading.push(file);
        },

        setProgress(state, { Key, progress }) {
            state.uploading.find((file) => file.Key === Key).progress =
				progress;
        },

        finishUpload(state, Key) {
            state.uploading = state.uploading.filter(
                (file) => file.Key !== Key
            );
        },

        setOpenedDropdown(state, id) {
            state.openedDropdown = id;
        },

        sort(state, headingSorted) {
            const flip = (orderBy) => (orderBy === "asc" ? "desc" : "asc");

            state.orderBy =
				state.headingSorted === headingSorted
				    ? flip(state.orderBy)
				    : "asc";
            state.headingSorted = headingSorted;
        },

        setCreateFolderInputShow(state, value) {
            state.createFolderInputShow = value;
        },

        openModal(state, path) {
            state.modalPath = path;
        },

        closeModal(state) {
            state.modalPath = null;
        },

        setFileShareModal(state, path) {
            state.fileShareModal = path;
        },

        closeFileShareModal(state) {
            state.fileShareModal = null;
        },

        addUploadToChain(state, fn) {
            state.uploadChain = state.uploadChain.then(fn);
        }
    },
    actions: {
        async list({ commit, state }, path = state.path) {
            if (listCache.has(path) === true) {
                commit("updateFiles", {
                    path,
                    files: listCache.get(path)
                });
            }

            const response = await state.s3
                .listObjects({
                    Bucket: state.bucket,
                    Delimiter: "/",
                    Prefix: path
                })
                .promise();

            const { Contents, CommonPrefixes } = response;

            Contents.sort((a, b) =>
                a.LastModified < b.LastModified ? -1 : -1
            );

            const prefixToFolder = ({ Prefix }) => ({
                Key: Prefix.slice(path.length, -1),
                LastModified: new Date(0),
                type: "folder"
            });

            const makeFileRelative = (file) => ({
                ...file,
                Key: file.Key.slice(path.length),
                type: "file"
            });

            const isFileVisible = (file) =>
                file.Key.length > 0 && file.Key !== ".file_placeholder";

            const files = [
                ...CommonPrefixes.map(prefixToFolder),
                ...Contents.map(makeFileRelative).filter(isFileVisible)
            ];

            listCache.set(path, files);
            commit("updateFiles", {
                path,
                files
            });
        },

        async back({ state, dispatch }) {
            const getParentDirectory = (path) => {
                let i = path.length - 2;

                while (path[i - 1] !== "/" && i > 0) {
                    i--;
                }

                return path.slice(0, i);
            };

            dispatch("list", getParentDirectory(state.path));
        },

        async upload({ commit, state, dispatch }, e) {
            const items = e.dataTransfer
                ? e.dataTransfer.items
                : e.target.files;

            async function* traverse(item, path = "") {
                if (item.isFile) {
                    const file = await new Promise(item.file.bind(item));
                    yield { path, file };
                } else if (item instanceof File) {
                    let relativePath = (item as any).webkitRelativePath.split("/").slice(0, -1).join("/");

                    if (relativePath.length) {
                        relativePath += "/";
                    }

                    yield { path: relativePath, file: item };
                } else if (item.isDirectory) {
                    const dirReader = item.createReader();

                    const entries = await new Promise(
                        dirReader.readEntries.bind(dirReader)
                    ) as any[];
                    for (const entry of entries) {
                        yield* traverse(entry, path + item.name + "/");
                    }
                } else if (typeof item.length === "number") {
                    for (const i of item) {
                        yield* traverse(i);
                    }
                } else {
                    throw new Error("Item is not directory or file");
                }
            }

            const iterator =
				items instanceof FileList
				    ? [...items]
				    : [...items].map(
				        (item) =>
				            item.webkitGetAsEntry() || item.getAsEntry()
					  );

            const fileNames = state.files.map((file) => file.Key);

            function getUniqueFileName(fileName) {
                for (let count = 1; fileNames.includes(fileName); count++) {
                    if (count > 1) {
                        fileName = fileName.replace(
                            /\((\d+)\)(.*)/,
                            `(${count})$2`
                        );
                    } else {
                        fileName = fileName.replace(
                            /([^.]*)(.*)/,
                            `$1 (${count})$2`
                        );
                    }
                }

                return fileName;
            }

            for await (const { path, file } of traverse(iterator)) {
                const directories = path.split("/");
                const uniqueFirstDirectory = getUniqueFileName(directories[0]);
                directories[0] = uniqueFirstDirectory;

                const fileName = getUniqueFileName(
                    directories.join("/") + file.name
                );

                const params = {
                    Bucket: state.bucket,
                    Key: state.path + fileName,
                    Body: file
                };

                const upload = state.s3.upload(
                    { ...params },
                    { partSize: 64 * 1024 * 1024 }
                );

                upload.on("httpUploadProgress", (progress) => {
                    commit("setProgress", {
                        Key: params.Key,
                        progress: Math.round(
                            (progress.loaded / progress.total) * 100
                        )
                    });
                });

                commit("pushUpload", {
                    ...params,
                    upload,
                    progress: 0
                });

                commit("addUploadToChain", async () => {
                    if (
                        state.uploading.findIndex(
                            (file) => file.Key === params.Key
                        ) === -1
                    ) {
                        // upload cancelled or removed
                        return -1;
                    }

                    try {
                        await upload.promise();
                    } catch (e) {
                        // An error is raised if the upload is aborted by the user
                        console.log(e);
                    }

                    await dispatch("list");

                    const uploadedFiles = state.files.filter(
                        (file) => file.type === "file"
                    );

                    if (uploadedFiles.length === 1) {
                        const [{ Key }] = uploadedFiles;

                        if (state.openModalOnFirstUpload === true) {
                            commit("openModal", params.Key);
                        }
                    }

                    commit("finishUpload", params.Key);
                });
            }
        },

        async createFolder({ state, dispatch }, name) {
            await state.s3
                .putObject({
                    Bucket: state.bucket,
                    Key: state.path + name + "/.file_placeholder"
                })
                .promise();

            dispatch("list");
        },

        async delete({ commit, dispatch, state }, { path, file, folder }) {
            await state.s3
                .deleteObject({
                    Bucket: state.bucket,
                    Key: path + file.Key
                })
                .promise();

            if (!folder) {
                await dispatch("list");
                commit("removeFileToBeDeleted", file);
            }
        },

        async deleteFolder({ commit, dispatch, state }, { file, path }) {
            async function recurse(filePath) {
                const { Contents, CommonPrefixes } = await state.s3
                    .listObjects({
                        Bucket: state.bucket,
                        Delimiter: "/",
                        Prefix: filePath
                    })
                    .promise();

                async function thread() {
                    while (Contents.length) {
                        const file = Contents.pop();

                        await dispatch("delete", {
                            path: "",
                            file,
                            folder: true
                        });
                    }
                }

                await Promise.all([thread(), thread(), thread()]);

                for (const { Prefix } of CommonPrefixes) {
                    await recurse(Prefix);
                }
            }

            await recurse(path.length > 0 ? path + file.Key : file.Key + "/");

            commit("removeFileToBeDeleted", file);
            await dispatch("list");
        },

        async deleteSelected({ rootState, state, dispatch, commit }) {
            const filesToDelete = [
                ...state.selectedFiles,
                ...state.shiftSelectedFiles
            ];

            if (state.selectedAnchorFile) {
                filesToDelete.push(state.selectedAnchorFile);
            }

            commit("setFilesToBeDeleted", filesToDelete);

            await Promise.all(
                filesToDelete.map(async (file) => {
                    if (file.type === "file")
                        await dispatch("delete", {
                            file,
                            path: rootState.files.path
                        });
                    else
                        await dispatch("deleteFolder", {
                            file,
                            path: rootState.files.path
                        });
                })
            );

            dispatch("clearAllSelectedFiles");
        },

        async download({ state }, file) {
            const url = state.s3.getSignedUrl("getObject", {
                Bucket: state.bucket,
                Key: state.path + file.Key
            });

            const downloadURL = function (data, fileName) {
                let a;
                a = document.createElement("a");
                a.href = data;
                a.download = fileName;
                document.body.appendChild(a);
                a.style = "display: none";
                a.click();
                a.remove();
            };

            downloadURL(url, file.Key);
        },

        updateSelectedFiles({ commit }, files) {
            commit("setSelectedFiles", [...files]);
        },

        updateShiftSelectedFiles({ commit }, files) {
            commit("setShiftSelectedFiles", files);
        },

        addFileToBeDeleted({ commit }, file) {
            commit("setFilesToBeDeleted", [file]);
        },

        removeFileFromToBeDeleted({ commit }, file) {
            commit("removeFileToBeDeleted", file);
        },

        clearAllSelectedFiles({ commit, state }) {
            if (state.selectedAnchorFile || state.unselectedAnchorFile) {
                commit("removeAllSelectedFiles");
            }
        },

        openDropdown({ commit, dispatch }, id) {
            dispatch("clearAllSelectedFiles");
            commit("setOpenedDropdown", id);
        },

        closeDropdown({ commit }) {
            commit("setOpenedDropdown", null);
        },

        openFileBrowserDropdown({ commit }) {
            commit("setOpenedDropdown", "FileBrowser");
        },

        updateCreateFolderInputShow({ commit }, value) {
            commit("setCreateFolderInputShow", value);
        },

        cancelUpload({ commit, state }, key) {
            const file = state.uploading.find((file) => file.Key === key);

            if (typeof file === "object") {
                // if the file has already started uploading, then abort
                if (file.progress > 0) {
                    file.upload.abort();
                }

                commit("finishUpload", key);
            } else {
                throw new Error(`File ${JSON.stringify(key)} not found`);
            }
        },

        closeAllInteractions({ commit, state, dispatch }) {
            if (state.modalPath) {
                commit("closeModal");
            }

            if (state.fileShareModal) {
                commit("closeFileShareModal");
            }

            if (state.openedDropdown) {
                dispatch("closeDropdown");
            }

            if (state.selectedAnchorFile) {
                dispatch("clearAllSelectedFiles");
            }
        }
    }
};
