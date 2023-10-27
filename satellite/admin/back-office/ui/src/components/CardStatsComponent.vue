// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="title" :subtitle="subtitle" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-chip :variant="variant" :color="color" class="font-weight-bold">{{ data }}</v-chip>
        </v-card-text>
    </v-card>
</template>

<script lang="ts">
import { VCard, VCardText, VChip } from 'vuetify/components';

export default {
    components: {
        VCard,
        VCardText,
        VChip,
    },
    props: {
        title: {
            type: String,
            required: true,
        },
        subtitle: {
            type: String,
            required: true,
        },
        data: {
            type: String,
            required: true,
        },
        variant: {
            type: String,
            required: true,
        },
        color: {
            type: String,
            required: true,
        },
    },
};
</script>