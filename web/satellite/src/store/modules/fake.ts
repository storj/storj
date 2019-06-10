// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const fakeModule = {
    state: {
        node: {
            id: "12QKex7UUaFeX728x6divdRUApCsm2QybxTdvuWbG1SRdmJqfd1",
            status: 'Online',
            version: 'v0.11.1',
            wallet: '0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac',
        },

        satellite: {
            list: [
                {
                    name: 'US-East-1',
                    id: 0,
                    isSelected: false,
                },
                {
                    name: 'Two',
                    id: 1,
                    isSelected: false,
                },
                {
                    name: 'Three',
                    id: 2,
                    isSelected: false,
                },
                {
                    name: 'Four',
                    id: 3,
                    isSelected: false,
                },
                {
                    name: 'Five',
                    id: 4,
                    isSelected: false,
                },
            ],
            selected: 'US-East-1',
        },
    },
};
