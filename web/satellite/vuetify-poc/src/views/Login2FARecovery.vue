// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="7" lg="5">
                <v-card title="Enter your recovery code" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Enter one of your recovery codes to continue.</p>
                        <v-form>
                            <v-text-field
                                label="Recovery Code"
                                class="mt-5"
                            />
                            <v-btn color="primary" size="large" block router-link to="/projects">
                                Continue
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Not a member? <router-link class="link" to="/signup">Signup</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow, VTextField } from 'vuetify/components';
</script>