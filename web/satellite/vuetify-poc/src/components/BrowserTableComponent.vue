// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" class="rounded-xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            hide-details
            clearable
        />

        <v-data-table
            v-model="selected"
            :sort-by="sortBy"
            :headers="headers"
            :items="files"
            :search="search"
            class="elevation-1"
            item-key="path"
            show-select
        >
            <template #item.name="{ item }">
                <div>
                    <v-btn
                        class="rounded-lg w-100 pl-1 pr-4 justify-start font-weight-bold"
                        variant="text"
                        height="40"
                        color="default"
                        @click="previewFile"
                    >
                        <img :src="icons.get(item.raw.icon) || fileIcon" alt="Item icon" class="mr-3">
                        {{ item.raw.name }}
                    </v-btn>
                </div>
            </template>
        </v-data-table>

        <v-dialog v-model="previewDialog" transition="fade-transition" class="preview-dialog" fullscreen theme="dark">
            <v-card class="preview-card">
                <v-carousel hide-delimiters show-arrows="hover" height="100vh">
                    <template #prev="{ props }">
                        <v-btn
                            color="default"
                            class="rounded-circle"
                            icon
                            @click="props.onClick"
                        >
                            <svg width="10" height="17" viewBox="0 0 10 17" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <g clip-path="url(#clip0_24843_332342)">
                                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.30725 8.23141C0.276805 7.67914 0.528501 7.04398 1.03164 6.54085L6.84563 0.726856C7.64837 -0.0758889 8.78719 -0.238577 9.38925 0.363481C9.99131 0.96554 9.82862 2.10436 9.02587 2.9071L3.71149 8.22148L9.02681 13.5368C9.82955 14.3395 9.99224 15.4784 9.39018 16.0804C8.78812 16.6825 7.6493 16.5198 6.84656 15.717L1.03257 9.90305C0.535173 9.40565 0.283513 8.77923 0.30725 8.23141Z" fill="white" />
                                </g>
                                <defs>
                                    <clipPath id="clip0_24843_332342">
                                        <rect width="17.0002" height="10" fill="white" transform="translate(10) rotate(90)" />
                                    </clipPath>
                                </defs>
                            </svg>
                        </v-btn>
                    </template>
                    <template #next="{ props }">
                        <v-btn
                            color="default"
                            class="rounded-circle"
                            icon
                            @click="props.onClick"
                        >
                            <svg width="10" height="17" viewBox="0 0 10 17" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <g clip-path="url(#clip0_24843_332338)">
                                    <path fill-rule="evenodd" clip-rule="evenodd" d="M9.69263 8.23141C9.72307 7.67914 9.47138 7.04398 8.96824 6.54085L3.15425 0.726856C2.35151 -0.0758889 1.21269 -0.238577 0.61063 0.363481C0.00857207 0.96554 0.17126 2.10436 0.974005 2.9071L6.28838 8.22148L0.973072 13.5368C0.170328 14.3395 0.00763934 15.4784 0.609698 16.0804C1.21176 16.6825 2.35057 16.5198 3.15332 15.717L8.96731 9.90305C9.46471 9.40565 9.71637 8.77923 9.69263 8.23141Z" fill="white" />
                                </g>
                                <defs>
                                    <clipPath id="clip0_24843_332338">
                                        <rect width="17.0002" height="10" fill="white" transform="matrix(4.37114e-08 1 1 -4.37114e-08 0 0)" />
                                    </clipPath>
                                </defs>
                            </svg>
                        </v-btn>
                    </template>
                    <v-toolbar
                        color="rgba(0, 0, 0, 0.3)"
                        theme="dark"
                    >
                        <!-- <v-img src="../assets/logo-white.svg" height="30" width="160" class="ml-3" alt="Storj Logo"/> -->
                        <v-toolbar-title>
                            Image.jpg
                        </v-toolbar-title>
                        <template #append>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-download.svg" width="22" alt="Download">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Download
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-share.svg" width="22" alt="Share">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Share
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-geo-distribution.svg" width="22" alt="Geographic Distribution">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Geographic Distribution
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-more.svg" width="22" alt="More">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    More
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white" @click="previewDialog = false">
                                <img src="@poc/assets/icon-close.svg" width="18" alt="Close">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Close
                                </v-tooltip>
                            </v-btn>
                            <!-- <v-btn icon="$close" color="white" size="small" @click="previewDialog = false"></v-btn> -->
                        </template>
                    </v-toolbar>
                    <v-carousel-item
                        v-for="(item,i) in items"
                        :key="i"
                        :src="item.src"
                    />
                </v-carousel>
            </v-card>
        </v-dialog>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VCard,
    VTextField,
    VDialog,
    VCarousel,
    VBtn,
    VToolbar,
    VToolbarTitle,
    VTooltip,
    VCarouselItem,
} from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import folderIcon from '@poc/assets/icon-folder-tonal.svg';
import pdfIcon from '@poc/assets/icon-pdf-tonal.svg';
import imageIcon from '@poc/assets/icon-image-tonal.svg';
import videoIcon from '@poc/assets/icon-video-tonal.svg';
import audioIcon from '@poc/assets/icon-audio-tonal.svg';
import textIcon from '@poc/assets/icon-text-tonal.svg';
import zipIcon from '@poc/assets/icon-zip-tonal.svg';
import spreadsheetIcon from '@poc/assets/icon-spreadsheet-tonal.svg';
import fileIcon from '@poc/assets/icon-file-tonal.svg';

