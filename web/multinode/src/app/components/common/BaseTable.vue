// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table class="base-table" border="0" cellpadding="0" cellspacing="0">
        <slot name="head" />
        <slot name="body" />
    </table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class BaseTable extends Vue {}
</script>

<style lang="scss">
    .base-table {
        width: 100%;
        border: 1px solid var(--c-gray--light);
        border-radius: var(--br-table);
        font-family: 'font_semiBold', sans-serif;
        z-index: 997;

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
        }

        thead {
            background: var(--c-block-gray);

            tr {
                height: 40px;
                font-size: 12px;
                color: var(--c-gray);
                border-radius: var(--br-table);
                text-align: right;
            }
        }
    }

    .table-item {
        height: 56px;
        text-align: right;
        font-size: 16px;
        color: var(--c-line);

        th {
            box-sizing: border-box;
            padding: 0 20px;
            max-width: 250px;
            white-space: nowrap;
            text-overflow: ellipsis;
            position: relative;
            overflow: hidden;
        }

        &:nth-of-type(even) {
            background: var(--c-block-gray);
        }

        th:not(:first-of-type) {
            font-family: 'font_medium', sans-serif;
        }
    }

    .align-left {
        text-align: left;
    }

    .online {
        color: var(--c-success);
    }

    .offline {
        color: var(--c-error);
    }

    .overflow-visible {
        overflow: visible !important;
    }

    .options {
        width: 60px;
        text-align: center;
    }
</style>
