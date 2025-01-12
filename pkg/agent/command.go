// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"fmt"
	"net"
	"os"

	"github.com/gardener/network-problem-detector/pkg/agent/runners"
	"github.com/gardener/network-problem-detector/pkg/common"
	"github.com/gardener/network-problem-detector/pkg/common/nwpd"

	"github.com/hashicorp/mdns"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	agentConfigFile   string
	clusterConfigFile string
	hostNetwork       bool
	grpcServer        *grpc.Server
)

func CreateRunAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-agent",
		Short: "runs agent server",
		Long:  `The agent runs in a pod either on the host network or the pod network`,
	}
	cmd.Flags().StringVar(&agentConfigFile, "config", "agent.config", "file configuration of agent server.")
	cmd.Flags().StringVar(&clusterConfigFile, "cluster-config", "cluster.config", "file configuration of cluster nodes and agent pods.")
	cmd.Flags().BoolVar(&hostNetwork, "hostNetwork", false, "if agent runs on host network.")
	cmd.RunE = runAgent
	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	log := logrus.WithField("cmd", "agent")

	if agentConfigFile == "" {
		return fmt.Errorf("Missing --config option")
	}
	if clusterConfigFile == "" {
		return fmt.Errorf("Missing --cluster-config option")
	}

	srv, realPort, err := startAgentServer(log, agentConfigFile, clusterConfigFile, hostNetwork)
	if err != nil {
		return fmt.Errorf("cannot start server: %w", err)
	}

	if hostNetwork && srv.getNetworkCfg().StartMDNSServer {
		nodeName := runners.GetNodeName()
		ipstr := os.Getenv(common.EnvNodeIP)

		var ips []net.IP
		if ipstr != "" {
			ip := net.ParseIP(ipstr)
			if ip == nil {
				return fmt.Errorf("cannot parse IP %s", ipstr)
			}
			ips = append(ips, ip)
		}
		info := []string{"network problem detector agent server"}
		service, err := mdns.NewMDNSService(nodeName, common.MDNSServiceHostNetAgent, "", "", realPort, ips, info)
		if err != nil {
			return fmt.Errorf("NewMDNSService failed: %w", err)
		}

		// Create the mDNS server, defer shutdown
		server, err := mdns.NewServer(&mdns.Config{Zone: service})
		if err != nil {
			return fmt.Errorf("create MDNS server failed: %w", err)
		}
		defer server.Shutdown()
		log.Info("mDNS server started")
	}
	log.Info("running...")
	srv.run()
	return nil
}

func startAgentServer(log logrus.FieldLogger, agentConfigFile, clusterConfigFile string, hostNetwork bool) (*server, int, error) {
	agentServer, err := newServer(log, agentConfigFile, clusterConfigFile, hostNetwork)
	if err != nil {
		return nil, 0, err
	}

	err = agentServer.setup()
	if err != nil {
		return nil, 0, err
	}

	/*
		creds, err := loadTLSCredentials(log, config)
		if err != nil {
			return nil, err
		}
	*/
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", agentServer.getNetworkCfg().GRPCPort))
	if err != nil {
		return nil, 0, err
	}
	//	s := grpc.NewServer(grpc.Creds(creds))
	grpcServer = grpc.NewServer()
	nwpd.RegisterAgentServiceServer(grpcServer, agentServer)
	log.Infof("server listening at %s", listener.Addr())
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return agentServer, listener.Addr().(*net.TCPAddr).Port, nil
}
