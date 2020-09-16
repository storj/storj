// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BlueHideIcon from '@/../static/images/common/BlueMinus.svg';
import BlueExpandIcon from '@/../static/images/common/BluePlus.svg';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';

@Component({
    components: {
        BlueExpandIcon,
        BlueHideIcon,
    },
})
export default class BaseSmallHeldHistoryTable extends Vue {
    /**
     * Indicates if held info should be rendered.
     */
    public get allSatellitesHeldHistory(): SatelliteHeldHistory[] {
        return this.$store.state.payoutModule.heldHistory;
    }

    /**
     * Indicates if held info should be rendered.
     */
    public isExpanded: boolean = false;

    /**
     * Shows held info.
     */
    public expand(): void {
        this.isExpanded = true;
    }

    /**
     * Hides held info.
     */
    public hide(): void {
        this.isExpanded = false;
    }
}
</script>

<style scoped lang="scss">
    .held-history-table-container--small__item {
        padding: 12px;
        width: calc(100% - 24px);

        &__satellite-info {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__name {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
                max-width: calc(100% - 40px);
                word-break: break-word;
            }

            &__months {
                font-family: 'font_regular', sans-serif;
                font-size: 11px;
                color: #9b9db1;
                margin-top: 3px;
            }

            &__button {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 30px;
                height: 30px;
                min-width: 30px;
                min-height: 30px;
                background: var(--expand-button-background-color);
                border-radius: 3px;
                cursor: pointer;
            }
        }

        &__held-info {
            margin-top: 16px;

            &__item {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 12px;
                line-height: 12px;
                margin-bottom: 10px;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    color: #909bad;
                }

                &__value {
                    font-family: 'font_regular', sans-serif;
                    color: var(--regular-text-color);
                }
            }
        }
    }

    .icon {
        display: flex;
        align-items: center;
        justify-content: center;
        max-width: 100%;
        max-height: 100%;
        width: 100%;
        height: 100%;
    }

    .fade-enter-active,
    .fade-leave-active {
        transition: opacity 0.5s;
    }

    .fade-enter,
    .fade-leave-to /* .fade-leave-active below version 2.1.8 */ {
        opacity: 0;
    }
</style>
