/***** BEGIN LICENSE BLOCK *****
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this file,
# You can obtain one at http://mozilla.org/MPL/2.0/.
#
# The Initial Developer of the Original Code is the Mozilla Foundation.
# Portions created by the Initial Developer are Copyright (C) 2014
# the Initial Developer. All Rights Reserved.
#
# Contributor(s):
#   Anton Lindström (carlantonlindstrom@gmail.com)
#   Rob Miller (rmiller@mozilla.com)
#   Karl Matthias (karl.matthias@gonitro.com)
#
# ***** END LICENSE BLOCK *****/

package docker

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mozilla-services/heka/pipeline"
)

type DockerLogInputConfig struct {
	// A Docker endpoint.
	Endpoint                string   `toml:"endpoint"`
	CertPath                string   `toml:"cert_path"`
	SincePath               string   `toml:"since_path"`
	SinceInterval           string   `toml:"since_interval"`
	NameFromEnv             string   `toml:"name_from_env_var"`
	FieldsFromEnv           []string `toml:"fields_from_env"`
	FieldsFromLabels        []string `toml:"fields_from_labels"`
	ContainerExpiryDays     int      `toml:"container_expiry_days"`
	NewContainersReplayLogs bool     `toml:"new_containers_replay_logs"`
}

type DockerLogInput struct {
	stopChan  chan error
	closer    chan struct{}
	attachMgr *AttachManager
	pConfig   *pipeline.PipelineConfig
}

func (di *DockerLogInput) SetPipelineConfig(pConfig *pipeline.PipelineConfig) {
	di.pConfig = pConfig
}

func (di *DockerLogInput) ConfigStruct() interface{} {
	return &DockerLogInputConfig{
		Endpoint:                "unix:///var/run/docker.sock",
		CertPath:                "",
		SincePath:               filepath.Join("docker", "logs_since.txt"),
		SinceInterval:           "5s",
		ContainerExpiryDays:     30,
		NewContainersReplayLogs: true,
	}
}

func (di *DockerLogInput) Init(config interface{}) error {
	conf := config.(*DockerLogInputConfig)
	globals := di.pConfig.Globals
	sincePath := globals.PrependBaseDir(conf.SincePath)

	// Make sure since interval is valid.
	sinceInterval, err := time.ParseDuration(conf.SinceInterval)
	if err != nil {
		return fmt.Errorf("Can't parse since_interval value '%s': %s", conf.SinceInterval,
			err.Error())
	}

	// Make sure we have a sinces File.
	err = EnsureSincesFile(conf, sincePath)
	if err != nil {
		return err
	}

	di.stopChan = make(chan error)
	di.closer = make(chan struct{})

	m, err := NewAttachManager(
		conf.Endpoint,
		conf.CertPath,
		conf.NameFromEnv,
		conf.FieldsFromEnv,
		conf.FieldsFromLabels,
		sincePath,
		sinceInterval,
		conf.ContainerExpiryDays,
		conf.NewContainersReplayLogs,
	)
	if err != nil {
		return fmt.Errorf("DockerLogInput: failed to attach: %s", err.Error())
	}

	di.attachMgr = m
	return nil
}

func (di *DockerLogInput) Run(ir pipeline.InputRunner, h pipeline.PluginHelper) error {
	return di.attachMgr.Run(ir, h.Hostname(), di.stopChan)
}

func (di *DockerLogInput) CleanupForRestart() {
	// Intentially left empty. Cleanup happens in Run()
}

func (di *DockerLogInput) Stop() {
	close(di.stopChan)
}

func init() {
	pipeline.RegisterPlugin("DockerLogInput", func() interface{} {
		return new(DockerLogInput)
	})
}
