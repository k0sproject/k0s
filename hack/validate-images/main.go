package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	var architectures []string
	var architecturesString string
	flag.StringVar(&architecturesString, "architectures", "amd64,arm64", "which architectures to search for")
	flag.Parse()
	architectures = strings.Split(architecturesString, ",")
	if len(architectures) < 1 {
		panic("No architectures given")
	}
	cfg := v1beta1.DefaultClusterConfig()
	uris := airgap.GetImageURIs(cfg.Spec.Images)
	if err := validateImages(uris, architectures); err != nil {
		os.Exit(1)
	}

}

func validateImages(uris []string, architectures []string) error {
	var errs []error
	for _, name := range uris {
		fmt.Println("validating image", name, "to have architectures: ", architectures)
		imageRef, err := util.ParseName(name)
		check(err)
		memoryStore := store.NewMemoryStore()
		resolver := util.NewResolver("", "", true, true)
		descriptor, err := registry.FetchDescriptor(resolver, memoryStore, imageRef)
		check(err)
		_, db, _ := memoryStore.Get(descriptor)
		switch descriptor.MediaType {
		case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
			// this is a multi-platform image descriptor; marshal to Index type
			var idx ocispec.Index
			check(json.Unmarshal(db, &idx))
			if err := validateList(name, architectures, memoryStore, descriptor, idx); err != nil {
				errs = append(errs, err)
			}
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			errs = append(errs, fmt.Errorf("image %s has single manifest, but we need multiarch manifests", name))
		default:
			errs = append(errs, fmt.Errorf("image %s has unknown manifest type, can't validate architectures", name))
		}
	}
	if len(errs) > 0 {
		spew.Dump(errs)
		return fmt.Errorf("image manifests have wrong architectures")
	}
	return nil
}

func validateList(name string, architectures []string, cs *store.MemoryStore, descriptor ocispec.Descriptor, index ocispec.Index) error {
	searchFor := map[string]bool{}

	for _, m := range index.Manifests {
		for _, platformToCheck := range architectures {
			if m.Platform.Architecture == platformToCheck {
				searchFor[platformToCheck] = true
			}
		}
	}

	for _, platform := range architectures {
		_, found := searchFor[platform]
		if !found {
			return fmt.Errorf("platform %s not found for image %s", platform, name)
		}
	}
	return nil
}
