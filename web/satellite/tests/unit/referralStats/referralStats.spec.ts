// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import ReferralStats from '@/components/referral/ReferralStats.vue';
import { CreditsApi, CreditUsage } from '@/types/credits';
import { UpdatedUser, User, UsersApi } from '@/types/users';
import { makeCreditsModule } from '@/store/modules/credits';
import { makeUsersModule } from '@/store/modules/users';
import { USER_ACTIONS } from '@/utils/constants/actionNames';

const {
    GET,
} = USER_ACTIONS;

const localVue = createLocalVue();
localVue.use(Vuex);

const mockUser = new User('1', 'full name', 'short name');
const mockCredits = new CreditUsage(1, 2, 3);

class UsersApiMock implements UsersApi {
    get(): Promise<User> {
        return Promise.resolve(mockUser);
    }

    update(user: UpdatedUser): Promise<void> {
        throw new Error('not implemented');
    }

}
class CreditsApiMock implements CreditsApi {
    get(): Promise<CreditUsage> {
        return Promise.resolve(mockCredits);
    }
}

const creditsApi = new CreditsApiMock();
const usersApi = new UsersApiMock();

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
