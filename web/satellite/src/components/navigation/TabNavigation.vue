// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tab-navigation-container">
        <router-link
            :to="navLink.path"
            class="tab-navigation-container__item"
            v-for="navLink in navigation"
            :key="navLink.name">
            <p class="tab-navigation-container__item__name">{{navLink.name}}</p>
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { NavigationLink } from '@/types/navigation';

@Component
export default class TabNavigation extends Vue {
    @Prop({default: new Array(NavigationLink)})
    private navigation: NavigationLink[];
}
</script>

<style scoped lang="scss">
    .tab-navigation-container {
        width: auto;
        display: flex;
        border-bottom: 1px solid #afb7c1;

        &__item {
            width: 150px;
            height: 48px;
            display: flex;
            align-items: center;
            justify-content: center;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #afb7c1;
            }

            &.router-link-exact-active,
            &:hover {
                border-bottom: 3px solid #2683ff;

                .tab-navigation-container__item__name {
                    color: #354049;
                }
            }
        }
    }
</style>
