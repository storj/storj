// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Access Keys" />
        <PageSubtitleComponent subtitle="Create Access Grants, S3 Credentials, and API Keys." link="https://docs.storj.io/dcs/access" />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn @click="dialog = true">
                    <IconNew class="mr-2" size="16" bold />
                    New Access Key
                </v-btn>
            </v-row>
        </v-col>

        <AccessTableComponent />
    </v-container>
    <NewAccessDialog v-model="dialog" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';

import NewAccessDialog from '@poc/components/dialogs/CreateAccessDialog.vue';
import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import AccessTableComponent from '@poc/components/AccessTableComponent.vue';
import IconNew from '@poc/components/icons/IconNew.vue';

const dialog = ref<boolean>(false);
</script>
