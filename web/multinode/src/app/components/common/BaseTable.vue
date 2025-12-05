// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table class="base-table" border="0" cellpadding="0" cellspacing="0">
        <slot name="head" />
        <slot name="body" />
    </table>
</template>

<script setup lang="ts"></script>

<style lang="scss">
    .base-table {
        width: 100%;
        border: 1px solid var(--v-border-base);
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
            background: var(--v-active-base);

            tr {
                height: 40px;
                font-size: 12px;
                color: var(--v-text-base);
                border-radius: var(--br-table);
                text-align: right;
            }
        }
    }

    .table-item {
        height: 56px;
        text-align: right;
        font-size: 16px;
        color: var(--v-header-base);

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
            background: var(--v-active-base);
        }

        th:not(:first-of-type) {
            font-family: 'font_medium', sans-serif;
        }
    }

    .align-left {
        text-align: left;
    }

    .Online {
        color: var(--v-success-base);
    }

    .Offline {
        color: var(--v-error-base);
    }

    [class='Not Reachable'] {
        color: var(--v-warning-base);
    }

    .Unauthorized {
        color: var(--v-warning-base);
    }

    [class='Internal Error'] {
        color: var(--v-error-base);
    }

    .overflow-visible {
        overflow: visible !important;
    }

    .options {
        width: 60px;
        text-align: center;
    }
</style>
