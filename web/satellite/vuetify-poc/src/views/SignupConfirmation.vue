// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="7" lg="5">
                <v-card title="Check your inbox" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Enter the 6 digit confirmation code you received in your email to verify your account:</p>
                        <v-form>
                            <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                                <v-otp-input v-model="otp" :loading="loading" autofocus class="my-2" />
                            </v-card>

                            <v-btn :disabled="otp.length < 6 || loading" color="primary" size="large" block router-link to="/projects">
                                Verify Account
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-9 text-center text-body-2">Didn't receive a verification email? <router-link class="link" to="/signup">Resend</router-link></p>
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