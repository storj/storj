// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-item">
            <p class="sort-header-container__name-item__title">NAME</p>
        </div>
        <div class="sort-header-container__users-item">
            <p class="sort-header-container__users-item__title"># USERS</p>
        </div>
        <div class="sort-header-container__date-item">
            <p class="sort-header-container__date-item__title">DATE ADDED</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class SortProjectsListHeader extends Vue {}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        width: calc(100% - 32px);
        height: 40px;
        background-color: #fff;
        margin-top: 31px;
        padding: 16px 16px 0;
        border-radius: 8px 8px 0 0;

        &__name-item,
        &__data-item,
        &__users-item,
        &__date-item {
            width: 33%;
            display: flex;
            align-items: center;
            margin: 0;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin: 0 0 0 23px;
                color: #2a2a32;
            }

            .creation-date {
                margin-left: 0;
            }
        }

        &__date-item {

            &__title {
                margin: 0;
            }
        }

        &__name-item {

            &__title {
                margin: 0 0 0 40px;
            }
        }
    }
</style>
