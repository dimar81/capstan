/*
 * Copyright (C) 2015 XLAB, Ltd.
 *
 * This work is open source software, licensed under the terms of the
 * BSD license as described in the LICENSE file in the top-level directory.
 */

package runtime

import "fmt"

type nativeRuntime struct {
	CommonRuntime `yaml:"-,inline"`
	BootCmd       string `yaml:"bootcmd"`
}

//
// Interface implementation
//

func (conf nativeRuntime) GetRuntimeName() string {
	return string(Native)
}
func (conf nativeRuntime) GetRuntimeDescription() string {
	return "Run arbitrary command inside OSv"
}
func (conf nativeRuntime) GetDependencies() []string {
	return []string{}
}
func (conf nativeRuntime) Validate() error {
	inherit := conf.Base != ""

	if !inherit {
		if conf.BootCmd == "" {
			return fmt.Errorf("'bootcmd' must be provided")
		}
	}

	return conf.CommonRuntime.Validate(inherit)
}
func (conf nativeRuntime) GetBootCmd(cmdConfs map[string]*CmdConfig) (string, error) {
	cmd := conf.BootCmd
	return conf.CommonRuntime.BuildBootCmd(cmd, cmdConfs)
}
func (conf nativeRuntime) GetYamlTemplate() string {
	return `
# REQUIRED
# Command to be executed in OSv.
# Note that package root will correspond to filesystem root (/) in OSv image.
# Example value: /usr/bin/simpleFoam.so -help
bootcmd: <command>
`
}
