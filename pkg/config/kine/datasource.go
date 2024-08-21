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

package kine

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

// Splits a kine data source string into the backend and DSN parts.
//
// Kine data sources are of the form "<backend>://<dsn>".
// They look like URLs, but they aren't, so don't try to use [net/url.Parse].
func SplitDataSource(dataSource string) (backend, dsn string, _ error) {
	backend, dsn, ok := strings.Cut(dataSource, "://")

	// Kine behaves weirdly if the infix isn't found or the backend is empty: It
	// defaults to SQLite with an empty DSN, no matter what the data source is.
	// Let's not duplicate this in k0s, but insist on something with an infix.

	if !ok {
		return "", "", errors.New("failed to find infix between driver and DSN")
	}

	switch backend {
	case "":
		return "", "", errors.New("no backend specified")
	case "nats":
		dsn = dataSource
	case "http", "https":
		backend = "etcd3"
	}

	return backend, dsn, nil
}

// Gets the file system path of the SQLite database file from an SQLite DSN.
func GetSQLiteFilePath(workingDir, dsn string) (string, error) {
	// The DSN is preprocessed by kine's SQLite Go database driver, and not
	// passed to the SQLite library as is:
	if pos := strings.IndexByte(dsn, '?'); pos >= 1 {
		if !strings.HasPrefix(dsn, "file:") {
			dsn = dsn[:pos]
		}
	}

	// Now rely on the SQLite library's URI semantics.
	//
	// https://www.sqlite.org/c3ref/open.html
	// https://www.sqlite.org/uri.html

	// The DSN is treated as the file name if it's not a file URI.
	fileName := dsn
	uri, err := url.Parse(dsn)
	if err == nil && uri.Scheme == "file" {
		fileName = filepath.FromSlash(uri.Path)
	}

	switch fileName {
	case "":
		return "", errors.New("private temporary on-disk database")
	case ":memory:":
		return "", errors.New("in-memory database")
	}

	// Kine/SQLite should treat relative paths as relative to their working
	// directory, which _should_ be k0s's data dir.
	if !filepath.IsAbs(fileName) {
		return filepath.Join(workingDir, dsn), nil
	}

	// Clean the file path. This is to be consistent with filepath.Join which
	// cleans the path internally, too.
	return filepath.Clean(fileName), nil
}
