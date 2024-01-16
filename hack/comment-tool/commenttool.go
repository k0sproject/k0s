/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func errorOut(msg ...string) {
	println(strings.Join(msg, " "))
	os.Exit(1)
}

func main() {
	fixFlag := flag.Bool("fix", false, "Fix the file in place")
	flag.Parse()

	if flag.NArg() == 0 {
		errorOut("usage: commenttool [filename ...]")
	}

	for _, filename := range flag.Args() {
		file, err := os.OpenFile(filename, os.O_RDWR, 0)
		if err != nil {
			errorOut("failed to open file", filename, ":", err.Error())
		}
		if *fixFlag {
			fmt.Println("Processing", filename)
		}
		out, err := ProcessFile(filename, file)
		if err != nil {
			println("error in file", filename, ":", err.Error())
			continue
		}
		if *fixFlag {
			if err := file.Truncate(0); err != nil {
				errorOut("failed to truncate file", filename, ":", err.Error())
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				errorOut("failed to seek to beginning of file", filename, ":", err.Error())
			}
			if _, err := io.Copy(file, out); err != nil {
				errorOut("failed to write to file", filename, ":", err.Error())
			}
		} else {
			_, _ = io.Copy(os.Stdout, out)
		}
		if err := file.Close(); err != nil {
			errorOut("failed to close file", filename, ":", err.Error())
		}
	}
}
