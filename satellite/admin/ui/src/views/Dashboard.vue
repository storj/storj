// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <PageTitleComponent title="Dashboard" />
        <PageSubtitleComponent subtitle="Welcome to the Storj Admin UI for North America US1." />

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent
                    title="All Accounts" subtitle="Satellite Total" data="218,748" color="default" router-link
                    to="/accounts"
                />-->
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent
                    title="Enterprise" subtitle="Accounts" data="3,405" color="default" router-link
                    to="/accounts"
                />-->
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent
                    title="Priority" subtitle="Accounts" data="5,224" color="default" router-link
                    to="/accounts"
                />-->
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent title="Pro" subtitle="Accounts" data="82,386" color="default" router-link to="/accounts" />-->
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent title="Free" subtitle="Accounts" data="123,480" color="default" router-link to="/accounts" />-->
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <!--                <CardStatsComponent title="Suspended" subtitle="Accounts" data="1" color="default" router-link to="/accounts" />-->
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <v-card title="Limits" subtitle="Projects with more than 80% usage." border rounded="xlg">
                    <v-divider class="mb-1" />
                    <DashboardLimitsTableComponent />
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VContainer, VRow, VCol, VCard, VDivider } from 'vuetify/components';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import DashboardLimitsTableComponent from '@/components/DashboardLimitsTableComponent.vue';
</script>