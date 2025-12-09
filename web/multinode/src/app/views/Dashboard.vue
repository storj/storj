// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <nav class="dashboard-area__navigation-area">
            <navigation-area />
        </nav>
        <div class="dashboard-area__right-area">
            <header class="dashboard-area__right-area__header">
                <theme-selector />
                <add-new-node />
            </header>
            <div class="dashboard-area__right-area__content">
                <router-view />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import ThemeSelector from '../components/common/ThemeSelector.vue';

import { UnauthorizedError } from '@/api';
import { useNodesStore } from '@/app/store/nodesStore';

import AddNewNode from '@/app/components/modals/AddNewNode.vue';
import NavigationArea from '@/app/components/navigation/NavigationArea.vue';

const nodesStore = useNodesStore();

onMounted(async () => {
    try {
        await nodesStore.trustedSatellites();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }
        // TODO: notify error
    }
});
</script>

<style lang="scss" scoped>
    .dashboard-area {
        display: flex;

        &__right-area {
            position: relative;
            flex: 1;

            &__header {
                width: 100%;
                height: 80px;
                padding: 0 60px;
                box-sizing: border-box;
                display: flex;
                align-items: center;
                justify-content: flex-end;
                border: 1px solid var(--v-border-base);
                background: var(--v-background-base);
            }

            &__content {
                position: absolute;
                box-sizing: border-box;
                height: calc(100vh - 80px);
                top: 80px;
                left: 0;
                width: 100%;
            }
        }
    }
</style>
