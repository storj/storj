// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <!-- <v-breadcrumbs :items="['My Account', 'Settings']" class="pl-0"></v-breadcrumbs> -->

        <h1 class="text-h5 font-weight-bold mb-2">Settings</h1>

        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three">
                <v-list-subheader class="mb-2">Profile</v-list-subheader>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Name</v-list-item-title>

                    <v-list-item-subtitle>
                        Tome Boshevski
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small">Edit Name</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Email</v-list-item-title>

                    <v-list-item-subtitle>
                        tome@storj.io
                    </v-list-item-subtitle>

                    <!-- <template v-slot:append>
                         <v-list-item-action>
                            <v-btn>Change Email</v-btn>
                        </v-list-item-action>
                    </template> -->
                </v-list-item>
            </v-list>
        </v-card>
        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three">
                <v-list-subheader class="mb-2">Security</v-list-subheader>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Password</v-list-item-title>

                    <v-list-item-subtitle>
                        **********
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small">Change Password</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Two-factor authentication</v-list-item-title>

                    <v-list-item-subtitle>
                        Improve account security by enabling 2FA.
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn size="small">Enable Two-factor</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Session Timeout</v-list-item-title>

                    <v-list-item-subtitle>
                        Set timeout to log you out for inactivity.
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small">Set Timeout</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>
            </v-list>
        </v-card>

        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three" select-strategy="classic">
                <v-list-subheader class="mb-2">Notifications</v-list-subheader>

                <v-divider />

                <v-list-item value="notifications" color="default">
                    <template #append="{ isActive }">
                        <v-list-item-action start>
                            <v-checkbox-btn :model-value="isActive" />
                        </v-list-item-action>
                    </template>

                    <v-list-item-title>Product newsletter</v-list-item-title>

                    <v-list-item-subtitle>
                        Notify me about product updates.
                    </v-list-item-subtitle>
                </v-list-item>
            </v-list>
        </v-card>
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VCard,
    VList,
    VListSubheader,
    VDivider,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
    VListItemAction,
    VBtn,
    VCheckboxBtn,
} from 'vuetify/components';
</script>
