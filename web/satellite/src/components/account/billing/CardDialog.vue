// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog" v-click-outside="closeCardsDialog">
        <p class="label dialog__make-default" @click="makeDefault">Make Default</p>
        <p class="label dialog__delete">Delete</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

const {
    CLEAR_CARDS_SELECTION,
    MAKE_CARD_DEFAULT,
} = PAYMENTS_ACTIONS;

Vue.directive('click-outside', {
    bind: function (el, binding, vnode) {
        (el as any).clickOutsideEvent = function (event) {
            if (el === event.target) {
               return;
            }

            if (vnode.context) {
                vnode.context[binding.expression](event);
            }
        };
        document.body.addEventListener('click', (el as any).clickOutsideEvent);
    },
    unbind: function (el) {
        document.body.removeEventListener('click', (el as any).clickOutsideEvent);
    },
});

@Component
export default class CardDialog extends Vue {
    @Prop({default: ''})
    private readonly cardId: string;

    public closeCardsDialog(): void {
        this.$store.dispatch(CLEAR_CARDS_SELECTION);
    }

    public makeDefault() {
        this.$store.dispatch(MAKE_CARD_DEFAULT, this.cardId);
    }
}
</script>

<style scoped lang="scss">
    .dialog {
        position: absolute;
        top: 22px;
        right: -30px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: flex-end;
        z-index: 100;
        background-image: url("../../../../static/images/payments/Dialog.svg");
        background-size: contain;
        width: 167px;
        height: 122px;
        cursor: initial;

        &__make-default {
            color: #61666B;
        }

        &__delete {
            color: #EB5757;
        }
    }

    .label {
        font-family: 'font_medium';
        font-size: 16px;
        margin: 0;
        height: 35%;
        text-align: center;
        cursor: pointer;

        &:hover {
            text-decoration: underline;
        }
    }
</style>
