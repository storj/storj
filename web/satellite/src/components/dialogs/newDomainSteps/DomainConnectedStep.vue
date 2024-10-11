// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-3" @submit.prevent>
        <v-card-text>
            <v-alert type="success" variant="tonal">
                You should now be able to access your content using your custom domain.
            </v-alert>
            <v-alert type="info" variant="tonal" class="my-4">
                DNS propagation usually takes less than a few hours, but can take up to 48 hours in some cases.
            </v-alert>
        </v-card-text>
    </v-form>
</template>

<script setup lang="ts">
import { VAlert, VCardText, VForm } from 'vuetify/components';
</script>
