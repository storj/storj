// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper title="Your account">
        <template #content>
            <div class="info-step">
                <div class="info-step__column">
                    <h2 class="info-step__column__title">Free</h2>
                    <VButton
                        label="Current"
                        font-size="14px"
                        border-radius="10px"
                        width="280px"
                        height="48px"
                        :is-disabled="true"
                        :on-press="() => {}"
                    />
                    <div class="info-step__column__bullets">
                        <InfoBullet class="info-step__column__bullets__item" title="Projects" info="1 project" />
                        <InfoBullet class="info-step__column__bullets__item" title="Storage" info="25 GB limit" />
                        <InfoBullet class="info-step__column__bullets__item" title="Download" info="25 GB limit" />
                        <InfoBullet class="info-step__column__bullets__item" title="Segments" info="10,000 segments limit" />
                        <InfoBullet class="info-step__column__bullets__item" title="Link Sharing" info="Link sharing with Storj domain" />
                    </div>
                </div>
                <div class="info-step__column">
                    <h2 class="info-step__column__title">Pro Account</h2>
                    <VButton
                        label="Upgrade to Pro"
                        font-size="14px"
                        border-radius="10px"
                        width="280px"
                        height="48px"
                        :is-green="true"
                        :on-press="onUpgrade"
                    />
                    <div class="info-step__column__bullets">
                        <InfoBullet class="info-step__column__bullets__item" is-pro title="Projects" info="3 projects + more on request" />
                        <InfoBullet class="info-step__column__bullets__item" is-pro :title="storagePrice" info="25 GB free included" />
                        <InfoBullet class="info-step__column__bullets__item" is-pro title="Download $0.007 GB" :info="downloadInfo">
                            <template v-if="downloadMoreInfo" #moreInfo>
                                <p class="info-step__column__bullets__message">{{ downloadMoreInfo }}</p>
                            </template>
                        </InfoBullet>
                        <InfoBullet class="info-step__column__bullets__item" is-pro title="Segments" info="$0.0000088 segment per month">
                            <template #moreInfo>
                                <a
                                    class="info-step__column__bullets__link"
                                    href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing/billing-and-payment"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    Learn more about segments
                                </a>
                            </template>
                        </InfoBullet>
                        <InfoBullet class="info-step__column__bullets__item" is-pro title="Secure Custom Domains (HTTPS)" info="Link sharing with your domain" />
                    </div>
                </div>
            </div>
        </template>
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';

import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import VButton from '@/components/common/VButton.vue';
import InfoBullet from '@/components/modals/upgradeAccountFlow/InfoBullet.vue';

const usersStore = useUsersStore();
const notify = useNotify();

const props = defineProps<{
    onUpgrade: () => void;
}>();

const storagePrice = ref<string>('Storage $0.004 GB / month');
const downloadInfo = ref<string>('25 GB free every month');
const downloadMoreInfo = ref<string>('');

/**
 * Lifecycle hook before initial render.
 * If applicable, loads additional clarifying text based on user partner.
 */
onBeforeMount(() => {
    try {
        const partner = usersStore.state.user.partner;
        const config = require('@/components/modals/upgradeAccountFlow/upgradeConfig.json');
        if (partner && config[partner]) {
            if (config[partner].storagePrice) {
                storagePrice.value = config[partner].storagePrice;
            }

            if (config[partner].downloadInfo) {
                downloadInfo.value = config[partner].downloadInfo;
            }

            if (config[partner].downloadMoreInfo) {
                downloadMoreInfo.value = config[partner].downloadMoreInfo;
            }
        }
    } catch (e) {
        notify.error('No configuration file for page.', null);
    }
});
</script>

<style scoped lang="scss">
.info-step {
    display: flex;
    align-items: center;
    column-gap: 16px;
    font-family: 'font_regular', sans-serif;

    &__column {

        &:first-of-type {
            @media screen and (width <= 690px) {
                display: none;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: var(--c-black);
            text-align: left;
            margin-bottom: 16px;
        }

        &__bullets {
            padding: 16px 0 16px 16px;
            border: 1px solid var(--c-grey-2);
            border-radius: 8px;
            margin-top: 16px;
            width: 280px;
            box-sizing: border-box;

            &__item:not(:first-of-type) {
                margin-top: 24px;
            }

            &__message {
                font-weight: 500;
                font-size: 12px;
                line-height: 18px;
                color: var(--c-white);
            }

            &__link {
                font-weight: 500;
                font-size: 12px;
                line-height: 18px;
                text-decoration: underline !important;
                text-underline-position: under;
                color: var(--c-white);
            }
        }
    }
}
</style>
