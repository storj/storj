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
            <v-row class="mt-1 mb-2">
                <v-btn :prepend-icon="CirclePlus" @click="createNewDomain">
                    New Domain
                </v-btn>
            </v-row>
        </v-col>

        <DomainsTableComponent />

        <NewDomainDialog v-model="isNewDomainDialog" />
        <PromptOwnerUpgradeDialog v-model="isPromptToUpgradeDialog" />
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';
import { CirclePlus } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import DomainsTableComponent from '@/components/DomainsTableComponent.vue';
import NewDomainDialog from '@/components/dialogs/NewDomainDialog.vue';
import PromptOwnerUpgradeDialog from '@/components/dialogs/PromptOwnerUpgradeDialog.vue';

const projectsStore = useProjectsStore();

const isNewDomainDialog = ref<boolean>(false);
const isPromptToUpgradeDialog = ref<boolean>(false);

const projectCfg = computed(() => projectsStore.selectedProjectConfig);

function createNewDomain(): void {
    if (!projectCfg.value.hasPaidPrivileges) {
        isPromptToUpgradeDialog.value = true;
        return;
    }

    isNewDomainDialog.value = true;
}
</script>
