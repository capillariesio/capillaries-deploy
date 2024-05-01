package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/provider"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

const (
	CmdCreateFloatingIps      string = "create_floating_ips"
	CmdDeleteFloatingIps      string = "delete_floating_ips"
	CmdCreateSecurityGroups   string = "create_security_groups"
	CmdDeleteSecurityGroups   string = "delete_security_groups"
	CmdCreateNetworking       string = "create_networking"
	CmdDeleteNetworking       string = "delete_networking"
	CmdCreateVolumes          string = "create_volumes"
	CmdDeleteVolumes          string = "delete_volumes"
	CmdCreateInstances        string = "create_instances"
	CmdDeleteInstances        string = "delete_instances"
	CmdAttachVolumes          string = "attach_volumes"
	CmdUploadFiles            string = "upload_files"
	CmdDownloadFiles          string = "download_files"
	CmdInstallServices        string = "install_services"
	CmdConfigServices         string = "config_services"
	CmdStartServices          string = "start_services"
	CmdStopServices           string = "stop_services"
	CmdPingInstances          string = "ping_instances"
	CmdConfigCassandraCluster string = "config_cassandra_cluster"
)

type SingleThreadCmdHandler func() (l.LogMsg, error)

func DumpLogChan(logChan chan l.LogMsg) {
	for len(logChan) > 0 {
		msg := <-logChan
		fmt.Println(string(msg))
	}
}

func getNicknamesArg(entityName string) (string, error) {
	if len(os.Args) < 3 {
		return "", fmt.Errorf("not enough args, expected comma-separated list of %s or '*'", entityName)
	}
	if len(os.Args[2]) == 0 {
		return "", fmt.Errorf("bad arg, expected comma-separated list of %s or '*'", entityName)
	}
	return os.Args[2], nil
}

func filterByNickname[GenericDef prj.InstanceDef](nicknames string, sourceMap map[string]*GenericDef, entityName string) (map[string]*GenericDef, error) {
	var defMap map[string]*GenericDef
	rawNicknames := strings.Split(nicknames, ",")
	defMap = map[string]*GenericDef{}
	for _, rawNickname := range rawNicknames {
		if strings.Contains(rawNickname, "*") {
			matchFound := false
			reNickname := regexp.MustCompile("^" + strings.ReplaceAll(rawNickname, "*", "[a-zA-Z0-9]*") + "$")
			for fgNickname, fgDef := range sourceMap {
				if reNickname.MatchString(fgNickname) {
					matchFound = true
					defMap[fgNickname] = fgDef
				}
			}
			if !matchFound {
				return nil, fmt.Errorf("no match found for %s '%s', available definitions: %s", entityName, rawNickname, reflect.ValueOf(sourceMap).MapKeys())
			}
		} else {
			fgDef, ok := sourceMap[rawNickname]
			if !ok {
				return nil, fmt.Errorf("definition for %s '%s' not found, available definitions: %s", entityName, rawNickname, reflect.ValueOf(sourceMap).MapKeys())
			}
			defMap[rawNickname] = fgDef
		}
	}
	return defMap, nil
}

func waitForWorkers(errorsExpected int, errChan chan error, logChan chan l.LogMsg) int {
	finalCmdErr := 0
	for errorsExpected > 0 {
		select {
		case cmdErr := <-errChan:
			if cmdErr != nil {
				finalCmdErr = 1
				fmt.Fprintf(os.Stderr, "%s\n", cmdErr.Error())
			}
			errorsExpected--
		case msg := <-logChan:
			fmt.Println(msg)
		}
	}

	DumpLogChan(logChan)

	return finalCmdErr
}

