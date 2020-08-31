// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./headerArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NewProjectArea from '@/components/header/NewProjectArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import LogoIcon from '@/../static/images/header/logo.svg';
import NavigationCloseIcon from '@/../static/images/header/navigationClose.svg';
import NavigationMenuIcon from '@/../static/images/header/navigationMenu.svg';

import AccountButton from './AccountButton.vue';

@Component({
    components: {
        NewProjectArea,
        AccountButton,
        NavigationArea,
        NavigationMenuIcon,
        NavigationCloseIcon,
        LogoIcon,
    },
})
export default class DashboardHeader extends Vue {
    /**
     * Indicates if navigation toggling button is visible depending on screen width.
     */
    public isNavigationVisible: boolean = false;

    /**
     * Toggle navigation visibility.
     */
    public toggleNavigationVisibility(): void {
        this.isNavigationVisible = !this.isNavigationVisible;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }
}
</script>

<style src="./headerArea.scss" scoped lang="scss"></style>
