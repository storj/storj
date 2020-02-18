// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="held-progress">
        <div class="held-progress__steps-area">
            <div
                v-for="step in steps"
                :key="step.amount"
                :class="step.className"
            >
                <h1 class="amount">{{ step.amount }}</h1>
                <div class="divider"></div>
                <h4 class="label">{{ step.label }}</h4>
            </div>
        </div>
        <div class="held-progress__border"></div>
        <p class="held-progress__main-text">It is your <span class="bold">{{ '7' }} month</span> on network</p>
        <p class="held-progress__hint">25% of Storage Node revenue is withheld, 75% is paid to the Storage Node Operator</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

class HeldStep {
    public constructor(
        public amount: string = '0%',
        public label: string = '',
        public active: boolean = false,
        public inFuture: boolean = true,
    ) {}

    public get className(): string {
        if (this.active) return 'held-progress__steps-area__step--active';
        if (this.inFuture) return 'held-progress__steps-area__step--future';

        return 'held-progress__steps-area__step';
    }
}

@Component
export default class HeldProgress extends Vue {
    public steps: HeldStep[] = [
        new HeldStep('75%', 'Month 1-3', false, false),
        new HeldStep('50%', 'Month 4-6', false, false),
        new HeldStep('25%', 'Month 7-9', true, false),
        new HeldStep('0%', 'Month 10-15', false, true),
        new HeldStep('+50%', 'Month 15', false, true),
    ];
}
</script>

<style scoped lang="scss">
    p,
    h1,
    h4 {
        margin: 0;
    }

    @mixin step($active: false, $future: false) {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;

        .amount {
            font-family: 'font_bold', sans-serif;
            font-size: 20px;
            color: #224ca5;
            opacity: 0.4;

            @if $active {
                opacity: 1;
            }

            @if $future {
                color: #909bad;
            }
        }

        .divider {
            width: 100%;
            height: 4px;
            background: #224ca5;
            mix-blend-mode: normal;
            opacity: 0.4;
            border-radius: 4px;
            margin: 16px 0 8px 0;

            @if $active {
                opacity: 1;
            }

            @if $future {
                background: #909bad;
            }
        }

        .label {
            font-size: 14px;
            color: #586c86;
        }
    }

    .held-progress {
        display: flex;
        flex-direction: column;
        width: 100%;
        background: #fff;
        border: 1px solid #eaeaea;
        box-sizing: border-box;
        border-radius: 12px;
        padding: 29px;
        font-family: 'font_regular', sans-serif;

        &__steps-area {
            display: grid;
            grid-template-columns: repeat(5, 1fr);
            grid-column-gap: 2px;

            &__step {
                @include step();

                &--active {
                    @include step(true);
                }

                &--future {
                    @include step($future: true);
                }
            }
        }

        &__border {
            width: 100%;
            height: 1px;
            background: #a9b5c1;
            opacity: 0.3;
            margin: 24px 0 20px 0;
        }

        &__main-text {
            font-size: 16px;
            color: #535f77;

            .bold {
                font-family: 'font_bold', sans-serif;
            }
        }

        &__hint {
            font-size: 13px;
            line-height: 15px;
            color: #9b9db1;
        }
    }
</style>