const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);

const icons = new Map<string, string>([
    ['folder', folderIcon],
    ['pdf', pdfIcon],
    ['image', imageIcon],
    ['video', videoIcon],
    ['audio', audioIcon],
    ['text', textIcon],
    ['zip', zipIcon],
    ['spreadsheet', spreadsheetIcon],
    ['file', fileIcon],
]);

const items = [
    { src: 'https://cdn.vuetifyjs.com/images/carousel/squirrel.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/sky.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/bird.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/planet.jpg' },
];
const sortBy = [{ key: 'name', order: 'asc' }];
const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Type', key:'type' },
    { title: 'Size', key: 'size' },
    { title: 'Date', key: 'date' },
];
const files = [
    {
        name: 'Be The Cloud',
        path: 'folder-1',
        type: 'Folder',
        size: '2 GB',
        date: '02 Mar 2023',
        icon: 'folder',
    },
    {
        name: 'Folder',
        path: 'folder-2',
        type: 'Folder',
        size: '458 MB',
        date: '21 Apr 2023',
        icon: 'folder',
    },
    {
        name: 'Presentation.pdf',
        path: 'Presentation.pdf',
        type: 'PDF',
        size: '150 KB',
        date: '24 Mar 2023',
        icon: 'pdf',
    },
    {
        name: 'Image.jpg',
        path: 'image.jpg',
        type: 'JPG',
        size: '500 KB',
        date: '12 Mar 2023',
        icon: 'image',
    },
    {
        name: 'Video.mp4',
        path: 'video.mp4',
        type: 'MP4',
        size: '3 MB',
        date: '01 Apr 2023',
        icon: 'video',
    },
    {
        name: 'Song.mp3',
        path: 'Song.mp3',
        type: 'MP3',
        size: '8 MB',
        date: '22 May 2023',
        icon: 'audio',
    },
    {
        name: 'Text.txt',
        path: 'text.txt',
        type: 'TXT',
        size: '2 KB',
        date: '21 May 2023',
        icon: 'text',
    },
    {
        name: 'NewArchive.zip',
        path: 'newarchive.zip',
        type: 'ZIP',
        size: '21 GB',
        date: '20 May 2023',
        icon: 'zip',
    },
    {
        name: 'Table.csv',
        path: 'table.csv',
        type: 'CSV',
        size: '3 MB',
        date: '20 May 2023',
        icon: 'spreadsheet',
    },
    {
        name: 'Map-export.json',
        path: 'map-export.json',
        type: 'JSON',
        size: '1 MB',
        date: '23 May 2023',
        icon: 'file',
    },
];

function previewFile(): void {
    // Implement logic to fetch the file content for preview or generate a URL for preview
    // Then, open the preview dialog
    previewDialog.value = true;
}
</script>
