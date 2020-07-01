package applier

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/cmd/printers"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/config"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"

	applycmd "sigs.k8s.io/cli-utils/cmd/apply"

	fileutil "github.com/Mirantis/mke/pkg/util"
)

type Applier struct {
	Name string
	Dir  string

	log *logrus.Entry
}

func NewApplier(dir string) (Applier, error) {
	name := filepath.Base(dir)
	log := logrus.WithFields(logrus.Fields{
		"component": "applier",
		"bundle":    name,
	})

	a := Applier{
		log:  log,
		Dir:  dir,
		Name: name,
	}

	return a, nil
}

func (a *Applier) Run() error {
	a.log.Info("starting reconcile loop")

	kubeConfigPath := filepath.Join(constant.CertRoot, "admin.conf")

	f := util.NewFactory(&genericclioptions.ConfigFlags{
		KubeConfig: &kubeConfigPath,
	})

	readerOptions := manifestreader.ReaderOptions{
		Factory:   f,
		Namespace: "default",
	}

	reader := &manifestreader.PathManifestReader{
		Path:          a.Dir,
		ReaderOptions: readerOptions,
	}
	streams := genericclioptions.IOStreams{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// Check if we need to run init
	if !fileutil.FileExists(filepath.Join(a.Dir, "inventory-template.yaml")) {
		a.log.Info("initializing new bundle")
		// Need to run init
		io := config.NewInitOptions(streams)
		io.Complete([]string{a.Dir})
		if err := io.Run(); err != nil {
			a.log.Warnf("bundle init failed: %s", err.Error())
			return errors.Wrapf(err, "bundle init failed")
		}
	} else {
		a.log.Info("bundle already initialized")
	}

	// applier is now ready to be run in a reconcile loop fashion
	go func() {
		for {
			// TODO Maybe we need to make the command parse all the flags
			flags := []string{
				"--no-prune=false",
				a.Dir,
			}
			applyCmd := applycmd.ApplyCommand(f, streams)
			applyCmd.ParseFlags(flags)

			applier := apply.NewApplier(f, streams)

			if err := applier.Initialize(applyCmd); err != nil {
				a.log.Warnf("failed to initialize resource applier: %s", err.Error())
			}

			infos, err := reader.Read()
			if err != nil {
				a.log.Warnf("failed to read manifests: %w", err)
			} else if len(infos) > 0 {
				a.log.Debugf("found %d manifests:", len(infos))
				for _, i := range infos {
					a.log.Debugln(i.ObjectName())
				}
				ch := applier.Run(context.Background(), infos, apply.Options{
					PollInterval:           2 * time.Second,
					ReconcileTimeout:       0,
					EmitStatusEvents:       false,
					NoPrune:                false,
					DryRun:                 false,
					PrunePropagationPolicy: metav1.DeletePropagationBackground,
					PruneTimeout:           5 * time.Minute,
				})
				// TODO We probably want to implement our own printer so we can log properly based on bundle name etc.
				printer := printers.GetPrinter(printers.TablePrinter, streams)
				printer.Print(ch, false)
			}

			// TODO Would be better to have some inotify thingy to trigger this
			time.Sleep(10 * time.Second)
		}

	}()
	return nil
}

func (a *Applier) Stop() error {
	a.log.Info("stop reconcile loop")
	// TODO well, actually make the thing stop :D
	return nil
}
