// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-api-key-container" >
        <div class="delete-api-key-container__wrap">
            <div class="delete-api-key-container__selected-api-keys-count">
                <span class="delete-api-key-container__selected-api-keys-count__button"></span>
                <p class="delete-api-key-container__selected-api-keys-count__count">{{ selectedAPIKeysCount }}</p>
                <p class="delete-api-key-container__selected-api-keys-count__total-count"> of <span>{{ allAPIKeysCount }}</span> API Keys Selected</p>
            </div>
            <div class="delete-api-key-container__buttons-group">
                <Button 
                    class="delete-api-key-container__buttons-group__cancel" 
                    label="Cancel" 
                    width="140px" 
                    height="48px"
                    :onPress="onClearSelection"
                    isWhite />
                <Button 
                    label="Delete" 
                    width="140px" 
                    height="48px"
                    :onPress="onDelete" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import { API_KEYS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    methods: {
        onDelete: async function () {
            let selectedKeys: any[] = this.$store.getters.selectedAPIKeys.map((key) => {return key.id; });

            const dispatchResult = await this.$store.dispatch(API_KEYS_ACTIONS.DELETE, selectedKeys);

            let keySuffix = selectedKeys.length > 1 ? '\'s' : '';

            if (dispatchResult.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, `API key${keySuffix} deleted successfully`);
            } else {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Error during deletion API key${keySuffix}`);
            }
        },
        onClearSelection: function (): void {
            this.$store.dispatch(API_KEYS_ACTIONS.CLEAR_SELECTION);
        },
    },
    computed: {
        selectedAPIKeysCount: function (): number {
            return this.$store.getters.selectedAPIKeys.length;
        },
        allAPIKeysCount: function (): number {
            return this.$store.state.apiKeysModule.apiKeys.length;
        }
    },
    components: {
        Button
    }
})

export default class DeleteApiKeysArea extends Vue {
}
</script>

<style scoped lang="scss">
    .delete-api-key-container {
        padding-bottom: 50px;
        position: fixed;
        bottom: 0px;
        max-width: 79.7%;
        width: 100%;

        &__wrap {
            padding: 0 32px;
            height: 98px;
            background-color: #fff;
            display: flex;
            align-items: center;
            justify-content: space-between;
            box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
            border-radius: 6px;
        }

        &__buttons-group {
            display: flex;
            
            span {
                width: 142px;
                padding: 14px 0;
                display: flex;
                align-items: center;
                justify-content: center;
                border-radius: 6px;
                font-family: 'font_medium';
                font-size: 16px;
                cursor: pointer;
            }

            &__cancel {
                margin-right: 24px;
            }
        }

        &__selected-api-keys-count {
            display: flex;
            align-items: center;
            font-family: 'font_regular';
            font-size: 18px;
            color: #AFB7C1;

            &__count {
                margin: 0 7px;
            }

            &__button {
                height: 16px;
                display: block;
                cursor: pointer;
                width: 16px;
                background-image: url('../../../../static/images/team/delete.svg');
            }
        }
    }
    @media screen and (max-width: 1600px) {
        .delete-api-key-container {
            max-width: 74%;
        }
    }

    @media screen and (max-width: 1366px) {
        .delete-api-key-container {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1120px) {
        .delete-api-key-container {
            max-width: 65%;
        }
    }

    @media screen and (max-width: 1025px) {
        .delete-api-key-container {
            max-width: 84%;
        }
    }
</style>