func usage(flagset *flag.FlagSet) {
	fmt.Printf(`
Capillaries deploy
Usage: capideploy <command> [command parameters] [optional parameters]

Commands:
  %s
  %s
  %s
  %s
  %s
  %s
  %s <comma-separated list of instances to create volumes on, or 'all'>
  %s <comma-separated list of instances to attach volumes on, or 'all'>
  %s <comma-separated list of instances to delete volumes on, or 'all'>
  %s <comma-separated list of instances to create, or 'all'>
  %s <comma-separated list of instances to delete, or 'all'>
  %s <comma-separated list of instances to ping, or 'all'>
  %s <comma-separated list of instances to install services on, or 'all'>
  %s <comma-separated list of instances to config services on, or 'all'>
  %s <comma-separated list of instances to start services on, or 'all'>
  %s <comma-separated list of instances to stop services on, or 'all'>
  %s
`,
		CmdCreateFloatingIps,
		CmdDeleteFloatingIps,
		CmdCreateSecurityGroups,
		CmdDeleteSecurityGroups,
		CmdCreateNetworking,
		CmdDeleteNetworking,

		CmdCreateVolumes,
		CmdAttachVolumes,
		CmdDeleteVolumes,

		CmdCreateInstances,
		CmdDeleteInstances,
		CmdPingInstances,

		CmdInstallServices,
		CmdConfigServices,
		CmdStartServices,
		CmdStopServices,

		CmdConfigCassandraCluster,
	)
	fmt.Printf("\nOptional parameters:\n")
	flagset.PrintDefaults()
}

