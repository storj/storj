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
                'newProjectPopup',
                'newProjectButton',
                'accountDropdown',
                'accountDropdownButton',
                'projectDropdown',
                'projectDropdownButton',
                'deleteProjectPopup',
                'deleteProjectPopupButton',
                'deleteAccountPopupButton',
                'deleteAccountPopup',
                'addTeamMemberPopupButton',
                'addTeamMemberPopup',
                'addTeamMemberPopupButtonSVG',
                'addApiKeyPopup',
                'addApiKeyPopupButton',
                'addApiKeysPopupEmptyButton',
                'addTeamMemberPopupButtonSVG',
                'sortTeamMemberByDropdown',
				'sortTeamMemberByDropdownButton',
                'createAccountButton',
                'successfulRegistrationPopup',
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

    @font-face {
        font-family: "montserrat_regular";
        src: url("../static/fonts/montserrat_regular.ttf");
    }

    @font-face {
        font-family: "montserrat_medium";
        src: url("../static/fonts/montserrat_medium.ttf");
    }

    @font-face {
        font-family: "montserrat_bold";
        src: url("../static/fonts/montserrat_bold.ttf");
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
</style>
