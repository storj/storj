// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

func TestOrderLimitVerification(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	signer, err := identity.FullIdentityFromPEM(
		[]byte("-----BEGIN CERTIFICATE-----\nMIIBYjCCAQigAwIBAgIRAMM/5SHfNFMLl9uTAAQEoZAwCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS/\n9wOAe42DV90jcRJMMeGe9os528RNJbMthDMkAn58KyOH87Rvlz0uCRnhhk3AbDE+\nXXHfEyed/HPFEMxJwmlGoz8wPTAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYI\nKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwIDSAAw\nRQIhALl9VMhM6NFnPblqOsIHOznsKr0OfQREf/+GSk/t8McsAiAxyOYg3IlB9iA0\nq/pD+qUwXuS+NFyVGOhgdNDFT3amOA==\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWzCCAQGgAwIBAgIRAMfle+YJvbpRwr+FqiTrRyswCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARL\nO4n2UCp66X/MY5AzhZsfbBYOBw81Dv8V3y1BXXtbHNsUWNY8RT7r5FSTuLHsaXwq\nTwHdU05bjgnLZT/XdwqaozgwNjAOBgNVHQ8BAf8EBAMCAgQwEwYDVR0lBAwwCgYI\nKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEA2vce\nasP0sjt6QRJNkgdV/IONJCF0IGgmsCoogCbh9ggCIA3mHgivRBId7sSAU4UUPxpB\nOOfce7bVuJlxvsnNfkkz\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWjCCAQCgAwIBAgIQdzcArqh7Yp9aGiiJXM4+8TAKBggqhkjOPQQDAjAQMQ4w\nDAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw\nWjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABM/W\nTxYhs/yGKSg8+Hb2Z/NB2KJef+fWkq7mHl7vhD9JgFwVMowMEFtKOCAhZxLBZD47\nxhYDhHBv4vrLLS+m3wGjODA2MA4GA1UdDwEB/wQEAwICBDATBgNVHSUEDDAKBggr\nBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0gAMEUCIC+gM/sI\nXXHq5jJmolw50KKVHlqaqpdxjxJ/6x8oqTHWAiEA1w9EbqPXQ5u/oM+ODf1TBkms\nN9NfnJsY1I2A3NKEvq8=\n-----END CERTIFICATE-----\n"),
		[]byte("-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgzsFsVqt/GdqQlIIJ\nHH2VQNndv1A1fTk/35VPNzLW04ehRANCAATzXrIfcBZAHHxPdFD2PFRViRwe6eWf\nQipaF4iXQmHAW79X4mDx0BibjFfvmzurnYSlyIMZn3jp9RzbLMfnA10C\n-----END PRIVATE KEY-----\n"),
	)
	require.NoError(t, err)

	signee := signing.SignerFromFullIdentity(signer)

	hexes := []string{
		// 385c0467
		"0a1052fdfc072182654f163f5f0f9a621d7212200ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac30001a209566c74d10037c4d7bbb0407d1e2c64981855ad8681d0d86d1e91e001679390022206694d2c422acd208a0072939487f6999eb9d18a44784045d87f3c67cf22746002a2095af5a25367951baa2ff6cd471c483f15fb90badb37c5821b6d95526a41a950430904e3802420c088bd2a2e90510b0f18a8f034a0c088bd2a2e90510b0f18a8f035247304502201f90141b29d7fda0592431ef5ac5a4a46dcec5f7e0ffeac4bccd4dda3a78d5a0022100c21eab116549c22e50e03d1d3829de6f1fcf933e4eec974ae3c3d94a9504a07c5a1f121d68747470733a2f2f736174656c6c6974652e6578616d706c652e636f6d",
		// 385c0467 without satellite address
		"0a1052fdfc072182654f163f5f0f9a621d7212200ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac30001a209566c74d10037c4d7bbb0407d1e2c64981855ad8681d0d86d1e91e001679390022206694d2c422acd208a0072939487f6999eb9d18a44784045d87f3c67cf22746002a2095af5a25367951baa2ff6cd471c483f15fb90badb37c5821b6d95526a41a950430904e3802420c088bd2a2e90510b0f18a8f034a0c088bd2a2e90510b0f18a8f03524830460221009ebb9e39f650dee0bf5b2eff3520198ead66952ae85f4fc240ebcdeb58c1384a022100df0d69c4fe00a555a041455524f00931ccf7a789513d7d2676dc8c01d46728b15a1f121d68747470733a2f2f736174656c6c6974652e6578616d706c652e636f6d",
		// 385c0467 without piece expiration
		"0a1052fdfc072182654f163f5f0f9a621d7212200ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac30001a209566c74d10037c4d7bbb0407d1e2c64981855ad8681d0d86d1e91e001679390022206694d2c422acd208a0072939487f6999eb9d18a44784045d87f3c67cf22746002a2095af5a25367951baa2ff6cd471c483f15fb90badb37c5821b6d95526a41a950430904e3802420c088bd2a2e90510b0f18a8f034a0c088bd2a2e90510b0f18a8f0352483046022100ab4882fd1f5232267e70c42f36c0572d6188019077530958d4596f3bc585221202210099835ad862cdccf02c94878498c14352c0b7909f79a58d82cc461aed6c39d54b5a1f121d68747470733a2f2f736174656c6c6974652e6578616d706c652e636f6d",
	}
	/*
		now := ptypes.TimestampNow()

		limit := pb.OrderLimit{
			SerialNumber: testrand.SerialNumber(),
			SatelliteId:  signee.ID(),
			SatelliteAddress: &pb.NodeAddress{
				Address: "https://satellite.example.com",
			},
			UplinkId:        testrand.NodeID(),
			StorageNodeId:   testrand.NodeID(),
			PieceId:         testrand.PieceID(),
			Action:          pb.PieceAction_GET,
			Limit:           10000,
			PieceExpiration: now,
			OrderExpiration: now,
		}
		limitSigned, err := signing.SignOrderLimit(ctx, signee, &limit)
		require.NoError(t, err)
		limitBytes, err := proto.Marshal(limitSigned)
		require.NoError(t, err)
		hexes = append(hexes, hex.EncodeToString(limitBytes))

		limitx := limit
		limitx.SatelliteAddress = nil

		limitSigned, err = signing.SignOrderLimit(ctx, signee, &limit)
		require.NoError(t, err)
		limitBytes, err = proto.Marshal(limitSigned)
		require.NoError(t, err)
		hexes = append(hexes, hex.EncodeToString(limitBytes))

		limitx = limit
		limitx.PieceExpiration = nil

		limitSigned, err = signing.SignOrderLimit(ctx, signee, &limit)
		require.NoError(t, err)
		limitBytes, err = proto.Marshal(limitSigned)
		require.NoError(t, err)
		hexes = append(hexes, hex.EncodeToString(limitBytes))
	*/
	for _, orderLimitHex := range hexes {
		fmt.Println(orderLimitHex)

		orderLimitBytes, err := hex.DecodeString(orderLimitHex)
		require.NoError(t, err)

		orderLimit := pb.OrderLimit{}
		err = proto.Unmarshal(orderLimitBytes, &orderLimit)
		require.NoError(t, err)

		err = signing.VerifyOrderLimitSignature(ctx, signee, &orderLimit)
		require.NoError(t, err)
	}
}

