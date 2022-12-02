// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="closeDropdown" class="history-dropdown">
        <div class="history-dropdown__link-container" @click="redirect">
            <span class="history-dropdown__link-container__link">{{ label }}</span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';

// @vue/component
@Component
export default class HistoryDropdown extends Vue {
    @Prop({ default: '' })
    public readonly label: string;
    @Prop({ default: '' })
    public readonly route: string;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Holds logic to redirect user to history page.
     */
    public redirect(): void {
        this.analytics.pageVisit(this.route);
        this.$router.push(this.route);
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$emit('close');
    }
}
</script>

<style scoped lang="scss">
    .history-dropdown {
        z-index: 120;
        position: absolute;
        left: 0;
        top: 35px;
        background-color: #fff;
        border-radius: 6px;
        border: 1px solid #c5cbdb;
        box-shadow: 0 8px 34px rgb(161 173 185 / 41%);
        width: 210px;

        &__link-container {
            width: calc(100% - 30px);
            height: 50px;
            padding: 0 15px;
            display: flex;
            align-items: center;
            border-radius: 6px;

            &:hover {
                background-color: #f5f5f7;
            }

            &__link {
                font-size: 14px;
                line-height: 19px;
                color: #7e8b9c;
            }
        }
    }
</style>
