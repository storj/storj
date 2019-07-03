// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//var xmlHttp = new XMLHttpRequest();

//xmlHttp.open("GET", "/api/dashboard?satelliteId=12D1kqUXtJiCsS72UKeCGHyKSQy1FSonerZ5fb6nSQAaSim4Vag", false);
//xmlHttp.send();

var ws = new WebSocket("ws://localhost:14002/api/update/");

console.log(ws);

//ws.send("12D1kqUXtJiCsS72UKeCGHyKSQy1FSonerZ5fb6nSQAaSim4Vag");
ws.addEventListener("message", function(e) {console.log(e)});

//console.log(xmlHttp.responseText);
