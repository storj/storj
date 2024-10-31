// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-area">
        <nav class="dashboard-area__navigation-area">
            <navigation-area />
        </nav>
        <div class="dashboard-area__right-area">
            <header class="dashboard-area__right-area__header">
                <add-new-node />
            </header>
            <div class="dashboard-area__right-area__content">
                <router-view />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { UnauthorizedError } from '@/api';
import { Notify } from '@/app/plugins';

import AddNewNode from '@/app/components/modals/AddNewNode.vue';
import NavigationArea from '@/app/components/navigation/NavigationArea.vue';

// @vue/component
@Component({
    components: {
        AddNewNode,
        NavigationArea,
    },
})
export default class Dashboard extends Vue {

    public notify = new Notify();

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('nodes/trustedSatellites');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }
            this.notify.error({ message: error.message, title: error.name });

        }
    }
}
</script>

<style lang="scss" scoped>
    .dashboard-area {
        display: flex;

        &__navigation-area {
            width: 280px;
        }

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
                border: 1px solid var(--c-gray--light);
                background: var(--c-block-gray);
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
