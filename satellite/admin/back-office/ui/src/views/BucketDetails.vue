// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="12" md="8">
                <PageTitleComponent title="Bucket Details" />

                <v-chip variant="outlined" color="default" class="mr-2 mb-2 mb-md-0 pr-4" router-link to="/account-details">
                    <v-tooltip activator="parent" location="top">
                        Go to account
                    </v-tooltip>
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12.1271 6C14.1058 6 15.7099 7.60408 15.7099 9.58281C15.7099 10.7701 15.1324 11.8226 14.2429 12.4745C16.0273 13.1299 17.4328 14.5717 18.0402 16.3804C18.059 16.4363 18.077 16.4925 18.0942 16.5491L18.1195 16.6342C18.2377 17.0429 18.0022 17.4701 17.5934 17.5883C17.5239 17.6084 17.4518 17.6186 17.3794 17.6186H6.764C6.34206 17.6186 6 17.2765 6 16.8545C6 16.7951 6.00695 16.7358 6.02069 16.678L6.02974 16.6434C6.05458 16.5571 6.08121 16.4714 6.10959 16.3866C6.7237 14.5517 8.15871 13.0936 9.97792 12.4495C9.10744 11.7961 8.54432 10.7552 8.54432 9.58281C8.54432 7.60408 10.1484 6 12.1271 6ZM12.076 13.2168C9.95096 13.2168 8.07382 14.5138 7.29168 16.4355L7.26883 16.4925H16.8831L16.8826 16.4916C16.1224 14.5593 14.2607 13.2444 12.1429 13.2173L12.076 13.2168ZM12.1271 7.12603C10.7703 7.12603 9.67035 8.22596 9.67035 9.58281C9.67035 10.9397 10.7703 12.0396 12.1271 12.0396C13.4839 12.0396 14.5839 10.9397 14.5839 9.58281C14.5839 8.22596 13.4839 7.12603 12.1271 7.12603Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    itacker@gmail.com
                </v-chip>

                <img src="@/assets/icon-right.svg" alt="Project" width="10" class="mr-2">

                <v-chip variant="outlined" color="default" class="mr-2 mb-2 mb-md-0 pr-4" router-link to="/project-details">
                    <v-tooltip activator="parent" location="top">
                        Go to project
                    </v-tooltip>
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M16.2231 7.08668L16.2547 7.10399L23.4149 11.2391C23.6543 11.3774 23.7829 11.6116 23.8006 11.8529L23.8021 11.8809L23.8027 11.9121V20.1078C23.8027 20.3739 23.6664 20.6205 23.4432 20.7624L23.4136 20.7803L16.2533 24.8968C16.0234 25.029 15.7426 25.0342 15.5088 24.9125L15.4772 24.8951L8.38642 20.7787C8.15725 20.6457 8.01254 20.4054 8.00088 20.1422L8 20.1078L8.00026 11.8975L8 11.8738C8.00141 11.6177 8.12975 11.3687 8.35943 11.2228L8.38748 11.2058L15.4783 7.10425C15.697 6.97771 15.9622 6.96636 16.1893 7.07023L16.2231 7.08668ZM22.251 13.2549L16.6424 16.4939V22.8832L22.251 19.6588V13.2549ZM9.55175 13.2614V19.6611L15.0908 22.8766V16.4916L9.55175 13.2614ZM15.8669 8.67182L10.2916 11.8967L15.8686 15.149L21.4755 11.9109L15.8669 8.67182Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    56F82SR21Q284
                </v-chip>

                <img src="@/assets/icon-right.svg" alt="Project" width="10" class="mr-2">

                <v-chip class="mr-2 mr-2 mb-2 mb-md-0 font-weight-medium pr-4" variant="tonal" color="default">
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M23.3549 14.5467C24.762 15.9538 24.7748 18.227 23.3778 19.624C22.8117 20.1901 22.1018 20.5247 21.3639 20.6289L21.1106 22.0638C21.092 23.1897 18.7878 24.1 15.9481 24.1C13.1254 24.1 10.8319 23.2006 10.7863 22.0841L10.7858 22.0638L8.84466 11.066C8.82281 10.9882 8.80883 10.9095 8.80304 10.8299L8.8 10.8122L8.8019 10.8123C8.80063 10.7903 8.8 10.7682 8.8 10.746C8.8 9.17422 12.0003 7.90002 15.9481 7.90002C19.8959 7.90002 23.0962 9.17422 23.0962 10.746C23.0962 10.7682 23.0955 10.7903 23.0943 10.8123L23.0962 10.8122L23.093 10.8311C23.0872 10.9098 23.0734 10.9876 23.0519 11.0645L22.5749 13.7666L23.3549 14.5467ZM21.2962 12.6344C19.9867 13.2218 18.076 13.592 15.9481 13.592C13.8203 13.592 11.9096 13.2219 10.6001 12.6344L12.0072 20.6077L12.2373 21.8286L12.2586 21.8452C12.3789 21.9354 12.5652 22.0371 12.807 22.1351L12.8561 22.1546C13.6355 22.4594 14.7462 22.6439 15.9481 22.6439C17.1569 22.6439 18.2733 22.4573 19.0528 22.1497C19.3337 22.0388 19.5431 21.9223 19.6661 21.8231L19.6761 21.8148L19.9019 20.5348C19.3338 20.3787 18.7955 20.0812 18.3429 19.6429L18.3004 19.6011L15.3749 16.6756C15.0906 16.3913 15.0906 15.9303 15.3749 15.646C15.6523 15.3686 16.0978 15.3618 16.3834 15.6257L16.4045 15.646L19.33 18.5715C19.5717 18.8132 19.8555 18.9861 20.1569 19.0901L21.2962 12.6344ZM22.2661 15.517L21.6408 19.0597C21.8989 18.9575 22.1402 18.8024 22.3482 18.5944C23.1641 17.7784 23.1664 16.4494 22.355 15.6065L22.3253 15.5763L22.2661 15.517ZM15.9481 9.35612C14.2013 9.35612 12.5813 9.62893 11.4322 10.0864C10.9385 10.283 10.5712 10.4995 10.3598 10.6985C10.3463 10.7112 10.334 10.7232 10.3228 10.7347L10.3122 10.7459L10.3314 10.7661L10.3598 10.7936C10.5712 10.9926 10.9385 11.2091 11.4322 11.4056C12.5813 11.8631 14.2013 12.1359 15.9481 12.1359C17.6949 12.1359 19.3149 11.8631 20.4639 11.4056C20.9577 11.2091 21.325 10.9926 21.5364 10.7936C21.5499 10.7809 21.5622 10.7688 21.5733 10.7574L21.5841 10.7459L21.5647 10.726L21.5364 10.6985C21.325 10.4995 20.9577 10.283 20.4639 10.0864C19.3149 9.62893 17.6949 9.35612 15.9481 9.35612Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    Backups
                    <v-tooltip activator="parent" location="top">
                        This bucket
                    </v-tooltip>
                </v-chip>
            </v-col>

            <v-col cols="12" md="4" class="d-flex justify-end align-top align-md-center">
                <v-btn>
                    Bucket Actions
                    <template #append>
                        <v-icon icon="mdi-chevron-down" />
                    </template>
                    <!--                    <BucketActionsMenu />-->
                </v-btn>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" md="4">
                <v-card title="Bucket" subtitle="Backups" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="success" variant="tonal" class="mr-2 font-weight-bold">
                            Pro
                            <v-tooltip activator="parent" location="top">
                                Pro account since: 2 May 2022
                            </v-tooltip>
                        </v-chip>
                        <template v-if="featureFlags.bucket.updateInfo">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default" class="mr-2">
                                Edit Bucket Information
                                <!--                                <BucketUpdateDialog />-->
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Value" subtitle="Attribution" variant="flat" :border="true" rounded="xlg" class="mb-3">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2">Company</v-chip>
                        <template v-if="featureFlags.bucket.updateValueAttribution">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                <BucketUserAgentsDialog />
                                Set Value Attribution
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="4">
                <v-card title="Placement" subtitle="Region" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" class="mr-2">
                            Global
                        </v-chip>
                        <template v-if="featureFlags.bucket.updatePlacement">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                <BucketGeofenceDialog />
                                Set Bucket Placement
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" md="4">
                <!--                <CardStatsComponent title="Storage" subtitle="Total" data="524 GB" variant="tonal" />-->
            </v-col>
            <v-col cols="12" md="4">
                <!--                <CardStatsComponent title="Download" subtitle="This Month" data="273 GB" variant="tonal" />-->
            </v-col>
            <v-col cols="12" md="4">
                <!--                <CardStatsComponent title="Segments" subtitle="Total" data="241,721" variant="tonal" />-->
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VRow,
    VCol,
    VChip,
    VTooltip,
    VBtn,
    VIcon,
    VCard,
    VCardText,
    VDivider,
} from 'vuetify/components';

import { FeatureFlags } from '@/api/client.gen';
import { useAppStore } from '@/store/app';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import BucketGeofenceDialog from '@/components/BucketGeofenceDialog.vue';
import BucketUserAgentsDialog from '@/components/BucketUserAgentsDialog.vue';

const featureFlags = useAppStore().state.settings.admin.features as FeatureFlags;
</script>