func TestOrderVerification(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	signer, err := identity.FullIdentityFromPEM(
		[]byte("-----BEGIN CERTIFICATE-----\nMIIBYjCCAQigAwIBAgIRAMM/5SHfNFMLl9uTAAQEoZAwCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS/\n9wOAe42DV90jcRJMMeGe9os528RNJbMthDMkAn58KyOH87Rvlz0uCRnhhk3AbDE+\nXXHfEyed/HPFEMxJwmlGoz8wPTAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYI\nKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwIDSAAw\nRQIhALl9VMhM6NFnPblqOsIHOznsKr0OfQREf/+GSk/t8McsAiAxyOYg3IlB9iA0\nq/pD+qUwXuS+NFyVGOhgdNDFT3amOA==\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWzCCAQGgAwIBAgIRAMfle+YJvbpRwr+FqiTrRyswCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARL\nO4n2UCp66X/MY5AzhZsfbBYOBw81Dv8V3y1BXXtbHNsUWNY8RT7r5FSTuLHsaXwq\nTwHdU05bjgnLZT/XdwqaozgwNjAOBgNVHQ8BAf8EBAMCAgQwEwYDVR0lBAwwCgYI\nKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEA2vce\nasP0sjt6QRJNkgdV/IONJCF0IGgmsCoogCbh9ggCIA3mHgivRBId7sSAU4UUPxpB\nOOfce7bVuJlxvsnNfkkz\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWjCCAQCgAwIBAgIQdzcArqh7Yp9aGiiJXM4+8TAKBggqhkjOPQQDAjAQMQ4w\nDAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw\nWjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABM/W\nTxYhs/yGKSg8+Hb2Z/NB2KJef+fWkq7mHl7vhD9JgFwVMowMEFtKOCAhZxLBZD47\nxhYDhHBv4vrLLS+m3wGjODA2MA4GA1UdDwEB/wQEAwICBDATBgNVHSUEDDAKBggr\nBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0gAMEUCIC+gM/sI\nXXHq5jJmolw50KKVHlqaqpdxjxJ/6x8oqTHWAiEA1w9EbqPXQ5u/oM+ODf1TBkms\nN9NfnJsY1I2A3NKEvq8=\n-----END CERTIFICATE-----\n"),
		[]byte("-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgzsFsVqt/GdqQlIIJ\nHH2VQNndv1A1fTk/35VPNzLW04ehRANCAATzXrIfcBZAHHxPdFD2PFRViRwe6eWf\nQipaF4iXQmHAW79X4mDx0BibjFfvmzurnYSlyIMZn3jp9RzbLMfnA10C\n-----END PRIVATE KEY-----\n"),
	)
	require.NoError(t, err)

	signee := signing.SignerFromFullIdentity(signer)

	hexes := []string{
		`0a1068d2d6c52f5054e2d0836bf84c7174cb10e8071a4730450220531f1caceb78e4bd887ef236cebaf37b3fcc5f7d584078f4d5e1314e7d58506e022100d1b6fe27a49abd373af04ab915178578baa4fcb9629755d7d02cc1d61b529d87`,
	}
	/*
		now := ptypes.TimestampNow()
		_ = now
		limit := pb.Order{
			SerialNumber: testrand.SerialNumber(),
			Amount:       1000,
		}
		limitSigned, err := signing.SignOrder(ctx, signee, &limit)
		require.NoError(t, err)
		limitBytes, err := proto.Marshal(limitSigned)
		require.NoError(t, err)
		hexes = append(hexes, hex.EncodeToString(limitBytes))
	*/
	for _, orderHex := range hexes {
		fmt.Println(orderHex)
		orderBytes, err := hex.DecodeString(orderHex)
		require.NoError(t, err)

		order := pb.Order{}
		err = proto.Unmarshal(orderBytes, &order)
		require.NoError(t, err)

		err = signing.VerifyOrderSignature(ctx, signee, &order)
		require.NoError(t, err)
	}
}
