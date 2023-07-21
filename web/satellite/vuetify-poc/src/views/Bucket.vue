// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Browse Files" />

        <BrowserBreadcrumbsComponent />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn
                    color="primary"
                    min-width="120"
                    @click="snackbar = true"
                >
                    <BrowserSnackbarComponent :on-cancel="() => { snackbar = false }" />
                    <IconUpload />
                    Upload
                </v-btn>

                <v-btn
                    variant="outlined"
                    color="default"
                    class="mx-4"
                >
                    <IconFolder />
                    New Folder
                    <BrowserNewFolderDialog />
                </v-btn>
            </v-row>
        </v-col>

        <BrowserTableComponent />
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VContainer, VCol, VRow, VBtn } from 'vuetify/components';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import BrowserBreadcrumbsComponent from '@poc/components/BrowserBreadcrumbsComponent.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';
import BrowserTableComponent from '@poc/components/BrowserTableComponent.vue';
import BrowserNewFolderDialog from '@poc/components/dialogs/BrowserNewFolderDialog.vue';
import IconUpload from '@poc/components/icons/IconUpload.vue';
import IconFolder from '@poc/components/icons/IconFolder.vue';

const snackbar = ref<boolean>(false);
</script>
