// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export async function loadFonts(): Promise<void> {
    const webFontLoader = await import('webfontloader');

    webFontLoader.load({
        google: {
            families: ['Inter:400,600,800&display=swap'],
        },
    });
}
