// +build js,wasm
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/sha256"
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/uplink/private/access2"
)

// Permission defines what actions can be used to share.
//
// This struct has been taken from storj.io/uplink and duplicated to avoid
// pulling in that dependency.
type Permission struct {
	// AllowDownload gives permission to download the object's content. It
	// allows getting object metadata, but it does not allow listing buckets.
	AllowDownload bool
	// AllowUpload gives permission to create buckets and upload new objects.
	// It does not allow overwriting existing objects unless AllowDelete is
	// granted too.
	AllowUpload bool
	// AllowList gives permission to list buckets. It allows getting object
	// metadata, but it does not allow downloading the object's content.
	AllowList bool
	// AllowDelete gives permission to delete buckets and objects. Unless
	// either AllowDownload or AllowList is granted too, no object metadata and
	// no error info will be returned for deleted objects.
	AllowDelete bool
	// NotBefore restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is before NotBefore.
	// If set, this value should always be before NotAfter.
	NotBefore time.Time
	// NotAfter restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is after NotAfter.
	// If set, this value should always be after NotBefore.
	NotAfter time.Time
}

func main() {
	js.Global().Set("generateAccessGrant", generateAccessGrant())
	js.Global().Set("setAPIKeyPermission", setAPIKeyPermission())
	js.Global().Set("newPermission", newPermission())
	<-make(chan bool)
}

func generateAccessGrant() js.Func {
	return js.FuncOf(responseHandler(func(this js.Value, args []js.Value) (interface{}, error) {
		if len(args) < 4 {
			return nil, errs.New("not enough arguments. Need 4, but only %d supplied. The order of arguments are: satellite Node URL, API key, encryption passphrase, and project ID.", len(args))
		}
		satelliteNodeURL := args[0].String()
		apiKey := args[1].String()
		encryptionPassphrase := args[2].String()
		projectSalt := args[3].String()

		access, err := genAccessGrant(satelliteNodeURL,
			apiKey,
			encryptionPassphrase,
			projectSalt,
		)
		if err != nil {
			return nil, err
		}

		return access, nil
	}))
}

func genAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, projectID string) (string, error) {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	id, err := uuid.FromString(projectID)
	if err != nil {
		return "", err
	}

	const concurrency = 8
	salt := sha256.Sum256(id[:])

	key, err := encryption.DeriveRootKey([]byte(encryptionPassphrase), salt[:], "", concurrency)
	if err != nil {
		return "", err
	}

	encAccess := access2.NewEncryptionAccessWithDefaultKey(key)
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	a := &access2.Access{
		SatelliteAddress: satelliteNodeURL,
		APIKey:           parsedAPIKey,
		EncAccess:        encAccess,
	}
	accessString, err := a.Serialize()
	if err != nil {
		return "", err
	}
	return accessString, nil
}

// setAPIKeyPermission creates a new api key with specific permissions.
func setAPIKeyPermission() js.Func {
	return js.FuncOf(responseHandler(func(this js.Value, args []js.Value) (interface{}, error) {
		if len(args) < 3 {
			return nil, errs.New("not enough arguments. Need 3, but only %d supplied. The order of arguments are: API key, bucket names, and permission object.", len(args))
		}
		apiKey := args[0].String()

		// convert array of bucket names to go []string type
		buckets := args[1]
		if ok := buckets.InstanceOf(js.Global().Get("Array")); !ok {
			return nil, errs.New("invalid data type. Expect Array, Got %s", buckets.Type().String())
		}
		bucketNames, err := parseArrayOfStrings(buckets)
		if err != nil {
			return nil, err
		}

		// convert js permission to go permission type
		permissionJS := args[2]
		if permissionJS.Type() != js.TypeObject {
			return nil, errs.New("invalid argument type. Expect %s, Got %s", js.TypeObject.String(), permissionJS.Type().String())
		}
		permission, err := parsePermission(permissionJS)
		if err != nil {
			return nil, err
		}

		restrictedKey, err := setPermission(apiKey, bucketNames, permission)
		if err != nil {
			return nil, err
		}

		return restrictedKey.Serialize(), nil
	}))
}

// newPermission creates a new permission object.
func newPermission() js.Func {
	return js.FuncOf(responseHandler(func(this js.Value, args []js.Value) (interface{}, error) {
		p, err := json.Marshal(Permission{})
		if err != nil {
			return nil, err
		}

		var jsObj map[string]interface{}
		err = json.Unmarshal(p, &jsObj)
		if err != nil {
			return nil, err
		}
		return jsObj, nil
	}))
}

func setPermission(key string, buckets []string, permission Permission) (*macaroon.APIKey, error) {
	if permission == (Permission{}) {
		return nil, errs.New("permission is empty")
	}

	var notBefore, notAfter *time.Time
	if !permission.NotBefore.IsZero() {
		notBefore = &permission.NotBefore
	}
	if !permission.NotAfter.IsZero() {
		notAfter = &permission.NotAfter
	}

	if notBefore != nil && notAfter != nil && notAfter.Before(*notBefore) {
		return nil, errs.New("invalid time range")
	}

	caveat := macaroon.Caveat{
		DisallowReads:   !permission.AllowDownload,
		DisallowWrites:  !permission.AllowUpload,
		DisallowLists:   !permission.AllowList,
		DisallowDeletes: !permission.AllowDelete,
		NotBefore:       notBefore,
		NotAfter:        notAfter,
	}

	for _, b := range buckets {
		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{
			Bucket: []byte(b),
		})
	}

	apiKey, err := macaroon.ParseAPIKey(key)
	if err != nil {
		return nil, err
	}

	restrictedKey, err := apiKey.Restrict(caveat)
	if err != nil {
		return nil, err
	}

	return restrictedKey, nil
}

func parsePermission(arg js.Value) (Permission, error) {
	var permission Permission

	// convert javascript object to a json string
	jsJSON := js.Global().Get("JSON")
	p := jsJSON.Call("stringify", arg)

	err := json.Unmarshal([]byte(p.String()), &permission)
	if err != nil {
		return permission, err
	}

	return permission, nil
}

func parseArrayOfStrings(arg js.Value) ([]string, error) {
	data := make([]string, arg.Length())
	for i := 0; i < arg.Length(); i++ {
		data[i] = arg.Index(i).String()
	}

	return data, nil
}

type result struct {
	value interface{}
	err   error
}

func (r result) ToJS() map[string]interface{} {
	var errMsg string
	if r.err != nil {
		errMsg = r.err.Error()
	}
	return map[string]interface{}{
		"value": js.ValueOf(r.value),
		"error": errMsg,
	}
}

func responseHandler(fn func(this js.Value, args []js.Value) (value interface{}, err error)) func(js.Value, []js.Value) interface{} {
	return func(this js.Value, args []js.Value) interface{} {
		value, err := fn(this, args)
		return result{value, err}.ToJS()
	}
}
