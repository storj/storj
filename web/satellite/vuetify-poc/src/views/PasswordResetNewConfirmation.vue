// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5">
                <v-card title="Password Updated" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p class="mb-2">Your password was updated successfully.</p>
                        <p class="mb-6">You can now login with your new password.</p>
                        <v-btn color="primary" size="large" block router-link to="/login">
                            Go to login
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VRow } from 'vuetify/components';
</script>