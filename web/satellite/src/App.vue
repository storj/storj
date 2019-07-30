// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app" v-on:click="onClick">
        <router-view/>
        <!-- Area for displaying notification -->
        <NotificationArea/>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import NotificationArea from '@/components/notifications/NotificationArea.vue';
    import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        data: function() {
            return {
                ids: [
                    'accountDropdown',
                    'accountDropdownButton',
                    'projectDropdown',
                    'projectDropdownButton',
                    'sortTeamMemberByDropdown',
                    'sortTeamMemberByDropdownButton',
                    'notificationArea',
                    'successfulRegistrationPopup',
                    'deletePaymentMethodButton',
                    'deletePaymentMethodDialog',
                    'makeDefaultPaymentMethodButton',
                    'makeDefaultPaymentDialog'
                ]
            };
        },
        components: {
            NotificationArea
        },
        methods: {
            onClick: function(e) {
                let target: any = e.target;
                while (target) {
                    if (this.$data.ids.includes(target.id)) {
                        return;
                    }
                    target = target.parentNode;
                }

                this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
            }
        }
    })

    export default class App extends Vue {
    }
</script>

<style lang="scss">
    body {
        margin: 0px !important;
        height: 100vh;
        zoom: 100%;
    }

    @font-face {
        font-family: "font_regular";
        src: url("../static/fonts/font_regular.ttf");
    }

    @font-face {
        font-family: "font_medium";
        src: url("../static/fonts/font_medium.ttf");
    }

    @font-face {
        font-family: "font_bold";
        src: url("../static/fonts/font_bold.ttf");
    }

    a {
        cursor: pointer;
    }

    input,
    textarea {
        font-family: inherit;
        font-weight: 600;
        border: 1px solid rgba(56, 75, 101, 0.4);
        color: #354049;
        caret-color: #2683FF;
    }

    /* width */
    ::-webkit-scrollbar {
        width: 4px;
    }

    /* Track */
    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff;
    }

    /* Handle */
    ::-webkit-scrollbar-thumb {
        background: #AFB7C1;
        border-radius: 6px;
        height: 5px;
    }
</style>
