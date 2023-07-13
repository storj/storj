// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" variant="flat" :border="true" rounded="xlg">
        <v-card-item>
            <v-progress-linear color="success" :model-value="progress" rounded height="6" />
        </v-card-item>
        <v-card-item>
            <v-row>
                <v-col>
                    <h4>{{ used }}</h4>
                    <p class="text-medium-emphasis"><small>{{ limit }}</small></p>
                </v-col>
                <v-col>
                    <h4 class="text-right">{{ available }}</h4>
                    <p class="text-right text-medium-emphasis"><small>{{ cta }}</small></p>
                </v-col>
            </v-row>
        </v-card-item>
    </v-card>
</template>

<script setup lang="ts">
import { VCard, VCardItem, VProgressLinear, VRow, VCol } from 'vuetify/components';

const props = defineProps<{
    title: string;
    progress: number;
    used: string;
    limit: string;
    available: string;
    cta: string;
}>();
</script>
