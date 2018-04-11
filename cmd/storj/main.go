// Copyright (C) 2018 Storj Labs, Inc.
//
// This file is part of Storj CLI.
//
// Storj CLI is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// Storj CLI is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Storj CLI.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"log"
	"os"

	"storj.io/storj/internal/app/cli"
)

func main() {
	err := cli.New().Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
