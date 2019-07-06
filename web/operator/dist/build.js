// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

var xmlHttp = new XMLHttpRequest();

xmlHttp.open("GET", "/api/dashboard?satelliteId=12D1kqUXtJiCsS72UKeCGHyKSQy1FSonerZ5fb6nSQAaSim4Vag", false );
xmlHttp.send();

alert(xmlHttp.responseText);