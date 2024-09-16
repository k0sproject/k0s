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

package worker

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/stretchr/testify/require"
)

func TestGetImageSources(t *testing.T) {
	// test image without label
	got, err := GetImageSources(images.Image{})
	require.Nil(t, err)
	require.Equal(t, ImageSources{}, got)

	// test image with label
	when := time.Now().Truncate(time.Hour)
	value := map[string]time.Time{"path": when}
	data, err := json.Marshal(value)
	require.Nil(t, err)

	image := images.Image{
		Labels: map[string]string{
			ImageSourcePathsLabel: string(data),
		},
	}
	expected := ImageSources{"path": when}
	got, err = GetImageSources(image)
	require.Nil(t, err)
	require.True(t, expected["path"].Equal(got["path"]), "dates mismatch")

	// test image with invalid label
	image = images.Image{
		Labels: map[string]string{
			ImageSourcePathsLabel: "invalid",
		},
	}
	_, err = GetImageSources(image)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal label")
}

func TestSetImageSources(t *testing.T) {
	// test adding empty sources
	image := images.Image{}
	err := SetImageSources(&image, ImageSources{})
	require.Nil(t, err)
	require.Empty(t, image.Labels)

	// test setting one source
	fp, err := os.CreateTemp("", "test")
	require.Nil(t, err)
	defer func() {
		_ = fp.Close()
		_ = os.Remove(fp.Name())
	}()
	info, err := fp.Stat()
	require.Nil(t, err)
	image = images.Image{}
	expected := ImageSources{fp.Name(): info.ModTime()}
	err = SetImageSources(&image, expected)
	require.Nil(t, err)
	got, err := GetImageSources(image)
	require.Nil(t, err)
	require.True(t, expected[fp.Name()].Equal(got[fp.Name()]), "dates mismatch")

	// test sources replacement
	img0, err := os.CreateTemp("", "test")
	require.Nil(t, err)
	defer func() {
		_ = img0.Close()
		_ = os.Remove(img0.Name())
	}()
	info0, err := img0.Stat()
	require.Nil(t, err)

	data, err := json.Marshal(map[string]time.Time{img0.Name(): info0.ModTime()})
	require.Nil(t, err)
	image = images.Image{
		Labels: map[string]string{ImageSourcePathsLabel: string(data)},
	}

	img1, err := os.CreateTemp("", "test")
	require.Nil(t, err)
	defer func() {
		_ = img1.Close()
		_ = os.Remove(img1.Name())
	}()
	info1, err := img1.Stat()
	require.Nil(t, err)

	newsrc := ImageSources{img1.Name(): info1.ModTime()}
	err = SetImageSources(&image, newsrc)
	require.Nil(t, err)

	expected = ImageSources{img1.Name(): info1.ModTime()}
	got, err = GetImageSources(image)
	require.Nil(t, err)
	require.True(t, expected[img1.Name()].Equal(got[img1.Name()]), "dates mismatch")
}

func TestAddToImageSources(t *testing.T) {
	// test replacing sources
	img0, err := os.CreateTemp("", "test")
	require.Nil(t, err)
	defer func() {
		_ = img0.Close()
		_ = os.Remove(img0.Name())
	}()
	info0, err := img0.Stat()
	require.Nil(t, err)

	data, err := json.Marshal(map[string]time.Time{img0.Name(): info0.ModTime()})
	require.Nil(t, err)
	image := images.Image{
		Labels: map[string]string{ImageSourcePathsLabel: string(data)},
	}

	img1, err := os.CreateTemp("", "test")
	require.Nil(t, err)
	defer func() {
		_ = img1.Close()
		_ = os.Remove(img1.Name())
	}()
	info1, err := img1.Stat()
	require.Nil(t, err)

	err = AddToImageSources(&image, img1.Name(), info1.ModTime())
	require.Nil(t, err)

	expected := ImageSources{
		img0.Name(): info0.ModTime(),
		img1.Name(): info1.ModTime(),
	}
	got, err := GetImageSources(image)
	require.Nil(t, err)
	require.True(t, expected[img0.Name()].Equal(got[img0.Name()]), "dates mismatch")
	require.True(t, expected[img1.Name()].Equal(got[img1.Name()]), "dates mismatch")

	// test if it trims the sources
	err = img0.Close()
	require.Nil(t, err)
	err = os.Remove(img0.Name())
	require.Nil(t, err)

	err = AddToImageSources(&image, img1.Name(), info1.ModTime())
	require.Nil(t, err)

	expected = ImageSources{img1.Name(): info1.ModTime()}
	got, err = GetImageSources(image)
	require.Nil(t, err)
	require.True(t, expected[img1.Name()].Equal(got[img1.Name()]), "dates mismatch")
}
