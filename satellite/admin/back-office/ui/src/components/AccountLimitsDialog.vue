// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Account Default Limits
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn icon="$close" variant="text" size="small" color="default" @click="dialog = false" />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form v-model="valid" class="pa-7">
                <v-row>
                    <v-col cols="12">
                        <p>Enter default limits per project for this account.</p>
                    </v-col>
                </v-row>

                <v-row>
                    <v-col cols="12" md="6">
                        <v-text-field
                            label="Total Projects" model-value="3" suffix="Projects" variant="outlined"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field
                            label="Storage / project" model-value="100" suffix="TB" variant="outlined"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field
                            label="Download / month / project" model-value="300" suffix="TB" variant="outlined"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field
                            label="Segments / project" model-value="100,000,000" variant="outlined"
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field label="Rate / project" model-value="100" variant="outlined" hide-details="auto" />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field label="Burst / project" model-value="10,000" variant="outlined" hide-details="auto" />
                    </v-col>
                </v-row>
                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            model-value="itacker@gmail.com" label="Account Email" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="loading" @click="onButtonClick">Save</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar" :timeout="7000" color="error">
        Error. Cannot change limits.
        <template #actions>
            <v-btn color="default" variant="text" @click="snackbar = false">
                Close
            </v-btn>
        </template>
    </v-snackbar>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VTextField,
    VCardActions,
    VSnackbar,
} from 'vuetify/components';

const snackbar = ref<boolean>(false);
const dialog = ref<boolean>(false);

function onButtonClick() {
    snackbar.value = true;
    dialog.value = false;
}
</script>