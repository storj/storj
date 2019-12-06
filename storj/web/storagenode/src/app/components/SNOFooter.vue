// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="footer">
        <div class="footer__content-holder">
            <StorjIcon
                class="footer__content-holder__icon"
                alt="storj icon"
                @click="scrollUp"
            />
            <a class="footer__content-holder__community-link" href="https://forum.storj.io/c/sno-category" target="_blank" rel="noopener">Community</a>
            <a class="footer__content-holder__support-link" href="https://support.storj.io" target="_blank" rel="noopener">Support</a>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import StorjIcon from '@/../static/images/storjIcon.svg';

@Component({
    components: {
        StorjIcon,
    },
})
export default class SNOFooter extends Vue {
    public scrollUp(): void {
        window.scrollTo(0, 0);
    }
}
</script>

<style scoped lang="scss">
    .footer {
        width: 100%;
        height: 89px;
        display: flex;
        justify-content: center;
        background-color: #f8f9fb;
        align-items: center;
        user-select: none;

        &__content-holder {
            width: 822px;
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__icon {
                min-width: 125px;
                cursor: pointer;
            }

            &__community-link,
            &__support-link {
                font-size: 14px;
                text-decoration: none;
                color: #224ca5;
            }

            &__community-link {
                margin-left: 540px;
            }

            &__support-link {
                margin-left: 44px;
            }
        }
    }
</style>
