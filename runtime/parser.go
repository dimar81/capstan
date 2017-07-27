/*
 * Copyright (C) 2015 XLAB, Ltd.
 *
 * This work is open source software, licensed under the terms of the
 * BSD license as described in the LICENSE file in the top-level directory.
 */

package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// cmdConfigInternal is used just for meta/run.yaml unmarshalling.
type cmdConfigInternal struct {
	Runtime          RuntimeType                       `yaml:"runtime"`
	ConfigSet        map[string]map[string]interface{} `yaml:"config_set"`
	ConfigSetDefault string                            `yaml:"config_set_default"`
}

// CmdConfig is a result that parsing meta/run.yaml yields.
type CmdConfig struct {
	RuntimeType      RuntimeType
	ConfigSetDefault string

	// ConfigSets is a map of available <config-name>:<runtime> pairs.
	// The map is built based on meta/run.yaml.
	ConfigSets map[string]Runtime
}

// AllCmdConfigs is a result that parsing meta/run.yamls of all required
// packages yields.
type AllCmdConfigs struct {
	cmdConfigs map[string]*CmdConfig
	order      []string
}

// PackageRunManifestGeneral parses meta/run.yaml file into blank RunConfig.
// By 'blank' we mean that the struct has no fields populated, but it is of
// correct type i.e. appropriate implementation of Runtime interface.
// NOTE: We must differentiate two things regarding Runtime interface implementation:
//    a) what struct is it implemented with -> e.g. nodeJsRuntime
//    b) what fields is struct populated with -> e.g. nodeJsRuntime.Main
//
//    For a given meta/run.yaml all config sets get the same (a), but are populated
//    with different values for (b).
// NOTE: when Capstan needs to know what packages to require, it needs (a), but
//    not (b). And this function returns exactly this, (a) without (b).
func PackageRunManifestGeneral(cmdConfigFile string) (Runtime, error) {

	// Take meta/run.yaml from the current directory if not provided.
	if cmdConfigFile == "." {
		cmdConfigFile = filepath.Join(cmdConfigFile, "meta", "run.yaml")
	}

	// Abort silently if run.yaml does not exist (since it is not required to have one)
	if _, err := os.Stat(cmdConfigFile); os.IsNotExist(err) {
		return nil, nil
	}

	// From here on, no error is suppressed since we do not tolerate corrupted run.yaml.

	// Open file.
	data, err := ioutil.ReadFile(cmdConfigFile)
	if err != nil {
		return nil, err
	}

	// Parse basic fields only (to get runtime name).
	internal := cmdConfigInternal{}
	if err := yaml.Unmarshal(data, &internal); err != nil {
		return nil, fmt.Errorf("failed to parse meta/run.yaml: %s", err)
	}

	fmt.Printf("Resolved runtime into: %s\n", internal.Runtime)

	// Return blank implementation of runtime interface.
	blankRuntime, err := PickRuntime(internal.Runtime)
	return blankRuntime, err
}

