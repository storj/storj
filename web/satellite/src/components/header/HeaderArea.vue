// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__left-area">
            <div class="header-container__left-area__logo-area">
                <NavigationMenuIcon
                    class="header-container__left-area__logo-area__menu-button"
                    v-if="!isNavigationVisible && !isOnboardingTour"
                    @click.stop="toggleNavigationVisibility"
                />
                <div class="header-container__left-area__logo-area__close-button" v-if="isNavigationVisible" @click.stop="toggleNavigationVisibility">
                    <NavigationCloseIcon/>
                    <p class="header-container__left-area__logo-area__close-button__title">Close</p>
                </div>
                <LogoIcon
                    class="logo"
                    @click.stop="onLogoClick"
                />
            </div>
            <ProjectSelection class="project-selection"/>
            <ResourcesSelection class="resources-selection"/>
            <SettingsSelection class="settings-selection"/>
        </div>
        <div class="header-container__right-area">
            <div class="header-container__right-area__satellite-area">
                <div class="header-container__right-area__satellite-area__checkmark">
                    <CheckmarkIcon/>
                </div>
                <p class="header-container__right-area__satellite-area__name">{{ satelliteName }}</p>
            </div>
            <AccountButton class="header-container__right-area__account-button"/>
        </div>
        <NavigationArea
            class="adapted-navigation"
            v-if="isNavigationVisible"
        />
        <div class="navigation-blur" v-if="isNavigationVisible" @click.self.stop="toggleNavigationVisibility"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelection from '@/components/header/projectsDropdown/ProjectSelection.vue';
import ResourcesSelection from '@/components/header/resourcesDropdown/ResourcesSelection.vue';
import SettingsSelection from '@/components/header/settingsDropdown/SettingsSelection.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import LogoIcon from '@/../static/images/dcs-logo.svg';
import CheckmarkIcon from '@/../static/images/header/checkmark.svg';
import NavigationCloseIcon from '@/../static/images/header/navigationClose.svg';
import NavigationMenuIcon from '@/../static/images/header/navigationMenu.svg';

import { RouteConfig } from '@/router';

import AccountButton from './accountDropdown/AccountButton.vue';

@Component({
    components: {
        AccountButton,
        NavigationArea,
        NavigationMenuIcon,
        NavigationCloseIcon,
        LogoIcon,
        CheckmarkIcon,
        ProjectSelection,
        ResourcesSelection,
        SettingsSelection,
    },
})
export default class HeaderArea extends Vue {
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

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Returns satellite name from config.
     */
    public get satelliteName(): string {
        return this.$store.state.appStateModule.satelliteName;
    }
}
</script>

<style scoped lang="scss">
    .header-container {
        font-family: 'font_regular', sans-serif;
        min-height: 62px;
        background-color: #fff;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 30px 0 10px;
        position: relative;

        &__left-area {
            display: flex;
            align-items: center;

            &__logo-area {
                width: 220px;
                margin-right: 20px;

                &__menu-button {
                    display: none;
                }

                &__close-button {
                    display: flex;
                    align-items: center;
                    cursor: pointer;

                    &__title {
                        margin-left: 13px;
                        font-size: 16px;
                        line-height: 23px;
                        color: #a9b5c1;
                    }
                }
            }
        }

        &__right-area {
            display: flex;
            align-items: center;

            &__satellite-area {
                height: 36px;
                background: #f6f6fa;
                border-radius: 58px;
                display: flex;
                align-items: center;
                padding: 0 10px 0 5px;
                margin-right: 20px;

                &__checkmark {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    width: 22px;
                    height: 22px;
                    background: #fff;
                    border-radius: 11px;
                }

                &__name {
                    font-weight: 500;
                    font-size: 14px;
                    line-height: 18px;
                    color: #000;
                }
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .adapted-navigation {
        position: absolute;
        top: 62px;
        left: 0;
        z-index: 100;
        height: 100vh;
        background: #e6e9ef;
    }

    .navigation-blur {
        height: 100vh;
        width: 100%;
        position: absolute;
        top: 62px;
        left: 220px;
        z-index: 100;
        background: rgba(134, 134, 148, 0.4);
    }

    .project-selection,
    .resources-selection {
        margin-right: 35px;
    }

    @media screen and (min-width: 1281px) {

        .adapted-navigation,
        .navigation-blur,
        .header-container__left-area__logo-area__close-button {
            display: none;
        }
    }

    @media screen and (max-width: 1280px) {

        .header-container {
            padding: 0 40px;

            &__left-area {

                &__logo-area {

                    &__menu-button {
                        display: block;
                        cursor: pointer;
                    }
                }
            }
        }

        .logo {
            display: none;
        }

        /deep/ .edit-project {
            margin-left: 0;
        }
    }
</style>
