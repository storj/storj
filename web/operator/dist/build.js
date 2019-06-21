// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

var xmlHttp = new XMLHttpRequest();

xmlHttp.open("GET", "/api/dashboard/", false );
xmlHttp.send();

alert(xmlHttp.responseText);