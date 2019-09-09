// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-balance-area">
        <h1 class="account-balance-area__title">Account Balance</h1>
        <div class="account-balance-area__balance-area">
            <span class="account-balance-area__balance-area__balance">Balance <b>$25.00</b></span>
            <Button
                class="button"
                label="Earn Credits"
                width="153px"
                height="48px"
                :onPress="onEarnCredits"/>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import Button from '@/components/common/Button.vue';

@Component({
    components: {
        Button
    }
})
export default class AccountBalance extends Vue {
    public onEarnCredits(): void {
        return;
    }
}
</script>

<style scoped lang="scss">
    h1,
    span {
        margin: 0;
        color: #354049;
    }

    .button {

        &:hover {
            background-color: #0059D0;
            box-shadow: none;
        }
    }

    .account-balance-area {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 40px;
        margin-bottom: 47px;
        background-color: #FFFFFF;
        border-radius: 8px;
        font-family: 'font_regular';

        &__title {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 48px;
        }

        &__balance-area {
            display: flex;
            align-items: center;

            &__balance {
                margin-right: 27px;
                font-size: 18px;
                color: rgba(53, 64, 73, 0.5);

                b {
                    color: #354049;
                }
            }
        }
    }
</style>
