/*
Copyright 2022 k0s authors

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

package sysinfo

import (
	"errors"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/logrusorgru/aurora/v3"
	"github.com/stretchr/testify/assert"
)

func TestCliReporter_Pass(t *testing.T) {
	for _, data := range []struct {
		name  string
		desc  probes.ProbeDesc
		prop  probes.ProbedProp
		xpect string
	}{
		{
			"prints_nil",
			&testDesc{"", nil}, nil,
			"\x1b[97m: \x1b[0m\x1b[32m\x1b[0m(pass)\n",
		},
		{
			"prints_empty",
			&testDesc{"", nil}, testProp(""),
			"\x1b[97m: \x1b[0m\x1b[32m\x1b[0m(pass)\n",
		},
		{
			"prints_value",
			&testDesc{"", probes.ProbePath{"foo"}}, testProp("bar"),
			"\x1b[97m: \x1b[0m\x1b[32mbar\x1b[0m (pass)\n",
		},
		{
			"no_indent_for_single_path",
			&testDesc{"foo", probes.ProbePath{"bar"}}, testProp("baz"),
			"\x1b[97mfoo: \x1b[0m\x1b[32mbaz\x1b[0m (pass)\n",
		},
		{
			"indent_for_nested_path",
			&testDesc{"foo", probes.ProbePath{"bar", "baz"}}, testProp("qux"),
			"  \x1b[97mfoo: \x1b[0m\x1b[32mqux\x1b[0m (pass)\n",
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			underTest := &cliReporter{
				colors: aurora.NewAurora(true),
			}
			err := underTest.Pass(data.desc, data.prop)
			assert.NoError(t, err)
			assert.False(t, underTest.failed)
			var buf strings.Builder
			err = underTest.printHuman(&buf)
			assert.NoError(t, err)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Warn(t *testing.T) {
	for _, data := range []struct {
		name  string
		desc  probes.ProbeDesc
		prop  probes.ProbedProp
		msg   string
		xpect string
	}{
		{
			"prints_nil",
			&testDesc{"", nil}, nil, "",
			"\x1b[97m: \x1b[0m\x1b[33m\x1b[0m(warning)\n",
		},
		{
			"prints_empty",
			&testDesc{"", nil}, testProp(""), "",
			"\x1b[97m: \x1b[0m\x1b[33m\x1b[0m(warning)\n",
		},
		{
			"prints_prop",
			&testDesc{"foo", nil}, testProp("bar"), "baz",
			"\x1b[97mfoo: \x1b[0m\x1b[33mbar\x1b[0m (warning: baz)\n",
		},
		{
			"prints_msg",
			&testDesc{"foo", nil}, nil, "bar",
			"\x1b[97mfoo: \x1b[0m\x1b[33m\x1b[0m(warning: bar)\n",
		},
		{
			"prints_value_and_msg",
			&testDesc{"foo", nil}, testProp("bar"), "baz",
			"\x1b[97mfoo: \x1b[0m\x1b[33mbar\x1b[0m (warning: baz)\n",
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			underTest := &cliReporter{
				colors: aurora.NewAurora(true),
			}
			err := underTest.Warn(data.desc, data.prop, data.msg)
			assert.NoError(t, err)
			assert.False(t, underTest.failed)
			var buf strings.Builder
			err = underTest.printHuman(&buf)
			assert.NoError(t, err)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Reject(t *testing.T) {
	for _, data := range []struct {
		name  string
		desc  probes.ProbeDesc
		prop  probes.ProbedProp
		msg   string
		xpect string
	}{
		{
			"prints_nil",
			&testDesc{"", nil}, nil, "",
			"\x1b[97m: \x1b[0m\x1b[1;31m\x1b[0m(rejected)\n",
		},
		{
			"prints_empty",
			&testDesc{"", nil}, testProp(""), "",
			"\x1b[97m: \x1b[0m\x1b[1;31m\x1b[0m(rejected)\n",
		},
		{
			"prints_prop",
			&testDesc{"foo", nil}, testProp("bar"), "baz",
			"\x1b[97mfoo: \x1b[0m\x1b[1;31mbar\x1b[0m (rejected: baz)\n",
		},
		{
			"prints_msg",
			&testDesc{"foo", nil}, nil, "bar",
			"\x1b[97mfoo: \x1b[0m\x1b[1;31m\x1b[0m(rejected: bar)\n",
		},
		{
			"prints_value_and_msg",
			&testDesc{"foo", nil}, testProp("bar"), "baz",
			"\x1b[97mfoo: \x1b[0m\x1b[1;31mbar\x1b[0m (rejected: baz)\n",
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			underTest := &cliReporter{
				colors: aurora.NewAurora(true),
			}
			err := underTest.Reject(data.desc, data.prop, data.msg)
			assert.NoError(t, err)
			assert.True(t, underTest.failed)
			var buf strings.Builder
			err = underTest.printHuman(&buf)
			assert.NoError(t, err)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Error(t *testing.T) {
	for _, data := range []struct {
		name  string
		desc  probes.ProbeDesc
		err   error
		xpect string
	}{
		{
			"prints_nil",
			&testDesc{"", nil}, nil,
			"\x1b[97m: \x1b[0m\x1b[1;31merror\x1b[0m\n",
		},
		{
			"prints_empty",
			&testDesc{"", nil}, errors.New(""),
			"\x1b[97m: \x1b[0m\x1b[1;31merror\x1b[0m\n",
		},
		{
			"prints_err",
			&testDesc{"foo", probes.ProbePath{}}, errors.New("bar"),
			"\x1b[97mfoo: \x1b[0m\x1b[1;31merror: bar\x1b[0m\n",
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			underTest := &cliReporter{
				colors: aurora.NewAurora(true),
			}
			err := underTest.Error(data.desc, data.err)
			assert.NoError(t, err)
			assert.True(t, underTest.failed)
			var buf strings.Builder
			err = underTest.printHuman(&buf)
			assert.NoError(t, err)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter(t *testing.T) {
	for _, data := range []struct {
		name         string
		probe        func(t *testing.T, cli *cliReporter)
		xpectResults []Probe
		xpect        string
		xpectFailed  bool
	}{
		{
			"success",
			func(t *testing.T, cli *cliReporter) {
				err := cli.Pass(&testDesc{"foo", probes.ProbePath{"bar"}}, testProp("baz"))
				assert.NoError(t, err)
				err = cli.Pass(&testDesc{"foo", probes.ProbePath{"bar", "baz"}}, testProp("qux"))
				assert.NoError(t, err)
				err = cli.Warn(&testDesc{"foo", nil}, testProp("bar"), "baz")
				assert.NoError(t, err)
			},
			[]Probe{
				{Path: []string{"bar"}, DisplayName: "foo", Prop: "baz", Message: "", Category: "pass", Error: nil},
				{Path: []string{"bar", "baz"}, DisplayName: "foo", Prop: "qux", Message: "", Category: "pass", Error: nil},
				{Path: []string(nil), DisplayName: "foo", Prop: "bar", Message: "baz", Category: "warning", Error: nil},
			},
			("\x1b[97mfoo: \x1b[0m\x1b[32mbaz\x1b[0m (pass)\n" +
				"  \x1b[97mfoo: \x1b[0m\x1b[32mqux\x1b[0m (pass)\n" +
				"\x1b[97mfoo: \x1b[0m\x1b[33mbar\x1b[0m (warning: baz)\n"),
			true,
		},
		{
			"has_reject",
			func(t *testing.T, cli *cliReporter) {
				err := cli.Pass(&testDesc{"foo", probes.ProbePath{"bar"}}, testProp("baz"))
				assert.NoError(t, err)
				err = cli.Pass(&testDesc{"foo", probes.ProbePath{"bar", "baz"}}, testProp("qux"))
				assert.NoError(t, err)
				err = cli.Warn(&testDesc{"foo", nil}, testProp("bar"), "baz")
				assert.NoError(t, err)
				err = cli.Reject(&testDesc{"foo", nil}, testProp("bar"), "baz")
				assert.NoError(t, err)
			},
			[]Probe{
				{Path: []string{"bar"}, DisplayName: "foo", Prop: "baz", Message: "", Category: "pass", Error: nil},
				{Path: []string{"bar", "baz"}, DisplayName: "foo", Prop: "qux", Message: "", Category: "pass", Error: nil},
				{Path: []string(nil), DisplayName: "foo", Prop: "bar", Message: "baz", Category: "warning", Error: nil},
				{Path: []string(nil), DisplayName: "foo", Prop: "bar", Message: "baz", Category: "rejected", Error: nil},
			},
			("\x1b[97mfoo: \x1b[0m\x1b[32mbaz\x1b[0m (pass)\n" +
				"  \x1b[97mfoo: \x1b[0m\x1b[32mqux\x1b[0m (pass)\n" +
				"\x1b[97mfoo: \x1b[0m\x1b[33mbar\x1b[0m (warning: baz)\n" +
				"\x1b[97mfoo: \x1b[0m\x1b[1;31mbar\x1b[0m (rejected: baz)\n"),
			true,
		},
		{
			"has_error",
			func(t *testing.T, cli *cliReporter) {
				err := cli.Pass(&testDesc{"foo", probes.ProbePath{"bar"}}, testProp("baz"))
				assert.NoError(t, err)
				err = cli.Pass(&testDesc{"foo", probes.ProbePath{"bar", "baz"}}, testProp("qux"))
				assert.NoError(t, err)
				err = cli.Warn(&testDesc{"foo", nil}, testProp("bar"), "baz")
				assert.NoError(t, err)
				err = cli.Error(&testDesc{"foo", probes.ProbePath{}}, errors.New("bar"))
				assert.NoError(t, err)
			},
			[]Probe{
				{Path: []string{"bar"}, DisplayName: "foo", Prop: "baz", Message: "", Category: "pass", Error: nil},
				{Path: []string{"bar", "baz"}, DisplayName: "foo", Prop: "qux", Message: "", Category: "pass", Error: nil},
				{Path: []string(nil), DisplayName: "foo", Prop: "bar", Message: "baz", Category: "warning", Error: nil},
				{Path: []string(nil), DisplayName: "foo", Prop: "", Message: "", Category: "error", Error: errors.New("bar")},
			},
			("\x1b[97mfoo: \x1b[0m\x1b[32mbaz\x1b[0m (pass)\n" +
				"  \x1b[97mfoo: \x1b[0m\x1b[32mqux\x1b[0m (pass)\n" +
				"\x1b[97mfoo: \x1b[0m\x1b[33mbar\x1b[0m (warning: baz)\n" +
				"\x1b[97mfoo: \x1b[0m\x1b[1;31merror: bar\x1b[0m\n"),
			true,
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			underTest := &cliReporter{
				colors: aurora.NewAurora(true),
			}
			data.probe(t, underTest)
			assert.Equal(t, data.xpectResults, underTest.results)
			var buf strings.Builder
			err := underTest.printResults(&buf, "human")
			assert.NoError(t, err)
			result := buf.String()
			assert.Equal(t, data.xpect, result)
		})
	}
}

type testDesc struct {
	name string
	path probes.ProbePath
}

func (d *testDesc) Path() probes.ProbePath { return d.path }
func (d *testDesc) DisplayName() string    { return d.name }

type testProp string

func (p testProp) String() string { return string(p) }
