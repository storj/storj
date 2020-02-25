// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog" v-click-outside="closeCardsDialog">
        <p class="label dialog__make-default" @click="onMakeDefaultClick">Make Default</p>
        <p class="label dialog__delete" @click="onRemoveClick">Delete</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

const {
    CLEAR_CARDS_SELECTION,
    MAKE_CARD_DEFAULT,
    REMOVE_CARD,
} = PAYMENTS_ACTIONS;

@Component
export default class CardDialog extends Vue {
    @Prop({default: ''})
    private readonly cardId: string;

    /**
     * Closes card selection dialog.
     */
    public closeCardsDialog(): void {
        this.$store.dispatch(CLEAR_CARDS_SELECTION);
    }

    /**
     * Selects card as default.
     */
    public async onMakeDefaultClick(): Promise<void> {
        try {
            await this.$store.dispatch(MAKE_CARD_DEFAULT, this.cardId);
        } catch (error) {
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
        }
    }

    /**
     * Removes card from list.
     */
    public async onRemoveClick(): Promise<void> {
        try {
            await this.$store.dispatch(REMOVE_CARD, this.cardId);
        } catch (error) {
            await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
        }
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
        background-image: url('../../../../../static/images/payments/Dialog.png');
        background-size: contain;
        width: 167px;
        height: 122px;
        cursor: initial;

        &__make-default {
            color: #61666b;
        }

        &__delete {
            color: #eb5757;
        }
    }

    .label {
        font-family: 'font_medium', sans-serif;
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
