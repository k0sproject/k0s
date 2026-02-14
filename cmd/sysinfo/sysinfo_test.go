// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	var buf strings.Builder

	underTest := &cliReporter{
		w:      &buf,
		colors: aurora.NewAurora(true),
	}

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
			buf.Reset()
			err := underTest.Pass(data.desc, data.prop)
			assert.NoError(t, err)
			assert.False(t, underTest.failed)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Warn(t *testing.T) {
	var buf strings.Builder

	underTest := &cliReporter{
		w:      &buf,
		colors: aurora.NewAurora(true),
	}

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
			buf.Reset()
			err := underTest.Warn(data.desc, data.prop, data.msg)
			assert.NoError(t, err)
			assert.False(t, underTest.failed)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Reject(t *testing.T) {
	var buf strings.Builder

	underTest := &cliReporter{
		w:      &buf,
		colors: aurora.NewAurora(true),
	}

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
			buf.Reset()
			err := underTest.Reject(data.desc, data.prop, data.msg)
			assert.NoError(t, err)
			assert.True(t, underTest.failed)
			result := buf.String()
			t.Log(result)
			assert.Equal(t, data.xpect, result)
		})
	}
}

func TestCliReporter_Error(t *testing.T) {
	var buf strings.Builder

	underTest := &cliReporter{
		w:      &buf,
		colors: aurora.NewAurora(true),
	}

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
			buf.Reset()
			err := underTest.Error(data.desc, data.err)
			assert.NoError(t, err)
			assert.True(t, underTest.failed)
			result := buf.String()
			t.Log(result)
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

func Test_resultsCollector(t *testing.T) {
	for _, data := range []struct {
		name         string
		probe        func(t *testing.T, cli *resultsCollector)
		xpectResults []Probe
		xpectFailed  bool
	}{
		{
			"success",
			func(t *testing.T, cli *resultsCollector) {
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
			false,
		},
		{
			"has_reject",
			func(t *testing.T, cli *resultsCollector) {
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
			true,
		},
		{
			"has_error",
			func(t *testing.T, cli *resultsCollector) {
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
			true,
		},
	} {
		t.Run(data.name, func(t *testing.T) {
			c := &resultsCollector{}
			data.probe(t, c)
			assert.Equal(t, data.xpectResults, c.results)
			assert.Equal(t, data.xpectFailed, c.failed)
		})
	}
}
