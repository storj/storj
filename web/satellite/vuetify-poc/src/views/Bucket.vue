// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-container>

      <PageTitleComponent title="Browse Files"/>

      <BrowserBreadcrumbsComponent/>

      <v-col>
        <v-row class="mt-2 mb-4">

          <v-btn
            color="primary"
            min-width="120"
            @click="snackbar = true"
          >
            <BrowserSnackbarComponent :snackbar="snackbar" />
            <IconUpload/>
            Upload
          </v-btn>

          <v-btn
            variant="outlined"
            color="default"
            class="mx-4"
          >
            <IconFolder/>
            New Folder
            <BrowserNewFolderDialog/>
          </v-btn>

        </v-row>
      </v-col>

      <BrowserTableComponent/>

  </v-container>
</template>

<script>
import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import BrowserBreadcrumbsComponent from '@poc/components/BrowserBreadcrumbsComponent.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';
import BrowserTableComponent from '@poc/components/BrowserTableComponent.vue';
import BrowserNewFolderDialog from '@poc/components/BrowserNewFolderDialog.vue';
import IconUpload from '@poc/components/icons/IconUpload.vue';
import IconFolder from '@poc/components/icons/IconFolder.vue';

export default {
  components: {
    PageTitleComponent,
    BrowserBreadcrumbsComponent,
    BrowserSnackbarComponent,
    BrowserTableComponent,
    BrowserNewFolderDialog,
    IconUpload,
    IconFolder
  },
  data() {
    return {
      // upload snackbar
      snackbar: false,
    }
  }
};
</script>
