// Copyright © 2017 Maciej Raciborski <maciej.raciborski@sbb.ch>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"encoding/json"
	"runtime"

	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/oscp/openshift-monitoring-checks/checks"
	"strings"
	"path"
)

var pretty bool
var debug bool

var log = logging.MustGetLogger("openshift-monitoring-cli")

// the data type for single shot events
type EventData map[string]interface{}

// defines the format of the output JSON integrations will return
type IntegrationData struct {
	Name               string      `json:"name"`
	ProtocolVersion    string      `json:"protocol_version"`
	IntegrationVersion string      `json:"integration_version"`
	Events             []EventData `json:"events"`
}

var rootCmd = &cobra.Command{
	Use:   "openshift-monitoring-cli",
	Short: "This cli tool runs monitoring checks for OpenShift installations.",
	Long: `This application will run major and minor checks for different node types like
for example worker node, master or storage node.`,
	Run: runChecks,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Critical(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initLogging)

	rootCmd.Flags().BoolVarP(&pretty, "pretty", "p", false, "print pretty json output")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "print debug messages")
}

func initLogging() {
	var format = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} - %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	stdOutBackend := logging.NewLogBackend(os.Stdout, "", 0)
	logging.SetBackend(logging.NewBackendFormatter(stdOutBackend, format))

	if runtime.GOOS != "windows" {
		sysLogBackend, err := logging.NewSyslogBackend("openshift-monitoring-cli")

		if err != nil {
			log.Warning("Wasn't able to initialize syslog.", err)
		} else {
			if debug {
				sysLogFormatter := logging.NewBackendFormatter(sysLogBackend, format)
				stdOutFormatter := logging.NewBackendFormatter(stdOutBackend, format)
				logging.SetBackend(logging.MultiLogger(sysLogFormatter, stdOutFormatter))
			} else {
				logging.SetBackend(logging.NewBackendFormatter(sysLogBackend, format))
			}
		}
	}

	if viper.GetString("logging.level") == "debug" {
		logging.SetLevel(logging.DEBUG, "openshift-monitoring-cli")
	} else {
		logging.SetLevel(logging.INFO, "openshift-monitoring-cli")
	}

}

func initConfig() {
	_, filename, _, _ := runtime.Caller(0)

	viper.AddConfigPath(path.Dir(filename)+"/..")
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		log.Error("Not able to read config file (path of script is ", path.Dir(filename)+")", viper.ConfigFileUsed()+".")
	}

}

func createEvent(err error) map[string]interface{} {
	var event = map[string]interface{}{}
	event["summary"] = err.Error()
	return event
}

func createMajorEvent(err error) map[string]interface{} {
	var event = createEvent(err)
	event["category"] = "MAJOR"
	log.Error("MAJOR:", err.Error())
	return event
}

func createMinorEvent(err error) map[string]interface{} {
	var event = createEvent(err)
	event["category"] = "MINOR"
	log.Error("MINOR:", err.Error())
	return event
}

func runChecks(cmd *cobra.Command, args []string) {
	log.Info("Running", viper.GetString("node.type"), "checks for OpenShift.")

	var data = IntegrationData{
		Name:               "ch.sbb.openshift-integration",
		ProtocolVersion:    "1",
		IntegrationVersion: "1.0.0",
		Events:             make([]EventData, 0),
	}

	if len(viper.GetString("etcd.ips")) == 0 || len(viper.GetString("registry.ip")) == 0 || len(viper.GetString("router.ips")) == 0 {
		log.Fatal("Can't read service IPs from configuration file.")
	}

	/////////////////
	//// MAJORS ////
	////////////////
	log.Debug("Running major checks.")

	// majors on storage
	if viper.GetString("node.type") == "storage" {
		log.Debug("Running major checks for storage.")
		if err := checks.CheckIfGlusterdIsRunning(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckMountPointSizes(90); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckLVPoolSizes(90); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckVGSizes(5); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}
	}

	// majors on node
	if viper.GetString("node.type") == "node" {
		log.Debug("Running major checks for node.")

		if err := checks.CheckDockerPool(90); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckDnsNslookupOnKubernetes(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckDnsServiceNode(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}
	}

	// majors on master
	if viper.GetString("node.type") == "master" {
		log.Debug("Running major checks for master.")

		if err := checks.CheckOcGetNodes(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckEtcdHealth(viper.GetString("etcd.ips"), ""); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckRegistryHealth(viper.GetString("registry.ip")); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		for _, rip := range strings.Split(viper.GetString("router.ips"), ",") {
			if err := checks.CheckRouterHealth(rip); err != nil {
				data.Events = append(data.Events, createMajorEvent(err))
			}
		}

		if err := checks.CheckMasterApis("https://localhost:8443/api"); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckDnsNslookupOnKubernetes(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}

		if err := checks.CheckDnsServiceNode(); err != nil {
			data.Events = append(data.Events, createMajorEvent(err))
		}
	}

	/////////////////
	//// MINORS ////
	////////////////
	log.Debug("Running minor checks.")

	// minors on storage
	if viper.GetString("node.type") == "storage" {
		log.Debug("Running minor checks for storage.")
		if err := checks.CheckOpenFileCount(); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckMountPointSizes(85); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckLVPoolSizes(80); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckVGSizes(10); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}
	}

	// minors on node
	if viper.GetString("node.type") == "node" {
		log.Debug("Running minor checks for node.")
		if err := checks.CheckDockerPool(80); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckHttpService(false); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}
	}

	// minors on master
	if viper.GetString("node.type") == "master" {
		log.Debug("Running minor checks for master.")
		if err := checks.CheckExternalSystem(viper.GetString("externalSystemUrl")); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckHawcularHealth(viper.GetString("hawcularIP")); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckRouterRestartCount(); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckLimitsAndQuotas(viper.GetInt("projectsWithoutLimits")); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckHttpService(false); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}

		if err := checks.CheckLoggingRestartsCount(); err != nil {
			data.Events = append(data.Events, createMinorEvent(err))
		}
	}

	log.Debug("Running minor checks for all node types.")
	// minor for all server types
	if err := checks.CheckNtpd(); err != nil {
		data.Events = append(data.Events, createMinorEvent(err))
	}

	OutputJSON(data)
}

func OutputJSON(data interface{}) {
	var output []byte
	var err error

	if pretty {
		output, err = json.MarshalIndent(data, "", "\t")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		log.Errorf("Error outputting JSON (%s).", err)
	}

	if string(output) == "null" {
		fmt.Print("[]")
	} else {
		fmt.Print(string(output))
	}
}
