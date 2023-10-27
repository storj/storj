// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Project Information
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
                        <p>Edit the project name and description.</p>
                    </v-col>
                </v-row>
                <v-row>
                    <v-col cols="12">
                        <v-text-field model-value="My First Project" label="Project Name" hide-details="auto" />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field model-value="Description..." label="Project Description" hide-details="auto" />
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

    <v-snackbar v-model="snackbar" :timeout="7000" color="success">
        {{ text }}
        <template #actions>
            <v-btn color="default" variant="text" @click="snackbar = false">
                Close
            </v-btn>
        </template>
    </v-snackbar>
</template>

<script lang="ts">
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

export default {
    components: {
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
    },
    data() {
        return {
            snackbar: false,
            text: `Successfully updated the project information.`,
            dialog: false,
        };
    },
    methods: {
        onButtonClick() {
            this.snackbar = true;
            this.dialog = false;
        },
    },
};
</script>