<template>
    <div class="dialog">
        <p class="dialog__make-default">Make Default</p>
        <p class="dialog__delete">Delete</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class CardDialog extends Vue {}
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

        p {
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

        &__make-default {
            color: #61666B;
        }

        &__delete {
            color: #EB5757;
        }
    }
</style>