func main() {
	commonArgs := flag.NewFlagSet("common args", flag.ExitOnError)
	argVerbosity := commonArgs.Bool("verbose", false, "Debug output")
	argPrjFile := commonArgs.String("prj", "capideploy.json", "Capideploy project file path")

	if len(os.Args) <= 1 {
		usage(commonArgs)
		os.Exit(1)
	}

	cmdStartTs := time.Now()

	throttle := time.Tick(time.Second) // One call per second, to avoid error 429 on openstack/aws/azure calls
	const maxWorkerThreads int = 10
	var logChan = make(chan l.LogMsg, maxWorkerThreads*5)
	var sem = make(chan int, maxWorkerThreads)
	var errChan chan error
	var parseErr error
	errorsExpected := 1
	var prjPair *prj.ProjectPair
	var fullPrjPath string
	var prjErr error

	singleThreadCommands := map[string]SingleThreadCmdHandler{
		CmdCreateFloatingIps:    nil,
		CmdDeleteFloatingIps:    nil,
		CmdCreateSecurityGroups: nil,
		CmdDeleteSecurityGroups: nil,
		CmdCreateNetworking:     nil,
		CmdDeleteNetworking:     nil,
	}

	if _, ok := singleThreadCommands[os.Args[1]]; ok {
		parseErr = commonArgs.Parse(os.Args[2:])
	} else {
		parseErr = commonArgs.Parse(os.Args[3:])
	}
	if parseErr != nil {
		log.Fatalf(parseErr.Error())
	}

	prjPair, fullPrjPath, prjErr = prj.LoadProject(*argPrjFile)
	if prjErr != nil {
		log.Fatalf(prjErr.Error())
	}

	deployProvider, deployProviderErr := provider.DeployProviderFactory(prjPair, context.TODO(), *argVerbosity)
	if deployProviderErr != nil {
		log.Fatalf(deployProviderErr.Error())
	}
	singleThreadCommands[CmdCreateFloatingIps] = deployProvider.CreateFloatingIps
	singleThreadCommands[CmdDeleteFloatingIps] = deployProvider.DeleteFloatingIps
	singleThreadCommands[CmdCreateSecurityGroups] = deployProvider.CreateSecurityGroups
	singleThreadCommands[CmdDeleteSecurityGroups] = deployProvider.DeleteSecurityGroups
	singleThreadCommands[CmdCreateNetworking] = deployProvider.CreateNetworking
	singleThreadCommands[CmdDeleteNetworking] = deployProvider.DeleteNetworking

	if cmdHandler, ok := singleThreadCommands[os.Args[1]]; ok {
		errChan = make(chan error, errorsExpected)
		sem <- 1
		go func() {
			logMsg, err := cmdHandler()
			logChan <- logMsg
			errChan <- err
			<-sem
		}()
	} else if os.Args[1] == CmdCreateInstances || os.Args[1] == CmdDeleteInstances {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}
		instances, err := filterByNickname(nicknames, prjPair.Live.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}
		errorsExpected = len(instances)
		errChan = make(chan error, errorsExpected)
		switch os.Args[1] {
		case CmdCreateInstances:
			// Make sure image/flavor is supported
			usedFlavors := map[string]string{}
			usedImages := map[string]string{}
			usedKeypairs := map[string]struct{}{}
			for _, instDef := range instances {
				usedFlavors[instDef.FlavorName] = ""
				usedImages[instDef.ImageName] = ""
				usedKeypairs[instDef.RootKeyName] = struct{}{}
			}
			logMsg, err := deployProvider.HarvestInstanceTypesByFlavorNames(usedFlavors)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			logMsg, err = deployProvider.HarvestImageIdsByImageNames(usedImages)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			logMsg, err = deployProvider.VerifyKeypairs(usedKeypairs)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			fmt.Printf("Creating instances, consider clearing known_hosts to avoid ssh complaints:\n")
			for _, i := range instances {
				fmt.Printf("ssh-keygen -f ~/.ssh/known_hosts -R %s;\n", i.BestIpAddress())
			}

			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(prjPair *prj.ProjectPair, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.CreateInstanceAndWaitForCompletion(
						iNickname,
						usedFlavors[prjPair.Live.Instances[iNickname].FlavorName],
						usedImages[prjPair.Live.Instances[iNickname].ImageName])
					logChan <- logMsg
					errChan <- err
					<-sem
				}(prjPair, logChan, errChan, iNickname)
			}
		case CmdDeleteInstances:
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(prjPair *prj.ProjectPair, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.DeleteInstance(iNickname)
					logChan <- logMsg
					errChan <- err
					<-sem
				}(prjPair, logChan, errChan, iNickname)
			}
		default:
			log.Fatalf("unknown create/delete instance command:" + os.Args[1])
		}
	} else if os.Args[1] == CmdPingInstances ||
		os.Args[1] == CmdInstallServices ||
		os.Args[1] == CmdConfigServices ||
		os.Args[1] == CmdStartServices ||
		os.Args[1] == CmdStopServices {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}

		instances, err := filterByNickname(nicknames, prjPair.Live.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}

		errorsExpected = len(instances)
		errChan = make(chan error, len(instances))
		for _, iDef := range instances {
			<-throttle
			sem <- 1
			go func(prj *prj.Project, logChan chan l.LogMsg, errChan chan error, iDef *prj.InstanceDef) {
				var logMsg l.LogMsg
				var finalErr error
				switch os.Args[1] {
				case CmdPingInstances:
					// Just run WhoAmI
					logMsg, finalErr = rexec.ExecCommandOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), "id", *argVerbosity)
				case CmdInstallServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Install, iDef.Service.Env, *argVerbosity)

				case CmdConfigServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Config, iDef.Service.Env, *argVerbosity)

				case CmdStartServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Start, iDef.Service.Env, *argVerbosity)

				case CmdStopServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Stop, iDef.Service.Env, *argVerbosity)

				default:
					log.Fatalf("unknown service command:" + os.Args[1])
				}

				logChan <- logMsg
				errChan <- finalErr
				<-sem
			}(&prjPair.Live, logChan, errChan, iDef)
		}

	} else if os.Args[1] == CmdCreateVolumes || os.Args[1] == CmdAttachVolumes || os.Args[1] == CmdDeleteVolumes {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}

		instances, err := filterByNickname(nicknames, prjPair.Live.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}

		volCount := 0
		for _, iDef := range instances {
			volCount += len(iDef.Volumes)
		}
		if volCount == 0 {
			fmt.Printf("No volumes to create/attach/delete")
			os.Exit(0)
		}
		errorsExpected = volCount
		errChan = make(chan error, volCount)
		for iNickname, iDef := range instances {
			for volNickname := range iDef.Volumes {
				<-throttle
				sem <- 1
				switch os.Args[1] {
				case CmdCreateVolumes:
					go func(prjPair *prj.ProjectPair, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.CreateVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(prjPair, logChan, errChan, iNickname, volNickname)
				case CmdAttachVolumes:
					go func(prjPair *prj.ProjectPair, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.AttachVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(prjPair, logChan, errChan, iNickname, volNickname)
				case CmdDeleteVolumes:
					go func(prjPair *prj.ProjectPair, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.DeleteVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(prjPair, logChan, errChan, iNickname, volNickname)
				default:
					log.Fatalf("unknown command:" + os.Args[1])
				}
			}
		}
	} else {
		switch os.Args[1] {
		case CmdConfigCassandraCluster:
			var someCassIpAddress string
			cassandraInstanceDefs := map[string]*prj.InstanceDef{}
			for iNickname, iDef := range prjPair.Live.Instances {
				if iDef.Purpose == string(prj.InstancePurposeCassandra) {
					cassandraInstanceDefs[iNickname] = iDef
					if someCassIpAddress == "" {
						someCassIpAddress = iDef.IpAddress
					}
				}
			}

			if len(cassandraInstanceDefs) == 0 {
				log.Fatalf("no cassandra instances")
			}

			// Stop all at once

			errorsExpected = len(cassandraInstanceDefs)
			errChan = make(chan error, len(cassandraInstanceDefs))
			for _, iDef := range cassandraInstanceDefs {
				<-throttle
				sem <- 1
				go func(prj *prj.Project, logChan chan l.LogMsg, errChan chan error, iDef *prj.InstanceDef) {
					logMsg, finalErr := rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Stop, iDef.Service.Env, *argVerbosity)
					logChan <- logMsg
					errChan <- finalErr
					<-sem
				}(&prjPair.Live, logChan, errChan, iDef)
			}

			stopCassErr := waitForWorkers(errorsExpected, errChan, logChan)
			if stopCassErr > 0 {
				os.Exit(stopCassErr)
			}

			// Config/start all at once

			errorsExpected = len(cassandraInstanceDefs)
			errChan = make(chan error, len(cassandraInstanceDefs))
			for _, iDef := range cassandraInstanceDefs {
				<-throttle
				sem <- 1
				go func(prj *prj.Project, logChan chan l.LogMsg, errChan chan error, iDef *prj.InstanceDef) {
					logMsg, finalErr := rexec.ExecEmbeddedScriptsOnInstance(prjPair.Live.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Config, iDef.Service.Env, *argVerbosity)
					logChan <- logMsg
					errChan <- finalErr
					<-sem
				}(&prjPair.Live, logChan, errChan, iDef)
			}

			confCassErr := waitForWorkers(errorsExpected, errChan, logChan)
			if confCassErr > 0 {
				os.Exit(confCassErr)
			}

			// Verify the cluster is running

			errorsExpected = 1
			errChan = make(chan error, 1)
			sem <- 1
			go func(prj *prj.Project, logChan chan l.LogMsg, errChan chan error) {
				startWaitTs := time.Now()
				var logMsg l.LogMsg
				var finalErr error
				for {
					logMsg, finalErr = rexec.ExecCommandOnInstance(prjPair.Live.SshConfig, someCassIpAddress, "nodetool describecluster;nodetool status", true)
					if finalErr != nil {
						break
					}

					if strings.Contains(string(logMsg), "Normal/Leaving/Joining/Moving") {
						completeCount := 0
						for _, iDef := range cassandraInstanceDefs {
							if strings.Contains(string(logMsg), "UN  "+iDef.IpAddress) {
								completeCount++
							}
						}

						if completeCount == len(cassandraInstanceDefs) {
							break
						}
					} else {
						if !strings.Contains(string(logMsg), "nodetool: Failed to connect") ||
							!strings.Contains(string(logMsg), "InstanceNotFoundException") ||
							!strings.Contains(string(logMsg), "Has this node finished starting up") {
							// Unknown problem
							finalErr = fmt.Errorf("unknown nodetool output")
							break
						}
					}

					if time.Since(startWaitTs).Seconds() > float64(60) {
						finalErr = fmt.Errorf("giving up waiting for cluster to start")
						break
					}
					time.Sleep(5 * time.Second)
				}
				logChan <- logMsg
				errChan <- finalErr
				<-sem
			}(&prjPair.Live, logChan, errChan)

		default:
			log.Fatalf("unknown command:" + os.Args[1])
		}
	}

	finalCmdErr := waitForWorkers(errorsExpected, errChan, logChan)

	// Save updated project template, it may have some new ids and timestamps
	if prjErr = prjPair.Template.SaveProject(fullPrjPath); prjErr != nil {
		log.Fatalf(prjErr.Error())
	}

	if finalCmdErr > 0 {
		os.Exit(finalCmdErr)
	}

	fmt.Printf("%s %sOK%s, elapsed %.3fs\n", os.Args[1], l.LogColorGreen, l.LogColorReset, time.Since(cmdStartTs).Seconds())
}
