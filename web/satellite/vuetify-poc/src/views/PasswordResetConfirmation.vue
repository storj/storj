// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card title="Check your email" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p class="mb-4">We have sent you the password reset instructions to your email address.</p>
                        <v-btn color="primary" size="large" block router-link to="/login">
                            Go to login
                        </v-btn>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Didn't get the password reset email?</p>
                <p class="text-center text-body-2">Make sure you selected the correct satellite.</p>
                <p class="text-center text-body-2">Go back to <router-link class="link" to="/forgot-password">password reset</router-link>.</p>
                <p class="pt-6 text-center text-body-2">Still having problems? <a class="link" href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000379291" target="_blank" rel="noopener noreferrer">Contact Support</a></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VRow } from 'vuetify/components';
import { ref } from 'vue';

const valid = ref(false);
const checked = ref(false);
const email = ref('');
const select = ref({ satellite: 'US1' });
const items = ref([
    { satellite: 'US1' },
    { satellite: 'EU1' },
    { satellite: 'AP1' },
]);
</script>