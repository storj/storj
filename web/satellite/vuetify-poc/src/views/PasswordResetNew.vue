// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5">
                <v-card title="Password Reset" subtitle="Enter your new password." class="pa-2 pa-sm-7">
                    <v-card-text>
                        <v-form class="pt-4">
                            <v-select
                                v-model="select"
                                label="Satellite"
                                :items="items"
                                item-title="satellite"
                                readonly
                                chips
                                disabled
                            />
                            <v-text-field
                                label="Email address"
                                name="email"
                                type="email"
                                required
                                readonly
                                model-value="email@email.com"
                                disabled
                            />

                            <v-text-field
                                v-model="password"
                                label="New Password"
                                placeholder="Enter a new password"
                                :type="showPassword ? 'text' : 'password'"
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="showPassword"
                                        type="password"
                                        @toggleVisibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-text-field
                                v-model="password"
                                label="Confirm Password"
                                placeholder="Enter the new password again"
                                :type="showPassword ? 'text' : 'password'"
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="showPassword"
                                        type="password"
                                        @toggleVisibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-btn color="primary" size="large" block router-link to="/password-reset-new-confirmation">
                                Save New Password
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

import PasswordInputEyeIcons from '@poc/components/PasswordInputEyeIcons.vue';

const valid = ref(false);
const checked = ref(false);
const showPassword = ref(false);
const password = ref('');
const email = ref('');
const select = ref({ satellite: 'US1' });
const items = ref([
    { satellite: 'US1' },
    { satellite: 'EU1' },
    { satellite: 'AP1' },
]);
</script>