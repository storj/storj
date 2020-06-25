// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="credits-dropdown" v-click-outside="closeDropdown">
        <div @click="redirect" class="credits-dropdown__link-container">
            <span class="credits-dropdown__link-container__link">Credits History</span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

@Component
export default class CreditsDropdown extends Vue {
    /**
     * Holds logic to redirect user to credit history page.
     */
    public redirect(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.CreditsHistory).path);
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
    .credits-dropdown {
        z-index: 120;
        position: absolute;
        left: 0;
        top: 35px;
        background-color: #fff;
        border-radius: 6px;
        border: 1px solid #c5cbdb;
        box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
        width: 220px;

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
