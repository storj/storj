// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content-holder">
            <div class="header__content-holder__logo-area">
                <StorjIcon
                    class="header__content-holder__icon"
                    alt="storj icon"
                />
                <div class="header__content-holder__logo-area__refresh-button" @click="onRefresh">
                    <RefreshIcon alt="refresh image"/>
                </div>
            </div>
            <div class="header__content-holder__node-id-container">
                <b class="header__content-holder__node-id-container__title">Node ID:</b>
                <p class="header__content-holder__node-id-container__id">{{this.nodeId}}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import RefreshIcon from '@/../static/images/refresh.svg';
import StorjIcon from '@/../static/images/storjIcon.svg';

import { NODE_ACTIONS } from '@/app/store/modules/node';

const {
    GET_NODE_INFO,
    SELECT_SATELLITE,
} = NODE_ACTIONS;

@Component({
    components: {
        StorjIcon,
        RefreshIcon,
    },
})
export default class SNOHeader extends Vue {
    public async onRefresh(): Promise<void> {
        const selectedSatellite = this.$store.state.node.selectedSatellite.id;

        try {
            await this.$store.dispatch(GET_NODE_INFO);
            await this.$store.dispatch(SELECT_SATELLITE, selectedSatellite);
        } catch (error) {
            console.error(`${error.message} satellite data.`);
        }
    }

    public get nodeId(): string {
        return this.$store.state.node.info.id;
    }
}
</script>

<style scoped lang="scss">
    .header {
        width: 100%;
        height: 89px;
        display: flex;
        justify-content: center;
        background-color: #fff;

        &__content-holder {
            width: 822px;
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__logo-area {
                display: flex;
                align-items: center;

                &__refresh-button {
                    margin-left: 25px;
                    max-height: 42px;
                    cursor: pointer;

                    &:hover {

                        .refresh-button-svg-rect {
                            fill: #133e9c;
                        }

                        .refresh-button-svg-path {
                            fill: #fff;
                        }
                    }
                }
            }

            &__node-id-container {
                color: #535f77;
                height: 44px;
                padding: 0 14px 0 14px;
                display: flex;
                align-items: center;
                border: 1px solid #e8e8e8;
                border-radius: 12px;
                font-size: 14px;

                &__title {
                    min-width: 55px;
                    margin-right: 5px;
                }

                &__id {
                    font-size: 11px;
                }
            }
        }
    }
</style>
