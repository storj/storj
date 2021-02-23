// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ReferralStats from '@/components/referral/ReferralStats.vue';

import { makeCreditsModule } from '@/store/modules/credits';
import { makeUsersModule, USER_ACTIONS } from '@/store/modules/users';
import { CreditUsage } from '@/types/credits';
import { User } from '@/types/users';
import { createLocalVue, mount } from '@vue/test-utils';

import { CreditsApiMock } from '../mock/api/credits';
import { UsersApiMock } from '../mock/api/users';

const {
    GET,
} = USER_ACTIONS;

const localVue = createLocalVue();
localVue.use(Vuex);

const mockUser = new User('1', 'full name', 'short name');
const mockCredits = new CreditUsage(1, 2, 3);

const creditsApi = new CreditsApiMock();
creditsApi.setMockCredits(mockCredits);
const usersApi = new UsersApiMock();
usersApi.setMockUser(mockUser);

const usersModule = makeUsersModule(usersApi);
const creditsModule = makeCreditsModule(creditsApi);

const store = new Vuex.Store({ modules: { usersModule, creditsModule } });

store.dispatch(GET);

describe('ReferralStats', () => {
    it('fetch method populate store', async () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        await wrapper.vm.fetch();

        const storeState = (store.state as any).creditsModule;

        expect(storeState.referred).toBe(mockCredits.referred);
        expect(storeState.usedCredits).toBe(mockCredits.usedCredits);
        expect(storeState.availableCredits).toBe(mockCredits.availableCredits);
    });

    it('snapshot not changed', () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('title renders correctly', () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        const title = wrapper.find('.referral-stats__title');

        expect(title.text()).toBe(`${mockUser.fullName}, ${wrapper.vm.$data.TITLE_SUFFIX}`);
    });

    it('credits type title renders correctly', () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        const titles = wrapper.findAll('.referral-stats__card-title');

        expect(titles.at(0).text()).toBe(wrapper.vm.$data.stats[0].title);
        expect(titles.at(1).text()).toBe(wrapper.vm.$data.stats[1].title);
        expect(titles.at(2).text()).toBe(wrapper.vm.$data.stats[2].title);
    });

    it('credits type description renders correctly', () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        const descriptions = wrapper.findAll('.referral-stats__card-description');

        expect(descriptions.at(0).text()).toBe(wrapper.vm.$data.stats[0].description);
        expect(descriptions.at(1).text()).toBe(wrapper.vm.$data.stats[1].description);
        expect(descriptions.at(2).text()).toBe(wrapper.vm.$data.stats[2].description);
    });

    it('credits type symbol with value renders correctly', () => {
        const wrapper = mount(ReferralStats, {
            store,
            localVue,
        });

        const descriptions = wrapper.findAll('.referral-stats__card-number');

        // TODO: write assertions when 'stat.symbol + usage[key]' will be fixed
    });
});
