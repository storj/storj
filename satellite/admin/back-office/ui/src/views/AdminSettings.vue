// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="6">
                <PageTitleComponent title="Admin Settings" />
                <PageSubtitleComponent subtitle="View and change your account settings." />
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" md="4">
                <v-card title="Account" subtitle="Tome Boshevski" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2 font-weight-medium">tome@storj.io</v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" size="small" color="default">
                            Edit
                            <AdminAccountDialog />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Role" subtitle="Account Type" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2 font-weight-medium">Admin</v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" size="small" color="default">
                            Edit
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Security" subtitle="Two-factor Authentication" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2 font-weight-medium">Disabled</v-chip>
                        <v-divider class="my-4" />
                        <v-btn size="small" color="primary">
                            Enable 2FA
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VRow,
    VCol,
    VCard,
    VCardText,
    VChip,
    VDivider,
    VBtn,
} from 'vuetify/components';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import AdminAccountDialog from '@/components/AdminAccountDialog.vue';
</script>
