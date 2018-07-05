// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package objects

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
)

func TestPutObject(t *testing.T) {
	os := Objects{}
	err := os.PutObject(context.Background(), nil, nil, nil, time.Now())
	assert.Nil(t, err)
}
func TestGetObject(t *testing.T) {
	os := Objects{}
	r, _, err := os.GetObject(context.Background(), nil)
	assert.Nil(t, r)
	assert.Nil(t, err)
}
func TestDeleteObject(t *testing.T) {
	os := Objects{}
	err := os.DeleteObject(context.Background(), nil)
	assert.Nil(t, err)
}
func TestListObjects(t *testing.T) {
	os := Objects{}
	op, trn, err := os.ListObjects(context.Background(), nil, nil)
	//objpaths := paths.Path{"objpath1", "objpath2", "objpath3"}
	objpaths := []paths.Path{{"x"}, {"objpath1", "objpath2", "objpath3"}}
	comp := reflect.DeepEqual(objpaths, op)
	assert.Equal(t, comp, true)
	assert.Equal(t, trn, true)
	assert.Nil(t, err)
}
func TestSetXAttr(t *testing.T) {
	os := Objects{}
	err := os.SetXAttr(context.Background(), nil, "", nil, nil)
	assert.Nil(t, err)
}
func TestGetXAttr(t *testing.T) {
	os := Objects{}
	r, _, err := os.GetXAttr(context.Background(), nil, "")
	assert.Nil(t, r)
	assert.Nil(t, err)
}
func TestDeleteXAttr(t *testing.T) {
	os := Objects{}
	err := os.DeleteXAttr(context.Background(), nil, "")
	assert.Nil(t, err)
}
func TestListXAttrs(t *testing.T) {
	os := Objects{}
	op, trn, err := os.ListXAttrs(context.Background(), nil, "", "")
	xattrs := []string{"xattrs1", "xattrs2", "xattrs3"}
	comp := reflect.DeepEqual(xattrs, op)
	assert.Equal(t, comp, true)
	assert.Equal(t, trn, true)
	assert.Nil(t, err)
}
