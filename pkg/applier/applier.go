package applier

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/cmd/printers"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/config"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"

	"github.com/Mirantis/mke/pkg/constant"
	fileutil "github.com/Mirantis/mke/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycmd "sigs.k8s.io/cli-utils/cmd/apply"
)

type Applier struct {
	Name string
	Dir  string

	log           *logrus.Entry
	watcher       *fsnotify.Watcher
	reader        *manifestreader.PathManifestReader
	streams       genericclioptions.IOStreams
	clientFactory util.Factory
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

	err := a.init()
	if err != nil {
		return a, err
	}

	return a, nil
}

func (a *Applier) init() error {
	kubeConfigPath := filepath.Join(constant.CertRoot, "admin.conf")

	a.clientFactory = util.NewFactory(&genericclioptions.ConfigFlags{
		KubeConfig: &kubeConfigPath,
	})

	readerOptions := manifestreader.ReaderOptions{
		Factory:   a.clientFactory,
		Namespace: "default",
	}

	a.reader = &manifestreader.PathManifestReader{
		Path:          a.Dir,
		ReaderOptions: readerOptions,
	}
	a.streams = genericclioptions.IOStreams{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// Check if we need to run init
	if !fileutil.FileExists(filepath.Join(a.Dir, "inventory-template.yaml")) {
		a.log.Info("initializing manifest bundle")
		// Need to run init
		io := config.NewInitOptions(a.streams)
		io.InventoryID = "mke-manifests"
		io.Complete([]string{a.Dir})
		if err := io.Run(); err != nil {
			a.log.Warnf("bundle init failed: %s", err.Error())
			return errors.Wrapf(err, "bundle init failed")
		}
	} else {
		a.log.Info("manifest bundle already initialized")
	}
	return nil
}

func (a *Applier) Apply() error {
	// TODO Maybe we need to make the command parse all the flags
	flags := []string{
		"--no-prune=false",
		a.Dir,
	}
	applyCmd := applycmd.ApplyCommand(a.clientFactory, a.streams)
	applyCmd.ParseFlags(flags)

	applier := apply.NewApplier(a.clientFactory, a.streams)

	if err := applier.Initialize(applyCmd); err != nil {
		a.log.Warnf("failed to initialize resource applier: %s", err.Error())
	}

	infos, err := a.reader.Read()
	if err != nil {
		return errors.Wrap(err, "failed to read manifests")
	}
	if len(infos) > 0 {
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
		printer := printers.GetPrinter(printers.TablePrinter, a.streams)
		printer.Print(ch, false)
	}

	return nil
}
