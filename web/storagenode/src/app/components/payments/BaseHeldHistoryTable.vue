// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class BaseHeldHistoryTable extends Vue {}
</script>

<style scoped lang="scss">
    .held-history-table-container--large {

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: var(--table-header-color);

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 11px 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            min-height: 34px;
            height: auto;
            border-bottom: 1px solid rgb(169 181 193 / 30%);

            &:last-of-type {
                border-bottom: none;
            }

            &__text {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
                max-width: 100%;
                word-break: break-word;
            }

            &__months {
                font-family: 'font_regular', sans-serif;
                font-size: 11px;
                color: #9b9db1;
                margin-top: 3px;
            }
        }
    }

    .held-history-table-container--small {
        display: none;
    }

    .column {
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .justify-start {
        justify-content: flex-start;
    }

    .justify-end {
        justify-content: flex-end;
    }

    .column-1 {
        width: 37%;
    }

    .column-2,
    .column-3,
    .column-4 {
        width: 21%;
    }

    @media screen and (max-width: 720px) {

        .column-1 {
            width: 31%;
        }

        .column-2,
        .column-3,
        .column-4 {
            width: 23%;
        }
    }

    @media screen and (max-width: 600px) {

        .held-history-table-container--large {
            display: none;
        }

        .held-history-table-container--small {
            display: block;
        }
    }
</style>
