// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5">
                <v-card title="Did you forgot your password?" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Select your account satellite, enter your email address, and we will send you a password reset link.</p>
                        <v-form class="pt-4">
                            <v-select
                                v-model="select"
                                label="Satellite"
                                :items="items"
                                item-title="satellite"
                                return-object
                                chips
                                class="mt-3 mb-2"
                            />
                            <v-text-field
                                label="Email address"
                                name="email"
                                type="email"
                                autofocus
                                clearable
                                required
                                class="my-2"
                            />
                            <v-btn color="primary" size="large" block router-link to="/password-reset-confirmation">
                                Request Password Reset
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Go back to <router-link class="link" to="/login">login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">

import { VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow, VSelect, VTextField } from 'vuetify/components';
import { ref } from 'vue';

const valid = ref(false);
const checked = ref(false);
const email = ref('');
const select = ref({ satellite: 'US1', hint: 'North and South America' });
const items = ref([
    { satellite: 'US1', hint: 'North and South America' },
    { satellite: 'EU1', hint: 'Europe and Africa' },
    { satellite: 'AP1', hint: 'Asia and Australia' },
]);
</script>