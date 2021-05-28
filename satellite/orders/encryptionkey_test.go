// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/orders"
)

func TestEncryptionKeys_New(t *testing.T) {
	var key1, key2 orders.EncryptionKey
	require.NoError(t, key1.Set(`11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF`))
	require.NoError(t, key2.Set(`0100000000000000=0100000000000000000000000000000000000000000000000000000000000000`))
	ekeys, err := orders.NewEncryptionKeys(key1, key2)
	require.NoError(t, err)
	require.Equal(t, ekeys.Default.Key, key1.Key)
	require.Equal(t, ekeys.Default.ID, key1.ID)
	const keyCount = 2
	require.Equal(t, len(ekeys.KeyByID), keyCount)
	require.Equal(t, len(ekeys.List), keyCount)
}
func TestEncryptionKey_Set_Valid(t *testing.T) {
	type Test struct {
		Hex string
		Key orders.EncryptionKey
	}

	tests := []Test{
		{
			Hex: `0100000000000000=0100000000000000000000000000000000000000000000000000000000000000`,
			Key: orders.EncryptionKey{
				ID:  orders.EncryptionKeyID{0x01},
				Key: storj.Key{0x01},
			},
		},
		{
			Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF`,
			Key: orders.EncryptionKey{
				ID: orders.EncryptionKeyID{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0xFF},
				Key: storj.Key{
					0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
					0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
					0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
					0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0xFF,
				},
			},
		},
	}

	for _, test := range tests {
		var got orders.EncryptionKey

		err := got.Set(test.Hex)
		assert.NoError(t, err, test.Hex)
		assert.Equal(t, test.Key, got, test.Hex)
	}
}

func TestEncryptionKey_Set_Invalid(t *testing.T) {
	type Test struct {
		Hex string
	}

	tests := []Test{
		{Hex: ``},
		{Hex: `=`},
		{Hex: `01=`},
		{Hex: `=01`},
		{Hex: `1=1`},

		{Hex: `=1122334455667788112233445566778811223344556677881122334455667788`},
		{Hex: `112233445566778=1122334455667788112233445566778811223344556677881122334455667788`},

		{Hex: `1122334455667788=`},
		{Hex: `1122334455667788=112233445566778811223344556677881122334455667788112233445566778`},

		{Hex: `11223344556677QQ=11223344556677881122334455667788112233445566778811223344556677QQ`},
	}

	for _, test := range tests {
		var got orders.EncryptionKey

		err := got.Set(test.Hex)
		assert.Error(t, err, test.Hex)
	}
}

func TestEncryptionKeys_Set_Valid(t *testing.T) {
	var keys orders.EncryptionKeys

	err := keys.Set(`11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,0100000000000000=0100000000000000000000000000000000000000000000000000000000000000`)
	require.NoError(t, err)

	first := orders.EncryptionKey{
		ID: orders.EncryptionKeyID{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0xFF},
		Key: storj.Key{
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0xFF,
		},
	}
	second := orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{0x01},
		Key: storj.Key{0x01},
	}

	assert.Equal(t, first, keys.Default)
	assert.EqualValues(t, []orders.EncryptionKey{first, second}, keys.List)

	assert.Equal(t, first.Key, keys.KeyByID[first.ID])
	assert.Equal(t, second.Key, keys.KeyByID[second.ID])
}

func TestEncryptionKeys_Set_Invalid(t *testing.T) {
	type Test struct {
		Hex string
	}

	tests := []Test{
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,=`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,01=`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,=01`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,1=1`},

		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,=1122334455667788112233445566778811223344556677881122334455667788`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,112233445566778=1122334455667788112233445566778811223344556677881122334455667788`},

		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,1122334455667788=`},
		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,1122334455667788=112233445566778811223344556677881122334455667788112233445566778`},

		{Hex: `11223344556677FF=11223344556677881122334455667788112233445566778811223344556677FF,11223344556677QQ=11223344556677881122334455667788112233445566778811223344556677QQ`},
	}

	for _, test := range tests {
		var got orders.EncryptionKeys

		err := got.Set(test.Hex)
		assert.Error(t, err, test.Hex)
	}
}

func randEncryptionKey() orders.EncryptionKey {
	k := orders.EncryptionKey{}
	r := testrand.Nonce()
	copy(k.ID[:], r[:])
	k.Key = testrand.Key()
	return k
}

func TestEncryptionKey_EncryptDecrypt(t *testing.T) {
	for i := 0; i < 10; i++ {
		key := randEncryptionKey()
		data := testrand.BytesInt(16)
		serial := testrand.SerialNumber()

		encrypted := key.Encrypt(data, serial)
		require.NotEqual(t, encrypted, data)

		decrypted, err := key.Decrypt(encrypted, serial)
		require.NoError(t, err)
		require.Equal(t, data, decrypted)
	}
}

func TestEncryptionKey_BackwardsCompatibility(t *testing.T) {
	type Test struct {
		Key          string
		DataHex      string
		Serial       storj.SerialNumber
		EncryptedHex string
	}

	tests := []Test{
		{
			Key:          "1d729566c74d1003=0d86d1e91e00167939cb6694d2c422acd208a0072939487f6999eb9d18a44784",
			DataHex:      "045d87f3c67cf22746e995af5a253679",
			Serial:       storj.SerialNumber{81, 186, 162, 255, 108, 212, 113, 196, 131, 241, 95, 185, 11, 173, 179, 124},
			EncryptedHex: "5fe4c9fc734baed429afe24502ffe9d4c6efd99de5d5ae07433f7449bc55e4e1",
		}, {
			Key:          "5821b6d95526a41a=86216325253fec738dd7a9e28bf921119c160f0702448615bbda08313f6a8eb6",
			DataHex:      "68d20bf5059875921e668a5bdf2c7fc4",
			Serial:       storj.SerialNumber{132, 69, 146, 210, 87, 43, 205, 6, 104, 210, 214, 197, 47, 80, 84, 226},
			EncryptedHex: "335eb0ab85e0624862b63c6bad78c4bdd1d52db621332773aef29b977626bcd0",
		}, {
			Key:          "d0836bf84c7174cb=358b0c3b525da1786f9fff094279db1944ebd7a19d0f7bbacbe0255aa5b7d44b",
			DataHex:      "ec40f84c892b9bffd43629b0223beea5",
			Serial:       storj.SerialNumber{244, 247, 67, 145, 244, 69, 209, 90, 253, 66, 148, 4, 3, 116, 246, 146},
			EncryptedHex: "2615a21b9968f4fa7fdc5eb423f708a0630ec25295852e513c658eecaee0493e",
		}, {
			Key:          "4b98cbf8713f8d96=b586b14323a6bc8f9e7df1d929333ff993933bea6f5b3af6de0374366c4719e4",
			DataHex:      "3a1b067d89bc7f01f1f573981659a44f",
			Serial:       storj.SerialNumber{241, 122, 76, 114, 21, 163, 181, 57, 235, 30, 88, 73, 198, 7, 125, 187},
			EncryptedHex: "e57f0b0e936bff96895aae6f963e0ff0a14350d2f787dda47116de0e1a7bf390",
		}, {
			Key:          "5722f5717a289a26=5e82ed6f4125c8fa7311e4d7defa922daae7786667f7e936cd4f24abf7df866b",
			DataHex:      "aa56038367ad6145de1ee8f4a8b0993e",
			Serial:       storj.SerialNumber{189, 248, 136, 58, 10, 216, 190, 156, 57, 120, 176, 72, 131, 229, 106, 21},
			EncryptedHex: "8a238f268df2ae5a58134e2f52a813ffc157e0f12d8430268f55712c6bf8aae9",
		},
	}

	for _, test := range tests {
		var key orders.EncryptionKey
		require.NoError(t, key.Set(test.Key))

		data, err := hex.DecodeString(test.DataHex)
		require.NoError(t, err)
		encrypted, err := hex.DecodeString(test.EncryptedHex)
		require.NoError(t, err)

		decrypted, err := key.Decrypt(encrypted, test.Serial)
		require.NoError(t, err)

		require.Equal(t, data, decrypted)
	}
}
