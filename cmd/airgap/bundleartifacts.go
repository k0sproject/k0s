// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/k0sproject/k0s/cmd/internal"
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
		debugFlags internal.DebugFlags
		outPath    string
		platform   = platforms.DefaultSpec()
		bundler    = airgap.OCIArtifactsBundler{
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
		PersistentPreRun: debugFlags.Run,
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
				refs, err = parseArtifactRefs(args)
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

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.StringVarP(&outPath, "output", "o", "", "output file path (writes to standard output if omitted)")
	flags.Var((*insecureRegistryFlag)(&bundler.InsecureRegistries), "insecure-registries", "one of no, skip-tls-verify or plain-http")
	flags.Var((*platformFlag)(&platform), "platform", "the platform to export")
	flags.StringArrayVar(&bundler.RegistriesConfigPaths, "registries-config", nil, "paths to the authentication files for OCI registries (uses the standard Docker config if omitted)")

	return cmd
}

func parseArtifactRefsFromReader(in io.Reader) ([]reference.Named, error) {
	words := bufio.NewScanner(in)
	words.Split(bufio.ScanWords)

	var refs []string
	for words.Scan() {
		refs = append(refs, words.Text())
	}
	if err := words.Err(); err != nil {
		return nil, err
	}

	return parseArtifactRefs(refs)
}

func parseArtifactRefs(refs []string) ([]reference.Named, error) {
	var collected []reference.Named
	for _, ref := range refs {
		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, fmt.Errorf("while parsing %s: %w", ref, err)
		}
		collected = append(collected, parsed)
	}
	return collected, nil
}

type insecureRegistryFlag airgap.InsecureOCIRegistryKind

func (insecureRegistryFlag) Type() string {
	return "string"
}

func (i insecureRegistryFlag) String() string {
	switch (airgap.InsecureOCIRegistryKind)(i) {
	case airgap.NoInsecureOCIRegistry:
		return "no"
	case airgap.SkipTLSVerifyOCIRegistry:
		return "skip-tls-verify"
	case airgap.PlainHTTPOCIRegistry:
		return "plain-http"
	default:
		return strconv.Itoa(int(i))
	}
}

func (i *insecureRegistryFlag) Set(value string) error {
	var kind airgap.InsecureOCIRegistryKind

	switch value {
	case "no":
		kind = airgap.NoInsecureOCIRegistry
	case "skip-tls-verify":
		kind = airgap.SkipTLSVerifyOCIRegistry
	case "plain-http":
		kind = airgap.PlainHTTPOCIRegistry
	default:
		return errors.New("must be one of no, skip-tls-verify or plain-http")
	}

	*(*airgap.InsecureOCIRegistryKind)(i) = kind
	return nil
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
