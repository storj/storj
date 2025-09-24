// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="200"
        persistent
    >
        <v-list
            class="py-2"
            color="primary"
            elevation="4"
            rounded="lg"
        >
            <v-list-item title="Please wait...">
                <template #append>
                    <v-progress-circular
                        color="primary"
                        indeterminate="disable-shrink"
                        size="16"
                        width="2"
                    />
                </template>
            </v-list-item>
        </v-list>
    </v-dialog>
</template>

<script setup lang="ts">
import { VDialog, VList, VListItem, VProgressCircular } from 'vuetify/components';

const model = defineModel<boolean>({ required: true });
</script>