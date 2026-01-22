// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/stretchr/testify/require"
)

func TestGetImageSources(t *testing.T) {
	// test image without label
	got, err := GetImageSources(images.Image{})
	require.NoError(t, err)
	require.Equal(t, ImageSources{}, got)

	// test image with label
	when := time.Now().Truncate(time.Hour)
	value := map[string]time.Time{"path": when}
	data, err := json.Marshal(value)
	require.NoError(t, err)

	image := images.Image{
		Labels: map[string]string{
			ImageSourcePathsLabel: string(data),
		},
	}
	expected := ImageSources{"path": when}
	got, err = GetImageSources(image)
	require.NoError(t, err)
	require.True(t, expected["path"].Equal(got["path"]), "dates mismatch")

	// test image with invalid label
	image = images.Image{
		Labels: map[string]string{
			ImageSourcePathsLabel: "invalid",
		},
	}
	_, err = GetImageSources(image)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal label")
}

func TestSetImageSources(t *testing.T) {
	// test adding empty sources
	image := images.Image{}
	err := SetImageSources(&image, ImageSources{})
	require.NoError(t, err)
	require.Empty(t, image.Labels)

	// test setting one source
	test := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.WriteFile(test, nil, 0644))
	info, err := os.Stat(test)
	require.NoError(t, err)
	image = images.Image{}
	expected := ImageSources{test: info.ModTime()}
	err = SetImageSources(&image, expected)
	require.NoError(t, err)
	got, err := GetImageSources(image)
	require.NoError(t, err)
	require.True(t, expected[test].Equal(got[test]), "dates mismatch")

	// test sources replacement
	img0 := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.WriteFile(img0, nil, 0644))
	info0, err := os.Stat(img0)
	require.NoError(t, err)

	data, err := json.Marshal(map[string]time.Time{img0: info0.ModTime()})
	require.NoError(t, err)
	image = images.Image{
		Labels: map[string]string{ImageSourcePathsLabel: string(data)},
	}

	img1 := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.WriteFile(img1, nil, 0644))
	info1, err := os.Stat(img1)
	require.NoError(t, err)

	newsrc := ImageSources{img1: info1.ModTime()}
	err = SetImageSources(&image, newsrc)
	require.NoError(t, err)

	expected = ImageSources{img1: info1.ModTime()}
	got, err = GetImageSources(image)
	require.NoError(t, err)
	require.True(t, expected[img1].Equal(got[img1]), "dates mismatch")
}

func TestAddToImageSources(t *testing.T) {
	// test replacing sources
	img0 := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.WriteFile(img0, nil, 0644))
	info0, err := os.Stat(img0)
	require.NoError(t, err)

	data, err := json.Marshal(map[string]time.Time{img0: info0.ModTime()})
	require.NoError(t, err)
	image := images.Image{
		Labels: map[string]string{ImageSourcePathsLabel: string(data)},
	}

	img1 := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.WriteFile(img1, nil, 0644))
	info1, err := os.Stat(img0)
	require.NoError(t, err)

	err = AddToImageSources(&image, img1, info1.ModTime())
	require.NoError(t, err)

	expected := ImageSources{
		img0: info0.ModTime(),
		img1: info1.ModTime(),
	}
	got, err := GetImageSources(image)
	require.NoError(t, err)
	require.True(t, expected[img0].Equal(got[img0]), "dates mismatch")
	require.True(t, expected[img1].Equal(got[img1]), "dates mismatch")

	// test if it trims the sources
	err = os.Remove(img0)
	require.NoError(t, err)

	err = AddToImageSources(&image, img1, info1.ModTime())
	require.NoError(t, err)

	expected = ImageSources{img1: info1.ModTime()}
	got, err = GetImageSources(image)
	require.NoError(t, err)
	require.True(t, expected[img1].Equal(got[img1]), "dates mismatch")
}