// ParsePackageRunManifestData returns parsed manifest data.
func ParsePackageRunManifestData(cmdConfigData []byte) (*CmdConfig, error) {
	res := CmdConfig{}

	// Parse basic fields.
	internal := cmdConfigInternal{}
	if err := yaml.Unmarshal(cmdConfigData, &internal); err != nil {
		return nil, fmt.Errorf("failed to parse meta/run.yaml: %s", err)
	} else {
		// Store basic fields into result struct
		res.RuntimeType = internal.Runtime
		res.ConfigSetDefault = internal.ConfigSetDefault
	}

	res.ConfigSets = make(map[string]Runtime)

	// We are marshalling the `map[interface{}]interface{}` data here (containing single
	// configuration set parameters) so that we will be able to unmarshal it in the next
	// step into the appropriate structure. This trick is used so that we do not need to
	// trouble with casting interfaces to extract config set parameters - we leave it to
	// yaml unmarshaller instead. In other words, we parse meta/run.yaml per partes:
	// config_set:
	//    name1: <map[interface{}]interface{}>  # <--- 1st part
	//    name2: <map[interface{}]interface{}>  # <--- 2nd part
	//    name3: <map[interface{}]interface{}>  # <--- 3rd part
	// Each part is unmarshalled into one interface.
	// Variable 'subdata' in the following for loop contains yaml string representing
	// single configuration set data that we then unmarshall into appropriate runtime
	// interface.
	for k := range internal.ConfigSet {
		// Prepare empty runtime struct that will be used for unmarshalling.
		theRuntime, err := PickRuntime(internal.Runtime)
		if err != nil {
			return nil, err
		}

		// Use appropriate subsection of yaml only.
		subdata, _ := yaml.Marshal(internal.ConfigSet[k])

		// Parse runtime-specific settings.
		if err := yaml.Unmarshal(subdata, theRuntime); err != nil {
			return nil, fmt.Errorf("failed to parse data for configset '%s': %s", k, err)
		}

		res.ConfigSets[k] = theRuntime
	}

	if len(res.ConfigSets) == 0 {
		return nil, fmt.Errorf("failed to parse meta/run.yaml: at least one config_set must be provided")
	}

	return &res, nil
}

// selectConfigSetByName selects appropriate config set and returns it.
func (r *CmdConfig) selectConfigSetByName(name string) (Runtime, error) {
	availableNames := fmt.Sprintf("['%s']", strings.Join(keysOfMap(r.ConfigSets), "', '"))

	// Handle unspecified configuration name.
	if name == "" && len(r.ConfigSets) == 1 {
		// If only one configuration set is provided, then there is no doubt.
		for k := range r.ConfigSets {
			return r.ConfigSets[k], nil
		}
	} else if name == "" {
		return nil, fmt.Errorf("Could not select which configuration set to run:\n"+
			"Neither --runconfig <name> is provided, nor config_set_default is set in meta/run.yaml\n"+
			"Available names: %s", availableNames)
	}

	if r.ConfigSets[name] == nil {
		return nil, fmt.Errorf("Could not select which configuration set to run:\n"+
			"Configuration set name '%s' not one of %s",
			name, availableNames)
	}

	return r.ConfigSets[name], nil
}

func (c *AllCmdConfigs) Add(pkgName string, cmdConfig *CmdConfig) {
	if c.cmdConfigs == nil {
		c.cmdConfigs = make(map[string]*CmdConfig, 15)
	}
	c.cmdConfigs[pkgName] = cmdConfig
	c.order = append(c.order, pkgName)
}

func (c *AllCmdConfigs) Persist(mpmDir string) error {
	// Prepare directory to store bootcmd files in.
	targetDir := filepath.Join(mpmDir, "run")
	if _, err := os.Stat(targetDir); err != nil {
		if err = os.MkdirAll(targetDir, 0775); err != nil {
			return err
		}
	}

	// Persist runscript scripts for all config_sets of all packages.
	for _, pkgName := range c.order {
		cmdConf := c.cmdConfigs[pkgName]
		if cmdConf == nil {
			continue
		}

		for confName := range cmdConf.ConfigSets {
			currConf := cmdConf.ConfigSets[confName]
			// Validate.
			if err := currConf.Validate(); err != nil {
				return fmt.Errorf("Validation failed for configuration set '%s': %s", confName, err)
			}

			// Calculate boot command.
			bootCmd, err := currConf.GetBootCmd(c.cmdConfigs)
			if err != nil {
				return err
			}

			// Persist to file.
			cmdFile := filepath.Join(targetDir, confName)
			if err := ioutil.WriteFile(cmdFile, []byte(bootCmd), 0775); err != nil {
				return err
			}
		}
	}

	return nil
}

// keysOfMap does nothing but returns a list of all the keys in a map.
func keysOfMap(myMap map[string]Runtime) []string {
	keys := make([]string, len(myMap))
	i := 0
	for k := range myMap {
		keys[i] = k
		i++
	}
	return keys
}
