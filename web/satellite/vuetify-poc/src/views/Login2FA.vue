// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="7" lg="5">
                <v-card title="Enter your 2FA code" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Enter the 6 digit code from your two factor authenticator application to continue.</p>
                        <v-form>
                            <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                                <v-otp-input v-model="otp" :loading="loading" autofocus class="my-2" />
                            </v-card>

                            <v-btn
                                router-link to="/projects"
                                :disabled="otp.length < 6"
                                color="primary"
                                block
                            >
                                <span v-if="otp.length === 0">6 digits left</span>

                                <span v-else-if="otp.length < 6">
                                    {{ 6 - otp.length }}
                                    digits left
                                </span>

                                <span v-else>
                                    Verify
                                </span>
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-9 text-center text-body-2">Or use a <router-link class="link" to="/login-2fa-recovery">recovery code</router-link></p>
                <p class="pt-6 text-center text-body-2">Not a member? <router-link class="link" to="/signup">Signup</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow } from 'vuetify/components';
import { VOtpInput } from 'vuetify/labs/components';
import { ref } from 'vue';

const loading = ref(false);
const otp = ref('');
</script>