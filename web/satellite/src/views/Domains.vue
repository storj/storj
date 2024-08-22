// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Domains" />
        <PageSubtitleComponent
            subtitle="Setup secure custom domains (HTTPS) for your shared content."
            link="https://docs.storj.io/dcs/code/static-site-hosting/custom-domains"
        />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn :prepend-icon="CirclePlus">
                    New Domain
                    <NewDomainDialog v-model="isNewDomainDialog" />
                </v-btn>
            </v-row>
        </v-col>

        <DomainsTableComponent />
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';
import { CirclePlus } from 'lucide-vue-next';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import DomainsTableComponent from '@/components/DomainsTableComponent.vue';
import NewDomainDialog from '@/components/dialogs/NewDomainDialog.vue';

const isNewDomainDialog = ref<boolean>(false);
</script>
