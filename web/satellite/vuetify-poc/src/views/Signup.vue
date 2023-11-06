// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="7" lg="5">
                <v-card title="Create your free account" subtitle="Get 25GB storage and 25GB download per month" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <v-form class="pt-4">
                            <v-select
                                v-model="select"
                                label="Satellite"
                                :items="items"
                                item-title="satellite"
                                :hint="`Recommended for ${select.hint}.`"
                                persistent-hint
                                return-object
                                chips
                                class="mb-5"
                            />

                            <v-text-field
                                label="Email address"
                                name="email"
                                type="email"
                                placeholder="Enter your email"
                                flat
                                clearable
                                required
                            >
                                <!-- <template v-slot:prepend-inner>
                  <v-icon
                    icon="mdi-email-outline"
                  />
                </template> -->
                            </v-text-field>

                            <v-text-field
                                v-model="password"
                                label="Password"
                                placeholder="Enter a password"
                                color="secondary"
                                :type="showPassword ? 'text' : 'password'"
                                :append-inner-icon="showPassword ? 'mdi-eye-outline' : 'mdi-eye-off-outline'"
                                @click:append-inner="showPassword = !showPassword"
                            >
                                <!-- <template v-slot:prepend-inner>
                  <v-icon
                    icon="mdi-key-outline"
                  />
                </template> -->
                            </v-text-field>

                            <v-checkbox density="compact" hide-details="true" required>
                                <template #label>
                                    <p class="text-body-2">I agree to the <a class="link">Terms of Service</a> and <a class="link">Privacy Policy</a>.</p>
                                </template>
                            </v-checkbox>

                            <v-checkbox density="compact">
                                <template #label>
                                    <p class="text-body-2">I want to receive the Storj newsletter by email.</p>
                                </template>
                            </v-checkbox>

                            <v-btn color="primary" size="large" block router-link to="/signup-confirmation">
                                Create your account
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-9 text-center text-body-2">Already have an account? <router-link class="link" to="/login">Login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import {
    VBtn,
    VCard,
    VCardText,
    VCheckbox,
    VCol,
    VContainer,
    VForm,
    VRow,
    VSelect,
    VTextField,
} from 'vuetify/components';
import { ref } from 'vue';

const valid = ref(false);
const checked = ref(false);
const showPassword = ref(false);
const email = ref('');
const password = ref('');
const select = ref({ satellite: 'US1', hint: 'North and South America' });
const items = ref([
    { satellite: 'US1', hint: 'North and South America' },
    { satellite: 'EU1', hint: 'Europe and Africa' },
    { satellite: 'AP1', hint: 'Asia and Australia' },
]);
</script>