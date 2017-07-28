/*
 * Copyright (C) 2015 XLAB, Ltd.
 *
 * This work is open source software, licensed under the terms of the
 * BSD license as described in the LICENSE file in the top-level directory.
 */

package runtime

import (
	"fmt"
	"strings"
)

type javaRuntime struct {
	CommonRuntime `yaml:"-,inline"`
	Xms           string   `yaml:"xms"`
	Xmx           string   `yaml:"xmx"`
	Classpath     []string `yaml:"classpath"`
	JvmArgs       []string `yaml:"jvm_args"`
	Main          string   `yaml:"main"`
	Args          []string `yaml:"args"`
}

//
// Interface implementation
//

func (conf javaRuntime) GetRuntimeName() string {
	return string(Java)
}
func (conf javaRuntime) GetRuntimeDescription() string {
	return "Run Java application"
}
func (conf javaRuntime) GetDependencies() []string {
	return []string{"openjdk8-zulu-compact1"}
}
func (conf javaRuntime) Validate() error {
	inherit := conf.Base != ""

	if !inherit {
		if conf.Main == "" {
			return fmt.Errorf("'main' must be provided")
		}

		if conf.Classpath == nil {
			return fmt.Errorf("'classpath' must be provided")
		}
	}

	return conf.CommonRuntime.Validate(inherit)
}
func (conf javaRuntime) GetBootCmd(cmdConfs map[string]*CmdConfig) (string, error) {
	conf.Base = "openjdk8-zulu-compact1:java"
	conf.setDefaultEnv(map[string]string{
		"XMS":       conf.Xms,
		"XMX":       conf.Xmx,
		"CLASSPATH": strings.Join(conf.Classpath, ":"),
		"JVM_ARGS":  conf.concatJvmArgs(),
		"MAIN":      conf.Main,
		"ARGS":      strings.Join(conf.Args, " "),
	})
	return conf.CommonRuntime.BuildBootCmd("", cmdConfs)
}
func (conf javaRuntime) GetYamlTemplate() string {
	return `
# REQUIRED
# Fully classified name of the main class.
# Example value: main.Hello
main: <name>

# REQUIRED
# A list of paths where classes and other resources can be found.
# Example value: classpath:
#                   - /
#                   - /package1
classpath:
   - <list>

# OPTIONAL
# Initial and maximum JVM memory size.
# Example value: xms: 512m
xms: <value>
xmx: <value>

# OPTIONAL
# A list of JVM args.
# Example value: jvm_args:
#                   - -Djava.net.preferIPv4Stack=true
#                   - -Dhadoop.log.dir=/hdfs/logs
jvm_args:
   - <list>

# OPTIONAL
# A list of command line args used by the application.
# Example value: args:
#                   - argument1
#                   - argument2
args:
   - <list>
` + conf.CommonRuntime.GetYamlTemplate()
}

//
// Utility
//

func (conf javaRuntime) concatJvmArgs() string {
	res := strings.Join(conf.JvmArgs, " ")

	// This is a workaround since runscript is currently unable to
	// handle empty environment variable as a parameter. So we set
	// dummy value unless user provided some actual value.
	if res == "" {
		return "-Dx=y"
	}
	return res
}
