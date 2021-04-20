/*
Copyright 2021 k0s authors

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
package backup

// import (
// 	"fmt"
// 	"net/url"
// 	"os"
// 	"path/filepath"

// 	"github.com/rqlite/rqlite/db"
// 	"github.com/sirupsen/logrus"
// )

// func (c *Config) saveSQLiteDB() error {
// 	dbPath, err := c.getKineDBPath()
// 	if err != nil {
// 		return err
// 	}
// 	kineDB, err := db.Open(dbPath)
// 	path := filepath.Join(c.tmpDir, kineBackup)

// 	logrus.Debugf("exporting kine db to %v", path)
// 	f, err := os.Create(path)
// 	if err != nil {
// 		return fmt.Errorf("failed to create kine backup: %v", err)
// 	}
// 	// create a hot backup of the kine db
// 	err = kineDB.Dump(f)
// 	if err != nil {
// 		return fmt.Errorf("failed to back-up kine db: %v", err)
// 	}
// 	c.savedAssets = append(c.savedAssets, path)
// 	return nil
// }

// func (c *Config) getKineDBPath() (string, error) {
// 	u, err := url.Parse(c.storageSpec.Kine.DataSource)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to parse Kind datasource string: %v", err)
// 	}
// 	logrus.Debugf("kinedb path: %v", u.Path)
// 	return u.Path, nil
// }
