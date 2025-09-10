// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="SSH Keys" />
        <PageSubtitleComponent
            subtitle="SSH keys are used to securely access virtual machines that support key-based authentication."
            link="https://docs.storj.io/dcs/access"
        />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn :prepend-icon="PlusCircle">
                    New SSH Key
                </v-btn>
            </v-row>
        </v-col>

        <ComputeKeysTableComponent />
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';
import { PlusCircle } from 'lucide-vue-next';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ComputeKeysTableComponent from '@/components/ComputeKeysTableComponent.vue';
</script>
