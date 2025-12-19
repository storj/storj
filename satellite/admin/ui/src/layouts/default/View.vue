// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-main>
        <v-container :max-width="xlAndUp ? '85%' : '100%'">
            <router-view />
        </v-container>
    </v-main>
</template>

<script setup lang="ts">
import { VMain, VContainer } from 'vuetify/components';
import { useDisplay } from 'vuetify';

const { xlAndUp } = useDisplay();
</script>
