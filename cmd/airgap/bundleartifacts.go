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

package airgap

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/airgap"

	"k8s.io/kubectl/pkg/util/term"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newAirgapBundleArtifactsCmd(log logrus.FieldLogger, rewriteBundleRef airgap.RewriteRefFunc) *cobra.Command {
	var (
		outPath  string
		platform = platforms.DefaultSpec()
		bundler  = airgap.OCIArtifactsBundler{
			Log:           log,
			RewriteTarget: rewriteBundleRef,
		}
	)

	cmd := &cobra.Command{
		Use:   "bundle-artifacts [flags] [names...]",
		Short: "Bundles artifacts needed for airgapped installations into a tarball",
		Long: `Bundles artifacts needed for airgapped installations into a tarball. Fetches the
artifacts from their OCI registries and bundles them into an OCI Image Layout
archive (written to standard output by default). Reads names from standard input
if no names are given on the command line.`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			cmd.SilenceUsage = true

			bundler.PlatformMatcher = platforms.Only(platform)

			var out io.Writer
			if outPath == "" {
				out = cmd.OutOrStdout()
				if term.IsTerminal(out) {
					return errors.New("cowardly refusing to write binary data to a terminal")
				}
			} else {
				f, openErr := file.AtomicWithTarget(outPath).Open()
				if openErr != nil {
					return openErr
				}
				defer func() {
					if err == nil {
						err = f.Finish()
					} else if closeErr := f.Close(); closeErr != nil {
						err = errors.Join(err, closeErr)
					}
				}()
				out = f
			}

			var refs []reference.Named
			if len(args) > 0 {
				refs, err = parseArtifactRefsFromSeq(slices.Values(args))
			} else {
				refs, err = parseArtifactRefsFromReader(cmd.InOrStdin())
			}
			if err != nil {
				return err
			}

			buffered := bufio.NewWriter(out)
			if err := bundler.Run(ctx, refs, out); err != nil {
				return err
			}
			return buffered.Flush()
		},
	}

	cmd.Flags().StringVarP(&outPath, "output", "o", "", "output file path (writes to standard output if omitted)")
	cmd.Flags().Var((*insecureRegistryFlag)(&bundler.InsecureRegistries), "insecure-registries", "one of "+strings.Join(insecureRegistryFlagValues[:], ", "))
	cmd.Flags().Var((*platformFlag)(&platform), "platform", "the platform to export")
	cmd.Flags().StringArrayVar(&bundler.RegistriesConfigPaths, "registries-config", nil, "paths to the authentication files for OCI registries (uses the standard Docker config if omitted)")

	return cmd
}

func parseArtifactRefsFromReader(in io.Reader) ([]reference.Named, error) {
	words := bufio.NewScanner(in)
	words.Split(bufio.ScanWords)
	refs, err := parseArtifactRefsFromSeq(func(yield func(string) bool) {
		for words.Scan() {
			if !yield(words.Text()) {
				return
			}
		}
	})
	if err := errors.Join(err, words.Err()); err != nil {
		return nil, err
	}

	return refs, nil
}

func parseArtifactRefsFromSeq(refs iter.Seq[string]) (collected []reference.Named, _ error) {
	for ref := range refs {
		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, fmt.Errorf("while parsing %s: %w", ref, err)
		}
		collected = append(collected, parsed)
	}
	return collected, nil
}

type insecureRegistryFlag airgap.InsecureOCIRegistryKind

var insecureRegistryFlagValues = [...]string{
	airgap.NoInsecureOCIRegistry:    "no",
	airgap.SkipTLSVerifyOCIRegistry: "skip-tls-verify",
	airgap.PlainHTTPOCIRegistry:     "plain-http",
}

func (insecureRegistryFlag) Type() string {
	return "string"
}

func (i insecureRegistryFlag) String() string {
	if i := int(i); i < len(insecureRegistryFlagValues) {
		return insecureRegistryFlagValues[i]
	} else {
		return strconv.Itoa(i)
	}
}

func (i *insecureRegistryFlag) Set(value string) error {
	idx := slices.Index(insecureRegistryFlagValues[:], value)
	if idx >= 0 {
		*(*airgap.InsecureOCIRegistryKind)(i) = airgap.InsecureOCIRegistryKind(idx)
		return nil
	}

	return errors.New("must be one of " + strings.Join(insecureRegistryFlagValues[:], ", "))
}

type platformFlag imagespecv1.Platform

func (p *platformFlag) Type() string {
	return "string"
}

func (p *platformFlag) String() string {
	return platforms.FormatAll(*(*imagespecv1.Platform)(p))
}

func (p *platformFlag) Set(value string) error {
	platform, err := platforms.Parse(value)
	if err != nil {
		return err
	}
	*(*imagespecv1.Platform)(p) = platform
	return nil
}
